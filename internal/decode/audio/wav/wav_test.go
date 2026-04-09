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
	"github.com/stretchr/testify/require"
)

func TestWAV_Registered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatWAV)
	assert.True(t, ok)
}

func TestWAV_Probe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "RIFF", data: []byte("RIFF\x00\x00\x00\x00WAVE"), want: true},
		{name: "RF64", data: []byte("RF64\xFF\xFF\xFF\xFFWAVE"), want: true},
		{name: "BW64", data: []byte("BW64\xFF\xFF\xFF\xFFWAVE"), want: true},
		{name: "Invalid", data: []byte("fLaC"), want: false},
	}

	dec := &Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dec.Probe(bytes.NewReader(tt.data)))
		})
	}
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
		dataSize:        int64(len(data)),
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
		dataSize:        int64(len(data)),
	}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = hdr.r.Seek(0, io.SeekStart)
		_, _ = decodeADPCMSamples(hdr)
	}
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

func writeU64LE(buf *bytes.Buffer, v uint64) {
	writeU32LE(buf, uint32(v))
	writeU32LE(buf, uint32(v>>32))
}

func TestWAV_DecodeVariants(t *testing.T) {
	buildExtensible := func() []byte {
		const fmtSize = uint32(40)
		const dataSize = uint32(4)
		riffSize := 4 + 8 + fmtSize + 8 + dataSize

		var buf bytes.Buffer
		buf.Write([]byte("RIFF"))
		writeU32LE(&buf, riffSize)
		buf.Write([]byte("WAVE"))

		buf.Write([]byte("fmt "))
		writeU32LE(&buf, fmtSize)
		writeU16LE(&buf, 0xFFFE)
		writeU16LE(&buf, 1)
		writeU32LE(&buf, 44100)
		writeU32LE(&buf, 44100*2)
		writeU16LE(&buf, 2)
		writeU16LE(&buf, 16)
		writeU16LE(&buf, 22)
		writeU16LE(&buf, 16)
		writeU32LE(&buf, 0x3)
		writeU16LE(&buf, 1)
		buf.Write(make([]byte, 14))

		buf.Write([]byte("data"))
		writeU32LE(&buf, dataSize)
		buf.Write([]byte{0x00, 0x40, 0x00, 0xC0})
		return buf.Bytes()
	}

	tests := []struct {
		name  string
		data  []byte
		check func(*testing.T, *ir.AudioClip)
	}{
		{
			name: "PCM8",
			data: buildWAV(1, 1, 44100, 8, []byte{128, 255, 0}, nil),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.Equal(t, ir.BitDepth8, clip.BitDepth)
				assert.InDelta(t, 0.0, getSamples(t, clip)[0], 0.02)
			},
		},
		{
			name: "PCM24",
			data: buildWAV(1, 1, 44100, 24, []byte{0xFF, 0xFF, 0x7F}, nil),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.InDelta(t, 1.0, getSamples(t, clip)[0], 0.02)
			},
		},
		{
			name: "PCM32",
			data: buildWAV(1, 1, 44100, 32, []byte{0xFF, 0xFF, 0xFF, 0x7F}, nil),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.InDelta(t, 1.0, getSamples(t, clip)[0], 0.02)
			},
		},
		{
			name: "IEEEFloat",
			data: buildWAV(3, 1, 44100, 32, []byte{0x00, 0x00, 0x00, 0x3F}, nil),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.InDelta(t, 0.5, getSamples(t, clip)[0], 0.02)
			},
		},
		{
			name: "ALaw",
			data: buildWAV(6, 1, 8000, 8, []byte{0xD5, 0x55}, nil),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.NotEqual(t, float32(0), getSamples(t, clip)[0])
			},
		},
		{
			name: "MuLaw",
			data: buildWAV(7, 1, 8000, 8, []byte{0xFF, 0x00}, nil),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.Len(t, getSamples(t, clip), 2)
			},
		},
		{
			name: "Stereo",
			data: buildWAV(1, 2, 44100, 16, []byte{0x00, 0x40, 0x00, 0xC0}, nil),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.Equal(t, ir.LayoutStereo, clip.Layout)
			},
		},
		{
			name: "Extensible",
			data: buildExtensible(),
			check: func(t *testing.T, clip *ir.AudioClip) {
				assert.Equal(t, uint32(0x3), clip.ChannelMask)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := wavDecodeOK(t, tt.data)
			tt.check(t, scene.AudioClips[0])
		})
	}
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

type cuePointSpec struct {
	id     uint32
	sample uint32
}

func buildCueChunk(count uint32, points ...cuePointSpec) []byte {
	var data bytes.Buffer
	writeU32LE(&data, count)
	for _, point := range points {
		writeU32LE(&data, point.id)
		writeU32LE(&data, 0)
		data.WriteString("data")
		writeU32LE(&data, 0)
		writeU32LE(&data, 0)
		writeU32LE(&data, point.sample)
	}

	var chunk bytes.Buffer
	chunk.WriteString("cue ")
	writeU32LE(&chunk, uint32(data.Len()))
	chunk.Write(data.Bytes())
	return chunk.Bytes()
}

func buildADTLLabelChunk(id uint32, label string) []byte {
	var labl bytes.Buffer
	writeU32LE(&labl, id)
	labl.WriteString(label)
	labl.WriteByte(0)

	var list bytes.Buffer
	list.WriteString("LIST")

	var data bytes.Buffer
	data.WriteString("adtl")
	data.WriteString("labl")
	writeU32LE(&data, uint32(labl.Len()))
	data.Write(labl.Bytes())
	if labl.Len()%2 != 0 {
		data.WriteByte(0)
	}

	writeU32LE(&list, uint32(data.Len()))
	list.Write(data.Bytes())
	return list.Bytes()
}

func buildWAV64(container string, channelMask uint32, pcm []byte, extra ...[]byte) []byte {
	fmtSize := uint32(40)
	dataSize := uint64(len(pcm))
	ds64Size := uint32(28)
	payloadSize := uint64(4 + 8 + fmtSize + 8)
	payloadSize += dataSize
	for _, chunk := range extra {
		payloadSize += uint64(len(chunk))
	}
	riffSize := payloadSize + 8 + uint64(ds64Size)

	var buf bytes.Buffer
	buf.WriteString(container)
	writeU32LE(&buf, ^uint32(0))
	buf.WriteString("WAVE")

	buf.WriteString("ds64")
	writeU32LE(&buf, ds64Size)
	writeU64LE(&buf, riffSize)
	writeU64LE(&buf, dataSize)
	writeU64LE(&buf, 0)
	writeU32LE(&buf, 0)

	buf.WriteString("fmt ")
	writeU32LE(&buf, fmtSize)
	writeU16LE(&buf, 0xFFFE)
	writeU16LE(&buf, 2)
	writeU32LE(&buf, 48000)
	writeU32LE(&buf, 48000*4)
	writeU16LE(&buf, 4)
	writeU16LE(&buf, 16)
	writeU16LE(&buf, 22)
	writeU16LE(&buf, 16)
	writeU32LE(&buf, channelMask)
	writeU16LE(&buf, 1)
	buf.Write(make([]byte, 14))

	for _, chunk := range extra {
		buf.Write(chunk)
	}

	buf.WriteString("data")
	writeU32LE(&buf, ^uint32(0))
	buf.Write(pcm)
	if len(pcm)%2 != 0 {
		buf.WriteByte(0)
	}

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

func TestWAV_CuePoints(t *testing.T) {
	pcm := []byte{0, 0, 0, 0}
	data := buildWAV(1, 1, 44100, 16, pcm, buildCueChunk(2,
		cuePointSpec{id: 7, sample: 100},
		cuePointSpec{id: 11, sample: 250},
	))

	scene := wavDecodeOK(t, data)
	require.Len(t, scene.AudioClips[0].Metadata.CuePoints, 2)
	assert.Equal(t, "7", scene.AudioClips[0].Metadata.CuePoints[0].Name)
	assert.Equal(t, 100, scene.AudioClips[0].Metadata.CuePoints[0].Sample)
	assert.Equal(t, "11", scene.AudioClips[0].Metadata.CuePoints[1].Name)
	assert.Equal(t, 250, scene.AudioClips[0].Metadata.CuePoints[1].Sample)
}

func TestWAV_CuePointsTruncatedRecordTable(t *testing.T) {
	pcm := []byte{0, 0, 0, 0}
	data := buildWAV(1, 1, 44100, 16, pcm, buildCueChunk(2, cuePointSpec{id: 9, sample: 320}))

	scene := wavDecodeOK(t, data)
	require.Len(t, scene.AudioClips[0].Metadata.CuePoints, 1)
	assert.Equal(t, "9", scene.AudioClips[0].Metadata.CuePoints[0].Name)
	assert.Equal(t, 320, scene.AudioClips[0].Metadata.CuePoints[0].Sample)
}

func TestWAV_CueLabels(t *testing.T) {
	pcm := []byte{0, 0, 0, 0}
	data := buildWAV(1, 1, 44100, 16, pcm,
		append(buildCueChunk(1, cuePointSpec{id: 7, sample: 100}), buildADTLLabelChunk(7, "Intro")...))

	scene := wavDecodeOK(t, data)
	require.Len(t, scene.AudioClips[0].Metadata.CuePoints, 1)
	assert.Equal(t, "Intro", scene.AudioClips[0].Metadata.CuePoints[0].Name)
	assert.Equal(t, 100, scene.AudioClips[0].Metadata.CuePoints[0].Sample)
}

func TestWAV_RF64AndBW64(t *testing.T) {
	pcm := []byte{0x00, 0x40, 0x00, 0xC0}
	for _, container := range []string{"RF64", "BW64"} {
		t.Run(container, func(t *testing.T) {
			scene := wavDecodeOK(t, buildWAV64(container, 0x3, pcm, buildADTLLabelChunk(0, "unused")))
			assert.Equal(t, uint32(0x3), scene.AudioClips[0].ChannelMask)
			assert.Equal(t, ir.LayoutStereo, scene.AudioClips[0].Layout)
		})
	}
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
	require.NoError(t, err)
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
