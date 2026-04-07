package aiff

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestRegistered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatAIFF)
	assert.True(t, ok)
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"AIFF", []byte("FORM\x00\x00\x00\x00AIFF"), true},
		{"AIFC", []byte("FORM\x00\x00\x00\x00AIFC"), true},
		{"RIFF", []byte("RIFF\x00\x00\x00\x00WAVE"), false},
		{"Short", []byte("FO"), false},
	}
	d := &Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, d.Probe(bytes.NewReader(tt.data)))
		})
	}
}

func TestBitDepths(t *testing.T) {
	tests := []struct {
		name  string
		bps   int
		depth ir.BitDepth
		pcm   []byte
		want  float64
	}{
		{"8bit", 8, ir.BitDepth8, []byte{128, 255, 0}, 0.0},
		{"16bit", 16, ir.BitDepth16, []byte{0x7F, 0xFF, 0x80, 0x01}, 1.0},
		{"24bit", 24, ir.BitDepth24, []byte{0x7F, 0xFF, 0xFF}, 1.0},
		{"32bit", 32, ir.BitDepth32, beU32(0x7FFFFFFF), 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildAIFF(1, len(tt.pcm)/(tt.bps/8), tt.bps, 44100, tt.pcm)
			scene := decodeOK(t, data)

			clip := scene.AudioClips[0]
			assert.Equal(t, tt.depth, clip.BitDepth)
			assert.InDelta(t, tt.want, getSamples(t, clip)[0], 0.02)
		})
	}
}

func TestChannelLayouts(t *testing.T) {
	tests := []struct {
		name     string
		channels int
		layout   ir.ChannelLayout
	}{
		{"Mono", 1, ir.LayoutMono},
		{"Stereo", 2, ir.LayoutStereo},
		{"5.1", 6, ir.Layout5_1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcm := make([]byte, tt.channels*2)
			data := buildAIFF(tt.channels, 1, 16, 44100, pcm)
			scene := decodeOK(t, data)
			assert.Equal(t, tt.layout, scene.AudioClips[0].Layout)
		})
	}
}

func TestAIFCCodecs(t *testing.T) {
	tests := []struct {
		name    string
		comp    string
		bps     int
		pcm     []byte
		minLen  int
		checkFn func(t *testing.T, samples []float32)
	}{
		{
			name: "NONE", comp: "NONE", bps: 16,
			pcm:    []byte{0x7F, 0xFF, 0x80, 0x01},
			minLen: 2,
			checkFn: func(t *testing.T, s []float32) {
				assert.InDelta(t, 1.0, s[0], 0.001)
			},
		},
		{
			name: "Sowt_LE", comp: "sowt", bps: 16,
			pcm:    []byte{0xFF, 0x7F, 0x01, 0x80},
			minLen: 2,
			checkFn: func(t *testing.T, s []float32) {
				assert.InDelta(t, 1.0, s[0], 0.001)
				assert.InDelta(t, -1.0, s[1], 0.001)
			},
		},
		{
			name: "Float32", comp: "fl32", bps: 32,
			pcm:    leF32(0.5),
			minLen: 1,
			checkFn: func(t *testing.T, s []float32) {
				assert.InDelta(t, 0.5, s[0], 0.001)
			},
		},
		{
			name: "Alaw", comp: "alaw", bps: 8,
			pcm: []byte{0xD5, 0x55}, minLen: 2,
			checkFn: func(t *testing.T, s []float32) {
				assert.NotEqual(t, float32(0), s[0])
			},
		},
		{
			name: "Ulaw", comp: "ulaw", bps: 8,
			pcm: []byte{0xFF, 0x00}, minLen: 2,
		},
		{
			name: "IMA4", comp: "ima4", bps: 16,
			pcm: make([]byte, 34), minLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := len(tt.pcm) / max(1, tt.bps/8)
			if tt.comp == "ima4" {
				frames = 64
			}
			data := buildAIFC(tt.comp, 1, frames, tt.bps, 44100, tt.pcm)
			scene := decodeOK(t, data)
			require.GreaterOrEqual(t, len(getSamples(t, scene.AudioClips[0])), tt.minLen)
			if tt.checkFn != nil {
				tt.checkFn(t, getSamples(t, scene.AudioClips[0]))
			}
		})
	}
}

func TestLoopPoints(t *testing.T) {
	tests := []struct {
		name      string
		markers   []marker
		inst      chunk
		wantStart int
		wantEnd   int
	}{
		{
			name:      "Sustain",
			markers:   []marker{{id: 1, pos: 10}, {id: 2, pos: 90}},
			inst:      instChunk(1, 1, 2, 0, 0, 0),
			wantStart: 10, wantEnd: 90,
		},
		{
			name:      "ReleaseFallback",
			markers:   []marker{{id: 3, pos: 20}, {id: 4, pos: 80}},
			inst:      instChunk(0, 0, 0, 1, 3, 4),
			wantStart: 20, wantEnd: 80,
		},
		{
			name:      "NoLoop",
			inst:      instChunk(0, 0, 0, 0, 0, 0),
			wantStart: ir.NoIndex, wantEnd: ir.NoIndex,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcm := make([]byte, 200)
			chunks := []chunk{tt.inst}
			if len(tt.markers) > 0 {
				chunks = append([]chunk{markChunk(tt.markers)}, chunks...)
			}
			data := buildAIFFWithChunks(1, 100, 16, 44100, pcm, chunks)
			scene := decodeOK(t, data)

			assert.Equal(t, tt.wantStart, scene.AudioClips[0].LoopStart)
			assert.Equal(t, tt.wantEnd, scene.AudioClips[0].LoopEnd)
		})
	}
}

func TestMetadata(t *testing.T) {
	tests := []struct {
		name    string
		chunkID string
		text    string
		check   func(t *testing.T, m ir.AudioMetadata)
	}{
		{"NAME", "NAME", "Test Title", func(t *testing.T, m ir.AudioMetadata) { assert.Equal(t, "Test Title", m.Title) }},
		{"AUTH", "AUTH", "Test Artist", func(t *testing.T, m ir.AudioMetadata) { assert.Equal(t, "Test Artist", m.Artist) }},
		{"ANNO", "ANNO", "A comment", func(t *testing.T, m ir.AudioMetadata) { assert.Equal(t, "A comment", m.Comment) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcm := make([]byte, 4)
			data := buildAIFFWithChunks(1, 2, 16, 44100, pcm, []chunk{textChunk(tt.chunkID, tt.text)})
			scene := decodeOK(t, data)
			tt.check(t, scene.AudioClips[0].Metadata)
		})
	}
}

func TestDuration(t *testing.T) {
	pcm := make([]byte, 44100*2)
	data := buildAIFF(1, 44100, 16, 44100, pcm)
	scene := decodeOK(t, data)
	assert.InDelta(t, 1.0, scene.AudioClips[0].Duration.Seconds(), 0.01)
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"NotFORM", []byte("RIFF\x00\x00\x00\x00WAVE")},
		{"NotAIFF", []byte("FORM\x00\x00\x00\x00XXXX")},
		{"TooShort", []byte("FORM")},
	}
	d := &Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := d.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			assert.Error(t, err)
		})
	}
}

func TestSowt8And24Bit(t *testing.T) {
	tests := []struct {
		name string
		bps  int
		pcm  []byte
	}{
		{"Sowt8bit", 8, []byte{128, 255}},
		{"Sowt24bit", 24, []byte{0xFF, 0x7F, 0x00, 0x01, 0x80, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := len(tt.pcm) / (tt.bps / 8)
			data := buildAIFC("sowt", 1, frames, tt.bps, 44100, tt.pcm)
			scene := decodeOK(t, data)
			require.GreaterOrEqual(t, len(getSamples(t, scene.AudioClips[0])), 1)
		})
	}
}

func TestUnknownChunkSkipped(t *testing.T) {
	pcm := make([]byte, 4)
	extra := []chunk{{id: "XYZW", data: []byte("unknown data padding!")}}
	data := buildAIFFWithChunks(1, 2, 16, 44100, pcm, extra)
	scene := decodeOK(t, data)
	assert.Equal(t, 2, len(getSamples(t, scene.AudioClips[0])))
}

// ─── Helpers ────────────────────────────────────────────────────────

func getSamples(t testing.TB, c *ir.AudioClip) []float32 {
	t.Helper()
	s, err := c.DecodeSamples()
	require.NoError(t, err)
	return s
}

func decodeOK(t *testing.T, data []byte) *ir.Asset {
	t.Helper()
	scene, err := (&Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	_, err = scene.AudioClips[0].DecodeSamples()
	require.NoError(t, err)
	return scene
}

func beU32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func leF32(v float32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
	return b
}

// ─── Binary builders ────────────────────────────────────────────────

type chunk struct {
	id   string
	data []byte
}

func buildAIFF(channels, frames, bps, sr int, pcm []byte) []byte { //nolint:unparam
	return buildAIFFWithChunks(channels, frames, bps, sr, pcm, nil)
}

func buildAIFFWithChunks(channels, frames, bps, sr int, pcm []byte, extra []chunk) []byte {
	var body bytes.Buffer
	body.Write([]byte("AIFF"))
	writeCOMM(&body, channels, frames, bps, sr)
	for _, c := range extra {
		writeRawChunk(&body, c.id, c.data)
	}
	writeSSND(&body, pcm)
	return wrapFORM(body.Bytes())
}

func buildAIFC(comp string, channels, frames, bps, sr int, pcm []byte) []byte { //nolint:unparam
	var body bytes.Buffer
	body.Write([]byte("AIFC"))
	writeCOMMAIFC(&body, channels, frames, bps, sr, comp)
	writeSSND(&body, pcm)
	return wrapFORM(body.Bytes())
}

func wrapFORM(body []byte) []byte {
	var buf bytes.Buffer
	buf.Write([]byte("FORM"))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(body)))
	buf.Write(body)
	return buf.Bytes()
}

func writeCOMM(w *bytes.Buffer, channels, frames, bps, sr int) {
	var d bytes.Buffer
	_ = binary.Write(&d, binary.BigEndian, int16(channels))
	_ = binary.Write(&d, binary.BigEndian, uint32(frames))
	_ = binary.Write(&d, binary.BigEndian, int16(bps))
	d.Write(intToExtended80(sr))
	writeRawChunk(w, "COMM", d.Bytes())
}

func writeCOMMAIFC(w *bytes.Buffer, channels, frames, bps, sr int, comp string) {
	var d bytes.Buffer
	_ = binary.Write(&d, binary.BigEndian, int16(channels))
	_ = binary.Write(&d, binary.BigEndian, uint32(frames))
	_ = binary.Write(&d, binary.BigEndian, int16(bps))
	d.Write(intToExtended80(sr))
	d.Write([]byte(comp)[:4])
	d.WriteByte(0)
	d.WriteByte(0)
	writeRawChunk(w, "COMM", d.Bytes())
}

func writeSSND(w *bytes.Buffer, pcm []byte) {
	var d bytes.Buffer
	_ = binary.Write(&d, binary.BigEndian, uint32(0))
	_ = binary.Write(&d, binary.BigEndian, uint32(0))
	d.Write(pcm)
	writeRawChunk(w, "SSND", d.Bytes())
}

func writeRawChunk(w *bytes.Buffer, id string, data []byte) {
	w.Write([]byte(id)[:4])
	_ = binary.Write(w, binary.BigEndian, uint32(len(data)))
	w.Write(data)
	if len(data)%2 != 0 {
		w.WriteByte(0)
	}
}

func intToExtended80(val int) []byte {
	if val == 0 {
		return make([]byte, 10)
	}
	exp := 16383 + 63
	mantissa := uint64(val)
	for mantissa != 0 && mantissa&(1<<63) == 0 {
		mantissa <<= 1
		exp--
	}
	b := make([]byte, 10)
	binary.BigEndian.PutUint16(b[0:], uint16(exp))
	binary.BigEndian.PutUint64(b[2:], mantissa)
	return b
}

type marker struct {
	id  int16
	pos uint32
}

func markChunk(markers []marker) chunk {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(markers)))
	for _, m := range markers {
		_ = binary.Write(&buf, binary.BigEndian, m.id)
		_ = binary.Write(&buf, binary.BigEndian, m.pos)
		buf.WriteByte(1)   // Pascal string length
		buf.WriteByte('M') // 1 char (odd length = no pad)
	}
	return chunk{id: "MARK", data: buf.Bytes()}
}

func instChunk(sustainMode, sustainBegin, sustainEnd, releaseMode, releaseBegin, releaseEnd int16) chunk {
	var buf bytes.Buffer
	buf.Write([]byte{60, 0, 0, 127, 0, 127, 0, 0})
	_ = binary.Write(&buf, binary.BigEndian, sustainMode)
	_ = binary.Write(&buf, binary.BigEndian, sustainBegin)
	_ = binary.Write(&buf, binary.BigEndian, sustainEnd)
	_ = binary.Write(&buf, binary.BigEndian, releaseMode)
	_ = binary.Write(&buf, binary.BigEndian, releaseBegin)
	_ = binary.Write(&buf, binary.BigEndian, releaseEnd)
	return chunk{id: "INST", data: buf.Bytes()}
}

func textChunk(id, text string) chunk {
	return chunk{id: id, data: []byte(text)}
}
