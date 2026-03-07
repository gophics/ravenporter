package opus

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestOpus_Registered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatOpus)
	assert.True(t, ok)
}

func TestOpus_ParseHead(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantErr  bool
		wantCh   int
		wantSR   int
		wantSkip int
		wantGain int16
	}{
		{
			name:     "Valid stereo 48kHz",
			input:    []byte("OpusHead\x01\x02\x38\x01\x80\xbb\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
			wantCh:   2,
			wantSR:   48000,
			wantSkip: 312,
		},
		{
			name:     "Valid mono 16kHz",
			input:    makeHead(1, 160, 16000, 0),
			wantCh:   1,
			wantSR:   16000,
			wantSkip: 160,
		},
		{
			name:     "With positive output gain",
			input:    makeHead(2, 312, 48000, 256),
			wantCh:   2,
			wantSR:   48000,
			wantSkip: 312,
			wantGain: 256,
		},
		{
			name:     "With negative output gain",
			input:    makeHead(2, 312, 48000, -512),
			wantCh:   2,
			wantSR:   48000,
			wantSkip: 312,
			wantGain: -512,
		},
		{"Too Short", []byte("Opus"), true, 0, 0, 0, 0},
		{"Bad Magic", []byte("OggSHead\x01\x02\x38\x01\x80\xbb\x00\x00\x00\x00\x00"), true, 0, 0, 0, 0},
		{"Short Payload", []byte("OpusHead\x01\x02\x38\x01"), true, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseHead(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, info)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, info)
			assert.Equal(t, tt.wantCh, info.channelCount)
			assert.Equal(t, tt.wantSR, info.sampleRate)
			assert.Equal(t, tt.wantSkip, info.preSkip)
			assert.Equal(t, tt.wantGain, info.outputGain)
		})
	}
}

func TestOpus_Probe_NonOGG(t *testing.T) {
	d := &Decoder{}
	assert.False(t, d.Probe(bytes.NewReader([]byte("RIFF"))))
}

func TestOpus_TOCFrameSamples(t *testing.T) {
	tests := []struct {
		name   string
		toc    byte
		expect int
	}{
		{"SILK NB 10ms (config 0)", 0 << configShft, tocSamples10ms},
		{"SILK NB 20ms (config 1)", 1 << configShft, tocSamples20ms},
		{"SILK NB 40ms (config 2)", 2 << configShft, tocSamples40ms},
		{"SILK NB 60ms (config 3)", 3 << configShft, tocSamples60ms},
		{"SILK WB 20ms (config 9)", 9 << configShft, tocSamples20ms},
		{"CELT NB 2.5ms (config 16)", 16 << configShft, tocSamples2500us},
		{"CELT NB 5ms (config 17)", 17 << configShft, tocSamples5ms},
		{"CELT FB 20ms (config 31)", 31 << configShft, tocSamples20ms},
		{"Hybrid SWB 10ms (config 12)", 12 << configShft, tocSamples10ms},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tocFrameSamples(tt.toc))
		})
	}
}

func TestOpus_ParseR128TagGain(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want int16
	}{
		{"No R128 tag", map[string]string{"ARTIST": "Test"}, 0},
		{"Track gain positive", map[string]string{"R128_TRACK_GAIN": "256"}, 256},
		{"Track gain negative", map[string]string{"R128_TRACK_GAIN": "-512"}, -512},
		{"Album gain fallback", map[string]string{"R128_ALBUM_GAIN": "128"}, 128},
		{"Invalid value", map[string]string{"R128_TRACK_GAIN": "notanumber"}, 0},
		{"Zero gain", map[string]string{"R128_TRACK_GAIN": "0"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildOpusTags(tt.tags)
			result := parseR128TagGain(data)
			assert.Equal(t, tt.want, result)
		})
	}

	t.Run("Track gain priority", func(t *testing.T) {
		data := buildOpusTagsOrdered([][2]string{
			{"R128_TRACK_GAIN", "100"},
			{"R128_ALBUM_GAIN", "200"},
		})
		result := parseR128TagGain(data)
		assert.Equal(t, int16(100), result)
	})
}

func TestOpus_ParseTags(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want ir.AudioMetadata
	}{
		{
			name: "Basic metadata",
			tags: map[string]string{"TITLE": "Hello", "ARTIST": "World"},
			want: ir.AudioMetadata{Title: "Hello", Artist: "World"},
		},
		{
			name: "Empty tags",
			tags: map[string]string{},
			want: ir.AudioMetadata{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildOpusTags(tt.tags)
			md := parseTags(data)
			assert.Equal(t, tt.want.Title, md.Title)
			assert.Equal(t, tt.want.Artist, md.Artist)
		})
	}
}

func TestOpus_ChainedDemux(t *testing.T) {
	chain1Head := buildOggPage(1, flagBOS, 0, 1000, makeHead(1, 160, 48000, 0))
	chain1Tags := buildOggPage(1, 0, 0, 1000, buildOpusTagsRaw(nil))
	chain1Audio := buildOggPage(1, 0, 960, 1000, []byte{0x08, 0x00})

	chain2Head := buildOggPage(2, flagBOS, 0, 2000, makeHead(2, 312, 48000, 0))
	chain2Tags := buildOggPage(2, 0, 0, 2000, buildOpusTagsRaw(nil))
	chain2Audio := buildOggPage(2, flagEOS, 960, 2000, []byte{0x0C, 0x00})

	var stream []byte
	stream = append(stream, chain1Head...)
	stream = append(stream, chain1Tags...)
	stream = append(stream, chain1Audio...)
	stream = append(stream, chain2Head...)
	stream = append(stream, chain2Tags...)
	stream = append(stream, chain2Audio...)

	demuxer := acquireOggDemuxer(bytes.NewReader(stream))

	err := demuxer.readNextPacket()
	assert.NoError(t, err)
	assert.True(t, demuxer.packet.bos)
	assert.Equal(t, uint32(1000), demuxer.packet.serial)

	err = demuxer.readNextPacket()
	assert.NoError(t, err)

	err = demuxer.readNextPacket()
	assert.NoError(t, err)
	assert.Equal(t, uint32(1000), demuxer.packet.serial)

	err = demuxer.readNextPacket()
	assert.NoError(t, err)
	assert.True(t, demuxer.packet.bos)
	assert.Equal(t, uint32(2000), demuxer.packet.serial)
}

func BenchmarkOpus_Decode(b *testing.B) {
	data, err := os.ReadFile("../testdata/minimal.opus")
	if err != nil {
		b.Skip("testdata not available")
	}
	d := &Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), opts)
	}
}

// --- helpers ---

func makeHead(channels uint8, preSkip uint16, sampleRate uint32, gain int16) []byte {
	buf := make([]byte, headTagLen+minPayload)
	copy(buf, "OpusHead")
	buf[8] = 1 // version
	buf[channelOff] = channels
	binary.LittleEndian.PutUint16(buf[preSkipOff:], preSkip)
	binary.LittleEndian.PutUint32(buf[srOff:], sampleRate)
	binary.LittleEndian.PutUint16(buf[gainOff:], uint16(gain))
	return buf
}

func buildOpusTags(tags map[string]string) []byte {
	return append([]byte(tagsTag), buildOpusTagsRaw(tags)...)
}

func buildOpusTagsRaw(tags map[string]string) []byte {
	var buf []byte
	vendor := "vendor"
	vl := make([]byte, 4)
	binary.LittleEndian.PutUint32(vl, uint32(len(vendor)))
	buf = append(buf, vl...)
	buf = append(buf, vendor...)
	cl := make([]byte, 4)
	binary.LittleEndian.PutUint32(cl, uint32(len(tags)))
	buf = append(buf, cl...)
	for k, v := range tags {
		entry := k + "=" + v
		el := make([]byte, 4)
		binary.LittleEndian.PutUint32(el, uint32(len(entry)))
		buf = append(buf, el...)
		buf = append(buf, entry...)
	}
	return buf
}

func buildOpusTagsOrdered(tags [][2]string) []byte {
	var buf []byte
	buf = append(buf, tagsTag...)
	vendor := "test"
	vl := make([]byte, 4)
	binary.LittleEndian.PutUint32(vl, uint32(len(vendor)))
	buf = append(buf, vl...)
	buf = append(buf, vendor...)
	cl := make([]byte, 4)
	binary.LittleEndian.PutUint32(cl, uint32(len(tags)))
	buf = append(buf, cl...)
	for _, kv := range tags {
		entry := kv[0] + "=" + kv[1]
		el := make([]byte, 4)
		binary.LittleEndian.PutUint32(el, uint32(len(entry)))
		buf = append(buf, el...)
		buf = append(buf, entry...)
	}
	return buf
}

func buildOggPage(seqNo int, headerType uint8, granule int64, serial uint32, payload []byte) []byte {
	segCount := (len(payload) / 255) + 1
	var header []byte
	header = append(header, "OggS"...)
	header = append(header, 0, headerType)

	g := make([]byte, 8)
	binary.LittleEndian.PutUint64(g, uint64(granule))
	header = append(header, g...)

	s := make([]byte, 4)
	binary.LittleEndian.PutUint32(s, serial)
	header = append(header, s...)

	sq := make([]byte, 4)
	binary.LittleEndian.PutUint32(sq, uint32(seqNo))
	header = append(header, sq...)

	header = append(header, 0, 0, 0, 0, uint8(segCount))

	remaining := len(payload)
	for remaining > 255 {
		header = append(header, 255)
		remaining -= 255
	}
	header = append(header, uint8(remaining))

	header = append(header, payload...)
	return header
}

func TestOpus_DecoderProperties(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, "Opus", d.FormatName())
	assert.Equal(t, []string{".opus"}, d.Extensions())
}

func TestOpus_DecodeStereo16(t *testing.T) {
	data, err := os.ReadFile("../testdata/stereo_16bit.opus")
	if err != nil {
		t.Skip("testdata not available")
	}
	d := &Decoder{}
	scene, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioOpus, clip.Format)
	assert.Equal(t, opusSR, clip.SampleRate)
	assert.Equal(t, ir.LayoutStereo, clip.Layout)
}

func TestOpus_ComputeGainScalar(t *testing.T) {
	tests := []struct {
		name       string
		headerGain int16
		tagsData   []byte
		wantOne    bool
	}{
		{"Zero gain returns 1.0", 0, nil, true},
		{"Header gain only", 256, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scalar := computeGainScalar(tt.headerGain, tt.tagsData)
			if tt.wantOne {
				assert.InDelta(t, 1.0, scalar, 0.001)
			} else {
				assert.NotEqual(t, float32(1.0), scalar)
			}
		})
	}
}

func TestOpus_ExtractPicture(t *testing.T) {
	t.Run("Invalid base64", func(t *testing.T) {
		md := ir.AudioMetadata{}
		extractPicture([]byte("not-valid-b64!!!"), &md)
		assert.Nil(t, md.Artwork)
	})

	t.Run("Too short decoded", func(t *testing.T) {
		md := ir.AudioMetadata{}
		extractPicture([]byte("AAAA"), &md)
		assert.Nil(t, md.Artwork)
	})
}

func TestOpus_Probe_ValidOpus(t *testing.T) {
	data, err := os.ReadFile("../testdata/stereo_16bit.opus")
	if err != nil {
		t.Skip("testdata not available")
	}
	d := &Decoder{}
	assert.True(t, d.Probe(bytes.NewReader(data)))
}

func TestOpus_Probe_ShortInput(t *testing.T) {
	d := &Decoder{}
	assert.False(t, d.Probe(bytes.NewReader([]byte("Ogg"))))
}
