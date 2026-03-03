package wav

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
)

func TestWAV_Registered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatWAV)
	assert.True(t, ok)
}

func TestWAV_Probe(t *testing.T) {
	d := &Decoder{}
	riff := []byte("RIFF\x00\x00\x00\x00WAVE")
	assert.True(t, d.Probe(bytes.NewReader(riff)))
	assert.False(t, d.Probe(bytes.NewReader([]byte("fLaC"))))
}

func TestWAV_ParseWAV(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{"Valid WAV", []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x02\x00\x44\xac\x00\x00\x10\xb1\x02\x00\x04\x00\x10\x00data\x00\x00\x00\x00"), false},
		{"Not RIFF", []byte("OggS\x24\x00\x00\x00WAVEfmt "), true},
		{"Not WAVE", []byte("RIFF\x24\x00\x00\x00AVI fmt "), true},
		{"Missing FMT", []byte("RIFF\x24\x00\x00\x00WAVEdata\x00\x00\x00\x00"), true},
		{"Missing DATA", []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x02\x00\x44\xac\x00\x00\x10\xb1\x02\x00\x04\x00\x10\x00"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr, err := parseWAV(context.Background(), bytes.NewReader(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, hdr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, hdr)
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	data, err := os.ReadFile("../testdata/minimal.wav")
	if err != nil {
		b.Skip("testdata not available")
	}

	d := &Decoder{}
	r := bytes.NewReader(data)
	opts := detect.DecodeOptions{}

	if _, err := d.Decode(r, opts); err != nil {
		b.Skip("testdata not decodable: " + err.Error())
	}
	b.ReportAllocs()

	for b.Loop() {
		r.Reset(data)
		_, _ = d.Decode(r, opts)
	}
}

type testReadSeeker struct{ *bytes.Reader }

func (t *testReadSeeker) ReadAt(p []byte, off int64) (n int, err error) {
	return t.Reader.ReadAt(p, off)
}

func TestADPCM_Decode(t *testing.T) {
	data := make([]byte, 256)
	data[0] = 0
	data[1] = 10
	data[2] = 0
	data[3] = 0
	data[4] = 0
	data[5] = 0
	data[6] = 0
	for i := 7; i < 256; i++ {
		data[i] = 0xAA
	}

	hdr := &wavHeader{
		audioFormat:     fmtADPCM,
		numChannels:     1,
		sampleRate:      8000,
		bitsPerSample:   4,
		blockAlign:      256,
		samplesPerBlock: 500,
		adpcmCoeffs:     [][2]int{{256, 0}, {512, -256}},
		r:               &testReadSeeker{bytes.NewReader(data)},
		dataSize:        uint32(len(data)),
	}

	samples, err := decodeADPCMSamples(hdr)
	assert.NoError(t, err)
	assert.NotNil(t, samples)
	assert.Equal(t, 500, len(samples))
}

func BenchmarkADPCM_Decode(b *testing.B) {
	data := make([]byte, 256*100)
	for i := 0; i < len(data); i += 256 {
		data[i] = 0
		data[i+1] = 10
	}

	hdr := &wavHeader{
		audioFormat:     fmtADPCM,
		numChannels:     1,
		sampleRate:      8000,
		bitsPerSample:   4,
		blockAlign:      256,
		samplesPerBlock: 500,
		adpcmCoeffs:     [][2]int{{256, 0}, {512, -256}},
		r:               &testReadSeeker{bytes.NewReader(data)},
		dataSize:        uint32(len(data)),
	}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = hdr.r.Seek(0, io.SeekStart)
		_, _ = decodeADPCMSamples(hdr)
	}
}

func TestExtensionsAndFormat(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, []string{".wav"}, d.Extensions())
	assert.Equal(t, "WAV", d.FormatName())
}

func buildWAV(audioFmt, channels uint16, sampleRate uint32, bps uint16, pcm, extra []byte) []byte {
	fmtSize := uint32(16)
	dataSize := uint32(len(pcm))
	riffSize := 4 + 8 + fmtSize + 8 + dataSize + uint32(len(extra))

	var buf bytes.Buffer
	buf.Write([]byte("RIFF"))
	writeU32LE(&buf, riffSize)
	buf.Write([]byte("WAVE"))

	buf.Write([]byte("fmt "))
	writeU32LE(&buf, fmtSize)
	writeU16LE(&buf, audioFmt)
	writeU16LE(&buf, channels)
	writeU32LE(&buf, sampleRate)
	bytesPerSec := sampleRate * uint32(channels) * uint32(bps) / 8
	writeU32LE(&buf, bytesPerSec)
	blockAlign := channels * bps / 8
	writeU16LE(&buf, blockAlign)
	writeU16LE(&buf, bps)

	buf.Write(extra)

	buf.Write([]byte("data"))
	writeU32LE(&buf, dataSize)
	buf.Write(pcm)

	return buf.Bytes()
}

func writeU16LE(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
}

func writeU32LE(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 24))
}

func TestWAV_PCM8bit(t *testing.T) {
	pcm := []byte{128, 255, 0}
	data := buildWAV(1, 1, 44100, 8, pcm, nil)
	scene := wavDecodeOK(t, data)
	assert.Equal(t, ir.BitDepth8, scene.AudioClips[0].BitDepth)
	assert.InDelta(t, 0.0, getSamples(t, scene.AudioClips[0])[0], 0.02)
}

func TestWAV_PCM32bit(t *testing.T) {
	pcm := make([]byte, 4)
	pcm[0], pcm[1], pcm[2], pcm[3] = 0xFF, 0xFF, 0xFF, 0x7F // max int32 LE
	data := buildWAV(1, 1, 44100, 32, pcm, nil)
	scene := wavDecodeOK(t, data)
	assert.InDelta(t, 1.0, getSamples(t, scene.AudioClips[0])[0], 0.02)
}

func TestWAV_IEEEFloat(t *testing.T) {
	pcm := []byte{0x00, 0x00, 0x00, 0x3F} // 0.5 in float32 LE
	data := buildWAV(3, 1, 44100, 32, pcm, nil)
	scene := wavDecodeOK(t, data)
	assert.InDelta(t, 0.5, getSamples(t, scene.AudioClips[0])[0], 0.02)
}

func TestWAV_Alaw(t *testing.T) {
	pcm := []byte{0xD5, 0x55}
	data := buildWAV(6, 1, 8000, 8, pcm, nil)
	scene := wavDecodeOK(t, data)
	assert.NotEqual(t, float32(0), getSamples(t, scene.AudioClips[0])[0])
}

func TestWAV_Ulaw(t *testing.T) {
	pcm := []byte{0xFF, 0x00}
	data := buildWAV(7, 1, 8000, 8, pcm, nil)
	scene := wavDecodeOK(t, data)
	assert.Len(t, getSamples(t, scene.AudioClips[0]), 2)
}

func TestWAV_LISTMetadata(t *testing.T) {
	var list bytes.Buffer
	list.Write([]byte("LIST"))
	infoData := buildINFO()
	writeU32LE(&list, uint32(len(infoData)))
	list.Write(infoData)

	pcm := []byte{0, 0, 0, 0}
	data := buildWAV(1, 1, 44100, 16, pcm, list.Bytes())
	scene := wavDecodeOK(t, data)
	assert.Equal(t, "Test", scene.AudioClips[0].Metadata.Title)
}

func buildINFO() []byte {
	var buf bytes.Buffer
	buf.Write([]byte("INFO"))
	buf.Write([]byte("INAM"))
	writeU32LE(&buf, 5)
	buf.Write([]byte("Test\x00"))
	buf.WriteByte(0) // pad
	return buf.Bytes()
}

func TestWAV_SmplLoop(t *testing.T) {
	var smpl bytes.Buffer
	smpl.Write([]byte("smpl"))
	smplData := make([]byte, 60)
	smplData[28] = 1 // numLoops = 1
	// loop start = 100
	smplData[44] = 100
	// loop end = 200
	smplData[48] = 200
	writeU32LE(&smpl, uint32(len(smplData)))
	smpl.Write(smplData)

	pcm := []byte{0, 0, 0, 0}
	data := buildWAV(1, 1, 44100, 16, pcm, smpl.Bytes())
	scene := wavDecodeOK(t, data)
	assert.Equal(t, 100, scene.AudioClips[0].LoopStart)
	assert.Equal(t, 200, scene.AudioClips[0].LoopEnd)
}

func TestWAV_PCM24bit(t *testing.T) {
	pcm := []byte{0xFF, 0xFF, 0x7F} // max int24 LE
	data := buildWAV(1, 1, 44100, 24, pcm, nil)
	scene := wavDecodeOK(t, data)
	assert.InDelta(t, 1.0, getSamples(t, scene.AudioClips[0])[0], 0.02)
}

func TestWAV_Stereo(t *testing.T) {
	pcm := []byte{0x00, 0x40, 0x00, 0xC0} // two 16-bit samples
	data := buildWAV(1, 2, 44100, 16, pcm, nil)
	scene := wavDecodeOK(t, data)
	assert.Equal(t, ir.LayoutStereo, scene.AudioClips[0].Layout)
}

func TestWAV_Extensible(t *testing.T) {
	// Build an extensible fmt chunk (format 0xFFFE) with PCM subformat
	fmtSize := uint32(40)
	dataSize := uint32(4)
	riffSize := 4 + 8 + fmtSize + 8 + dataSize

	var buf bytes.Buffer
	buf.Write([]byte("RIFF"))
	writeU32LE(&buf, riffSize)
	buf.Write([]byte("WAVE"))

	buf.Write([]byte("fmt "))
	writeU32LE(&buf, fmtSize)
	writeU16LE(&buf, 0xFFFE)    // extensible
	writeU16LE(&buf, 1)         // channels
	writeU32LE(&buf, 44100)     // sample rate
	writeU32LE(&buf, 44100*2)   // bytes/sec
	writeU16LE(&buf, 2)         // block align
	writeU16LE(&buf, 16)        // bits per sample
	writeU16LE(&buf, 22)        // cbSize
	writeU16LE(&buf, 16)        // validBitsPerSample
	writeU32LE(&buf, 0)         // channel mask
	writeU16LE(&buf, 1)         // subformat PCM
	buf.Write(make([]byte, 14)) // rest of GUID

	buf.Write([]byte("data"))
	writeU32LE(&buf, dataSize)
	buf.Write([]byte{0x00, 0x40, 0x00, 0xC0})

	d := &Decoder{}
	scene, err := d.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, scene)
}

func TestWAV_ADPCMFmt(t *testing.T) {
	var buf bytes.Buffer

	channels := uint16(1)
	sampleRate := uint32(8000)
	blockAlign := uint16(256)
	samplesPerBlock := uint16(500)
	numCoeffs := uint16(2)

	fmtSize := uint32(fmtCoeffDataOff + int(numCoeffs)*coeffPairSize)
	adpcmData := make([]byte, blockAlign)
	adpcmData[0] = 0
	adpcmData[1] = 10
	dataSize := uint32(len(adpcmData))
	riffSize := 4 + 8 + fmtSize + 8 + dataSize

	buf.Write([]byte("RIFF"))
	writeU32LE(&buf, riffSize)
	buf.Write([]byte("WAVE"))

	buf.Write([]byte("fmt "))
	writeU32LE(&buf, fmtSize)
	writeU16LE(&buf, fmtADPCM)
	writeU16LE(&buf, channels)
	writeU32LE(&buf, sampleRate)
	writeU32LE(&buf, sampleRate*uint32(blockAlign)/uint32(samplesPerBlock))
	writeU16LE(&buf, blockAlign)
	writeU16LE(&buf, 4)
	writeU16LE(&buf, uint16(fmtSize)-16) // cbSize
	writeU16LE(&buf, samplesPerBlock)
	writeU16LE(&buf, numCoeffs)
	writeU16LE(&buf, uint16(256)) // coeff1[0]
	writeU16LE(&buf, uint16(0))   // coeff1[1]
	writeU16LE(&buf, uint16(512)) // coeff2[0]
	neg256 := uint16(0xFF00)
	writeU16LE(&buf, neg256) // coeff2[1] is -256

	buf.Write([]byte("data"))
	writeU32LE(&buf, dataSize)
	buf.Write(adpcmData)

	scene := wavDecodeOK(t, buf.Bytes())
	assert.NotEmpty(t, getSamples(t, scene.AudioClips[0]))
}

func getSamples(t testing.TB, c *ir.AudioClip) []float32 {
	t.Helper()
	s, err := c.DecodeSamples()
	if err != nil {
		t.Fatalf("failed to decode samples: %v", err)
	}
	return s
}

func wavDecodeOK(t *testing.T, data []byte) *ir.Asset {
	t.Helper()
	d := &Decoder{}
	scene, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, scene)
	if len(scene.AudioClips) > 0 {
		_, err = scene.AudioClips[0].DecodeSamples()
		assert.NoError(t, err)
	}
	return scene
}

func minimalPCMWAV() []byte {
	var buf bytes.Buffer
	pcm := make([]byte, 8)
	riffSize := uint32(4 + 24 + 8 + len(pcm))
	buf.Write([]byte("RIFF"))
	writeU32LE(&buf, riffSize)
	buf.Write([]byte("WAVE"))
	buf.Write([]byte("fmt "))
	writeU32LE(&buf, 16)
	writeU16LE(&buf, 1)
	writeU16LE(&buf, 1)
	writeU32LE(&buf, 44100)
	writeU32LE(&buf, 88200)
	writeU16LE(&buf, 2)
	writeU16LE(&buf, 16)
	buf.Write([]byte("data"))
	writeU32LE(&buf, uint32(len(pcm)))
	buf.Write(pcm)
	return buf.Bytes()
}

func TestWAV_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := &Decoder{}
	opts := detect.DecodeOptions{Context: ctx}
	scene, err := d.Decode(bytes.NewReader(minimalPCMWAV()), opts)
	if err == nil && len(scene.AudioClips) > 0 {
		_, err = scene.AudioClips[0].DecodeSamples()
	}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestWAV_MaxAudioSamplesRejection(t *testing.T) {
	d := &Decoder{}
	opts := detect.DecodeOptions{MaxAudioSamples: 1}
	scene, err := d.Decode(bytes.NewReader(minimalPCMWAV()), opts)
	if err == nil && len(scene.AudioClips) > 0 {
		_, err = scene.AudioClips[0].DecodeSamples()
	}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "limit exceeded")
}

func TestWAV_HelperFunctions(t *testing.T) {
	assert.Equal(t, int64(4), padded(4))
	assert.Equal(t, int64(6), padded(5))
	assert.Error(t, decodePCM([]byte{0x00}, make([]float32, 1), 5))

	hdr := &wavHeader{loopStart: ir.NoIndex, loopEnd: ir.NoIndex}
	parseLISTChunk([]byte("BAD!"), hdr)
	assert.Empty(t, hdr.metadata.Title)

	parseSmplChunk([]byte{1, 2, 3}, hdr)
	assert.Equal(t, ir.NoIndex, hdr.loopStart)

	err := wavErr(errUnsupported)
	assert.Contains(t, err.Error(), "wav decode error")
}

func TestWAV_DurationAndFmtErrors(t *testing.T) {
	assert.Zero(t, (&wavHeader{}).duration())

	adpcm := &wavHeader{
		audioFormat:     fmtADPCM,
		sampleRate:      8000,
		blockAlign:      256,
		samplesPerBlock: 500,
		dataSize:        512,
	}
	assert.Positive(t, adpcm.duration())

	err := parseFmtChunk(bytes.NewReader(make([]byte, fmtMinSize)), fmtMinSize, &wavHeader{})
	assert.Error(t, err)
}

func TestParseChunksMissingData(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("fmt ")
	writeU32LE(&buf, 16)
	writeU16LE(&buf, 1)
	writeU16LE(&buf, 1)
	writeU32LE(&buf, 44100)
	writeU32LE(&buf, 88200)
	writeU16LE(&buf, 2)
	writeU16LE(&buf, 16)

	_, err := parseChunks(context.Background(), bytes.NewReader(buf.Bytes()), &wavHeader{loopStart: ir.NoIndex, loopEnd: ir.NoIndex})
	assert.Error(t, err)
}
