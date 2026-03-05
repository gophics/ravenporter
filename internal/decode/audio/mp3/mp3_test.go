package mp3

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/testutil"
	"github.com/gophics/ravenporter/ir"
)

func TestRegistered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatMP3)
	assert.True(t, ok)
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"ID3v2", magicID3, true},
		{"SyncWord", []byte{0xFF, 0xFB, 0x90}, true},
		{"RIFF", []byte("RIFF"), false},
		{"Short", []byte{0xFF}, false},
		{"Empty", nil, false},
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

func TestParseFrameHeader(t *testing.T) {
	tests := []struct {
		name    string
		version byte
		srIdx   byte
		brIdx   byte
		chMode  byte
		wantSR  int
		wantCh  int
		wantBR  int
	}{
		{"MPEG1_44100_Stereo_128", 0x3, 0x0, 0x9, 0x0, 44100, 2, 128},
		{"MPEG1_48000_Mono_192", 0x3, 0x1, 0xB, 0x3, 48000, 1, 192},
		{"MPEG2_22050_Stereo_64", 0x2, 0x0, 0x5, 0x1, 22050, 2, 64},
		{"MPEG25_11025_Stereo_32", 0x0, 0x0, 0x1, 0x0, 11025, 2, 32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := buildFrame(tt.version, tt.srIdx, tt.brIdx, tt.chMode)
			info := parseFrameHeader(frame)
			assert.Equal(t, tt.wantSR, info.sampleRate)
			assert.Equal(t, tt.wantCh, info.channels)
			assert.Equal(t, tt.wantBR, info.bitrate)
		})
	}
}

func TestSkipID3v2(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantLen int
	}{
		{"NoID3", []byte{0xFF, 0xFB, 0x90, 0x00, 0x00}, 5},
		{"WithID3", buildID3v2(100), 0},
		{"Short", []byte("ID"), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := skipID3v2(tt.data)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestChannelLayouts(t *testing.T) {
	tests := []struct {
		name     string
		chMode   byte
		channels int
		layout   ir.ChannelLayout
	}{
		{"Stereo", 0x0, 2, ir.LayoutStereo},
		{"JointStereo", 0x1, 2, ir.LayoutStereo},
		{"DualChannel", 0x2, 2, ir.LayoutStereo},
		{"Mono", 0x3, 1, ir.LayoutMono},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := buildFrame(0x3, 0x0, 0x9, tt.chMode)
			data := append(buildID3v2(0), frame...)
			scene := decodeOK(t, data)
			assert.Equal(t, tt.layout, scene.AudioClips[0].Layout)
		})
	}
}

func TestDuration(t *testing.T) {
	frame := buildFrame(0x3, 0x0, 0x9, 0x0)
	frameSize := (spfLayer3MPEG1 / 8 * 128 * bitrateMultiple) / 44100
	data := bytes.Repeat(frame, 1)
	padded := make([]byte, frameSize)
	copy(padded, data)

	dur := calcDuration(padded, 44100)
	assert.Greater(t, dur.Milliseconds(), int64(0))
}

func BenchmarkDecode_16bit_Stereo(b *testing.B) {
	data, err := os.ReadFile("../testdata/stereo_16bit.mp3")
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
	data, err := os.ReadFile("../testdata/minimal.mp3")
	if err != nil {
		t.Skip("testdata not available")
	}
	scene := decodeOK(t, data)
	assert.Equal(t, ir.AudioMP3, scene.AudioClips[0].Format)
}

func TestDecodeStereo16(t *testing.T) {
	data, err := os.ReadFile("../testdata/stereo_16bit.mp3")
	if err != nil {
		t.Skip("testdata not available")
	}
	scene := decodeOK(t, data)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioMP3, clip.Format)
	assert.Equal(t, 44100, clip.SampleRate)
	assert.Equal(t, ir.LayoutStereo, clip.Layout)
	samples, err := clip.DecodeSamples()
	require.NoError(t, err)
	assert.Greater(t, len(samples), 0)
}

func TestDecodeMinimalInput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"Empty", nil},
		{"TooShort", []byte{0xFF}},
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

func TestSampleRates(t *testing.T) {
	tests := []struct {
		name    string
		version byte
		srIdx   byte
		wantSR  int
	}{
		{"MPEG1_44100", 0x3, 0x0, 44100},
		{"MPEG1_48000", 0x3, 0x1, 48000},
		{"MPEG1_32000", 0x3, 0x2, 32000},
		{"MPEG2_22050", 0x2, 0x0, 22050},
		{"MPEG2_24000", 0x2, 0x1, 24000},
		{"MPEG2_16000", 0x2, 0x2, 16000},
		{"MPEG25_11025", 0x0, 0x0, 11025},
		{"MPEG25_12000", 0x0, 0x1, 12000},
		{"MPEG25_8000", 0x0, 0x2, 8000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := buildFrame(tt.version, tt.srIdx, 0x9, 0x0)
			info := parseFrameHeader(frame)
			assert.Equal(t, tt.wantSR, info.sampleRate)
		})
	}
}

func TestBitrates(t *testing.T) {
	tests := []struct {
		name   string
		brIdx  byte
		wantBR int
	}{
		{"32kbps", 0x1, 32},
		{"128kbps", 0x9, 128},
		{"320kbps", 0xE, 320},
		{"Free", 0x0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := buildFrame(0x3, 0x0, tt.brIdx, 0x0)
			info := parseFrameHeader(frame)
			assert.Equal(t, tt.wantBR, info.bitrate)
		})
	}
}

// Helpers.

func decodeOK(t *testing.T, data []byte) *ir.Asset {
	t.Helper()
	scene, err := (&Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	return scene
}

func buildFrame(version, srIdx, brIdx, chMode byte) []byte {
	b1 := syncMask1 | (version << versionShift) | 0x01
	b2 := (brIdx << bitrateShift) | (srIdx << srShift)
	b3 := chMode << channelShift
	return []byte{syncByte0, b1, b2, b3, 0x00}
}

func buildID3v2(bodySize int) []byte {
	var buf bytes.Buffer
	buf.Write(magicID3)
	buf.WriteByte(3)
	buf.WriteByte(0)
	buf.WriteByte(0)

	s := uint32(bodySize)
	buf.WriteByte(byte((s >> id3v2Shift0) & id3v2SizeMask))
	buf.WriteByte(byte((s >> id3v2Shift1) & id3v2SizeMask))
	buf.WriteByte(byte((s >> id3v2Shift2) & id3v2SizeMask))
	buf.WriteByte(byte(s & id3v2SizeMask))

	body := make([]byte, bodySize)
	buf.Write(body)

	return buf.Bytes()
}

func TestDecoderProperties(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, formatName, d.FormatName())
	assert.Equal(t, []string{extMP3}, d.Extensions())
}

func TestVBRHeaders(t *testing.T) {
	// MPEG-1 Stereo: offset is 4+32 = 36 bytes for Xing header
	data := make([]byte, 100)
	copy(data, buildFrame(3, 0, 1, 0)) // MPEG1, 44100, 32kbps, Stereo

	// Inject Xing header
	copy(data[36:], "Xing")
	data[40] = 0x00 // Flags byte 0
	data[41] = 0x00
	data[42] = 0x00
	data[43] = 0x03 // frames & bytes flags

	// Frames: 100
	data[44] = 0
	data[45] = 0
	data[46] = 0
	data[47] = 100
	// Bytes: 4000
	data[48] = 0
	data[49] = 0
	data[50] = 0x0F
	data[51] = 0xA0

	info := parseFrameHeader(data)
	assert.Equal(t, int64(100), info.vbrFrames)
	assert.Equal(t, int64(4000), info.vbrBytes)
}

func TestVBRVBRI(t *testing.T) {
	// VBRI header is 36 bytes after frame header
	data := make([]byte, 100)
	copy(data, buildFrame(3, 0, 1, 0))

	copy(data[36:], "VBRI")

	// Bytes at offset 10: 4000 (0x0FA0)
	data[46] = 0
	data[47] = 0
	data[48] = 0x0F
	data[49] = 0xA0
	// Frames at offset 14: 100
	data[50] = 0
	data[51] = 0
	data[52] = 0
	data[53] = 100

	info := parseFrameHeader(data)
	assert.Equal(t, int64(100), info.vbrFrames)
	assert.Equal(t, int64(4000), info.vbrBytes)
}

func TestFreeFormatLength(t *testing.T) {
	data := make([]byte, 200)
	// Frame 1: index 0, Free Format (br=0)
	copy(data[0:], buildFrame(3, 0, 0, 0))
	// Frame 2: index 150, Free Format
	copy(data[150:], buildFrame(3, 0, 0, 0))

	// Because we use a free-format frame, decodeAllFrames / calcTotalDuration will
	// look for the next sync word to measure distance.
	lenFrames, ok := parseFreeFormatLength(data, 0, 3) // version 3
	assert.True(t, ok)
	assert.Equal(t, 150, lenFrames)
}

func TestCheckCRC(t *testing.T) {
	sideInfo := []byte{0x00, 0x00, 0x00, 0x00}
	_ = checkCRC(0x00, 0x00, sideInfo, 0)

	// Valid self-check: compute CRC for known input
	b2, b3 := byte(0xFB), byte(0x90)
	si := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	crc := uint16(0xFFFF)
	for _, b := range []byte{b2, b3} {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x8005
			} else {
				crc <<= 1
			}
		}
	}
	for _, b := range si {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x8005
			} else {
				crc <<= 1
			}
		}
	}
	assert.True(t, checkCRC(b2, b3, si, crc))
	assert.False(t, checkCRC(b2, b3, si, crc^0xFFFF))
}

func TestDecodeFrameWithCRC(t *testing.T) {
	b1 := byte(syncMask1 | (0x3 << versionShift))
	b2 := byte((0x9 << bitrateShift) | (0x0 << srShift))
	b3 := byte(0x0 << channelShift)
	frame := []byte{syncByte0, b1, b2, b3}

	frame = append(frame, 0x00, 0x00)

	frameLen := 144 * 128 * 1000 / 44100
	padded := make([]byte, frameLen)
	copy(padded, frame)

	d := &Decoder{}
	scene, _ := d.Decode(bytes.NewReader(padded), detect.DecodeOptions{})
	require.NotNil(t, scene)
}

func TestCalcDurationMPEG2(t *testing.T) {
	frame := buildFrame(0x2, 0x0, 0x5, 0x0)
	frameSize := (spfLayer3MPEG2 / 8 * 64 * bitrateMultiple) / 22050
	data := make([]byte, frameSize*3)
	for i := range 3 {
		copy(data[i*frameSize:], frame)
	}
	dur := calcDuration(data, 22050)
	assert.Greater(t, dur.Milliseconds(), int64(0))
}

func TestCalcDurationVBR(t *testing.T) {
	data := make([]byte, 200)
	copy(data, buildFrame(3, 0, 0x9, 0))
	copy(data[36:], "Xing")
	data[43] = 0x03
	data[44], data[45], data[46], data[47] = 0, 0, 0, 50
	data[48], data[49], data[50], data[51] = 0, 0, 0x1F, 0x40

	dur := calcDuration(data, 44100)
	assert.Greater(t, dur.Milliseconds(), int64(0))
}
