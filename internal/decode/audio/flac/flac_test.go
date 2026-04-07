package flac

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/testutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatFLAC)
	assert.True(t, ok)
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"Valid", magic, true},
		{"RIFF", []byte("RIFF"), false},
		{"Short", []byte("fL"), false},
	}
	d := &Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, d.Probe(bytes.NewReader(tt.data)))
		})
	}
}

func TestDecodeRejectsOversizedInputBeforeRead(t *testing.T) {
	src := testutil.NewOversizeReadSeeker(8)
	_, err := (&Decoder{}).Decode(src, detect.DecodeOptions{MaxFileSize: 4})
	require.Error(t, err)
	assert.ErrorContains(t, err, "file too large")
	assert.Zero(t, src.Reads)
}

func TestParseStreamInfo(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate int
		channels   int
		bps        int
		total      int
	}{
		{"44100Hz_Stereo_16bit", 44100, 2, 16, 441000},
		{"48000Hz_Mono_24bit", 48000, 1, 24, 96000},
		{"96000Hz_Stereo_32bit", 96000, 2, 32, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildFLACMetadata(tt.sampleRate, tt.channels, tt.bps, tt.total)
			info, _, audioStart := parseMetadata(data)

			assert.Equal(t, tt.sampleRate, info.sampleRate)
			assert.Equal(t, tt.channels, info.channels)
			assert.Equal(t, tt.bps, info.bitsPerSample)
			assert.Equal(t, tt.total, info.totalSamples)
			assert.Greater(t, audioStart, magicLen)
		})
	}
}

func TestMetadata(t *testing.T) {
	tests := []struct {
		name  string
		tags  map[string]string
		check func(t *testing.T, m ir.AudioMetadata)
	}{
		{
			"Title", map[string]string{"TITLE": "Test Song"},
			func(t *testing.T, m ir.AudioMetadata) { assert.Equal(t, "Test Song", m.Title) },
		},
		{
			"Artist", map[string]string{"ARTIST": "Test Artist"},
			func(t *testing.T, m ir.AudioMetadata) { assert.Equal(t, "Test Artist", m.Artist) },
		},
		{
			"Album", map[string]string{"ALBUM": "Test Album"},
			func(t *testing.T, m ir.AudioMetadata) { assert.Equal(t, "Test Album", m.Album) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc := buildVorbisComment(tt.tags)
			data := buildFLACWithBlocks(44100, 2, 16, 0, []metaBlock{{typ: vorbisComment, data: vc}})
			_, meta, _ := parseMetadata(data)
			tt.check(t, meta)
		})
	}
}

func TestPictureBlock(t *testing.T) {
	pic := buildPictureBlock("image/png", "Cover Art", []byte{0x89, 'P', 'N', 'G'})
	data := buildFLACWithBlocks(44100, 2, 16, 0, []metaBlock{{typ: pictureBlock, data: pic}})
	_, meta, _ := parseMetadata(data)
	assert.Equal(t, "Cover Art", meta.Comment)
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
			data := buildFLACMetadata(44100, tt.channels, 16, 0)
			scene := decodeOK(t, data)
			assert.Equal(t, tt.layout, scene.AudioClips[0].Layout)
		})
	}
}

func TestBitDepths(t *testing.T) {
	tests := []struct {
		name  string
		bps   int
		depth ir.BitDepth
	}{
		{"8bit", 8, ir.BitDepth8},
		{"16bit", 16, ir.BitDepth16},
		{"24bit", 24, ir.BitDepth24},
		{"32bit", 32, ir.BitDepth32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildFLACMetadata(44100, 2, tt.bps, 0)
			scene := decodeOK(t, data)
			assert.Equal(t, tt.depth, scene.AudioClips[0].BitDepth)
		})
	}
}

func TestDuration(t *testing.T) {
	data := buildFLACMetadata(44100, 2, 16, 44100)
	scene := decodeOK(t, data)
	assert.InDelta(t, 1.0, scene.AudioClips[0].Duration.Seconds(), 0.01)
}

func TestBitReader(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		fn   func(t *testing.T, br *bitReader)
	}{
		{
			"ReadBits", []byte{0xFF, 0x00},
			func(t *testing.T, br *bitReader) {
				assert.Equal(t, uint32(0xFF), br.read(8))
				assert.Equal(t, uint32(0x00), br.read(8))
			},
		},
		{
			"ReadSigned", []byte{0xFF, 0xFE},
			func(t *testing.T, br *bitReader) {
				assert.Equal(t, int32(-1), br.readSigned(8))
			},
		},
		{
			"ReadUnary", []byte{0x20},
			func(t *testing.T, br *bitReader) {
				assert.Equal(t, 2, br.readUnary())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := newBitReader(tt.data)
			tt.fn(t, br)
		})
	}
}

func TestDecorrelate(t *testing.T) {
	tests := []struct {
		name  string
		mode  int
		in    [2][]int32
		wantL []int32
		wantR []int32
	}{
		{
			"LeftSide", channelLeftSide,
			[2][]int32{{100, 200}, {10, 20}},
			[]int32{100, 200}, []int32{90, 180},
		},
		{
			"RightSide", channelRightSide,
			[2][]int32{{10, 20}, {100, 200}},
			[]int32{110, 220}, []int32{100, 200},
		},
		{
			"MidSide", channelMidSide,
			[2][]int32{{100, 200}, {20, 40}},
			[]int32{110, 220}, []int32{90, 180},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := make([]int32, len(tt.in[0]))
			r := make([]int32, len(tt.in[1]))
			copy(l, tt.in[0])
			copy(r, tt.in[1])
			decorrelate([][]int32{l, r}, tt.mode)
			assert.Equal(t, tt.wantL, l)
			assert.Equal(t, tt.wantR, r)
		})
	}
}

func TestDecodeFixedCoeffs(t *testing.T) {
	tests := []struct {
		name  string
		order int
		len   int
	}{
		{"Order0", 0, 0},
		{"Order1", 1, 1},
		{"Order2", 2, 2},
		{"Order3", 3, 3},
		{"Order4", 4, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Len(t, fixedCoeffs[tt.order], tt.len)
		})
	}
}

func BenchmarkDecode_24bit_Mono(b *testing.B) {
	data, err := os.ReadFile("../testdata/mono_24bit.flac")
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

func TestDecodeRealFile(t *testing.T) {
	data, err := os.ReadFile("../testdata/minimal.flac")
	if err != nil {
		t.Skip("testdata not available")
	}
	scene := decodeOK(t, data)
	assert.Equal(t, ir.AudioFLAC, scene.AudioClips[0].Format)
}

func TestDecodeMinimalInput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"Empty", nil},
		{"TooShort", []byte("fLa")},
	}
	d := &Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := d.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			if err != nil {
				return
			}
			require.Len(t, scene.AudioClips, 1)
			assert.Equal(t, 0, scene.AudioClips[0].SampleRate)
		})
	}
}

func TestDecodeRejectsOversizedAndInvalidInput(t *testing.T) {
	d := &Decoder{}

	_, err := d.Decode(bytes.NewReader([]byte("fLaC")), detect.DecodeOptions{MaxFileSize: 2})
	require.Error(t, err)

	_, err = d.Decode(bytes.NewReader([]byte("RIFF")), detect.DecodeOptions{})
	require.Error(t, err)
}

func TestParseMetadataStopsOnTruncatedBlock(t *testing.T) {
	data := append([]byte("fLaC"), 0x00, 0x00, 0x00, 0x10)
	info, meta, pos := parseMetadata(data)
	assert.Equal(t, flacInfo{}, info)
	assert.Equal(t, ir.AudioMetadata{}, meta)
	assert.Equal(t, magicLen+metaHeaderSize, pos)
}

func TestParseMetadataShortInput(t *testing.T) {
	info, meta, pos := parseMetadata([]byte("fLaC"))
	assert.Equal(t, flacInfo{}, info)
	assert.Equal(t, ir.AudioMetadata{}, meta)
	assert.Equal(t, 4, pos)
}

func TestDecodeRespectsSampleLimit(t *testing.T) {
	data, err := os.ReadFile("../testdata/stereo_16bit.flac")
	if err != nil {
		t.Skip("testdata not available")
	}

	scene, err := (&Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{MaxAudioSamples: 1})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	_, err = scene.AudioClips[0].DecodeSamples()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audio sample limit exceeded")
}

// ─── Helpers ────────────────────────────────────────────────────────

func decodeOK(t *testing.T, data []byte) *ir.Asset {
	t.Helper()
	scene, err := (&Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	return scene
}

type metaBlock struct {
	typ  byte
	data []byte
}

func buildFLACMetadata(sampleRate, channels, bps, totalSamples int) []byte {
	return buildFLACWithBlocks(sampleRate, channels, bps, totalSamples, nil)
}

func buildFLACWithBlocks(sampleRate, channels, bps, totalSamples int, extra []metaBlock) []byte {
	var buf bytes.Buffer
	buf.Write(magic)

	si := buildStreamInfo(sampleRate, channels, bps, totalSamples)
	isLast := byte(0)
	if len(extra) == 0 {
		isLast = metaLastMask
	}
	writeMetaBlock(&buf, isLast|streamInfoType, si)

	for i, b := range extra {
		last := byte(0)
		if i == len(extra)-1 {
			last = metaLastMask
		}
		writeMetaBlock(&buf, last|b.typ, b.data)
	}
	return buf.Bytes()
}

func writeMetaBlock(buf *bytes.Buffer, header byte, data []byte) {
	buf.WriteByte(header)
	l := len(data)
	buf.WriteByte(byte(l >> 16))
	buf.WriteByte(byte(l >> 8))
	buf.WriteByte(byte(l))
	buf.Write(data)
}

func buildStreamInfo(sampleRate, channels, bps, totalSamples int) []byte {
	si := make([]byte, streamInfoSize)
	binary.BigEndian.PutUint16(si[0:], 4096)
	binary.BigEndian.PutUint16(si[2:], 4096)

	// Bytes 10-13: sample_rate(20) | channels(3) | bps(5) | total_samples_high(4)
	v := uint32(sampleRate)<<srShift |
		uint32(channels-1)<<channelShift |
		uint32(bps-1)<<bitsShift |
		uint32(totalSamples>>totalShift)&uint32(totalSampleMask)
	binary.BigEndian.PutUint32(si[10:], v)

	// Bytes 14-17: total_samples_low(32)
	binary.BigEndian.PutUint32(si[14:], uint32(totalSamples))
	return si
}

func buildVorbisComment(tags map[string]string) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(tags)))
	for k, v := range tags {
		comment := k + "=" + v
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(comment)))
		buf.WriteString(comment)
	}
	return buf.Bytes()
}

func buildPictureBlock(mime, desc string, picData []byte) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(3))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(mime)))
	buf.WriteString(mime)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(desc)))
	buf.WriteString(desc)
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(picData)))
	buf.Write(picData)
	return buf.Bytes()
}

func TestDecodeStereo16(t *testing.T) {
	data, err := os.ReadFile("../testdata/stereo_16bit.flac")
	if err != nil {
		t.Skip("testdata not available")
	}
	d := &Decoder{}
	scene, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioFLAC, clip.Format)
	assert.Equal(t, 44100, clip.SampleRate)
	assert.Equal(t, ir.LayoutStereo, clip.Layout)
	assert.Equal(t, ir.BitDepth16, clip.BitDepth)
	samples, err := clip.DecodeSamples()
	require.NoError(t, err)
	assert.Greater(t, len(samples), 0)
}

func TestBitReaderReset(t *testing.T) {
	data := []byte{0xAB, 0xCD, 0xEF}
	br := newBitReader(data)
	br.read(4)
	br.reset(data[1:])
	assert.Equal(t, uint32(0xCD), br.read(8))
}

func TestBitReaderAlignByte(t *testing.T) {
	data := []byte{0xFF, 0xAA}
	br := newBitReader(data)
	br.read(3)     // read 3 bits
	br.alignByte() // skip remaining 5 bits
	assert.Equal(t, uint32(0xAA), br.read(8))
}

func TestDecodeFramesTruncated(t *testing.T) {
	// Build valid metadata but provide no frame data
	data := buildFLACMetadata(44100, 2, 16, 100)
	// Append a truncated frame sync code
	data = append(data, 0xFF, 0xF8)

	scene := decodeOK(t, data)
	// Should decode metadata without crashing, samples may be empty
	assert.Equal(t, 44100, scene.AudioClips[0].SampleRate)
}

func TestSkipUTF8Number(t *testing.T) {
	// 1-byte UTF-8 (0xxxxxxx)
	br := newBitReader([]byte{0x42, 0x00, 0x00})
	skipUTF8Number(br)
	// After 1-byte UTF-8, reader consumed 1 byte
	assert.Equal(t, uint32(0x00), br.read(8))

	// 2-byte UTF-8 (110xxxxx 10xxxxxx)
	br2 := newBitReader([]byte{0xC2, 0x80, 0xEE})
	skipUTF8Number(br2)
	// After 2-byte UTF-8, reader consumed 2 bytes
	assert.Equal(t, uint32(0xEE), br2.read(8))
}

func TestNeedsExtraBit(t *testing.T) {
	assert.True(t, needsExtraBit(channelLeftSide, 1))
	assert.False(t, needsExtraBit(channelLeftSide, 0))
	assert.True(t, needsExtraBit(channelRightSide, 0))
	assert.False(t, needsExtraBit(channelRightSide, 1))
	assert.True(t, needsExtraBit(channelMidSide, 1))
	assert.False(t, needsExtraBit(channelMidSide, 0))
	assert.False(t, needsExtraBit(0, 0))
}
