package ogg

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

func TestOGG_Registered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatOGG)
	assert.True(t, ok)
}

func TestOGG_Probe(t *testing.T) {
	d := &Decoder{}
	validHeader := append([]byte("OggS\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01\x1e\x01vorbis"), make([]byte, 20)...)
	assert.True(t, d.Probe(bytes.NewReader(validHeader)))
	assert.False(t, d.Probe(bytes.NewReader([]byte("RIFF"))))
}

func TestOGG_Decode(t *testing.T) {
	data, err := os.ReadFile("../testdata/minimal.ogg")
	if err != nil {
		t.Skip("testdata not available")
	}
	d := &Decoder{}
	scene, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioOGG, clip.Format)
}

func TestOGG_IdentificationHeader(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
		wantCh  int
		wantSR  int
	}{
		{
			name:   "Valid stereo 44100Hz",
			input:  makeIDHeader(2, 44100, 8, 11),
			wantCh: 2,
			wantSR: 44100,
		},
		{
			name:   "Valid mono 48000Hz",
			input:  makeIDHeader(1, 48000, 8, 11),
			wantCh: 1,
			wantSR: 48000,
		},
		{
			name:    "Too short",
			input:   []byte{0x01},
			wantErr: true,
		},
		{
			name:    "Bad magic",
			input:   append([]byte{0x01}, []byte("notvor")...),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var setup vorbisSetup
			err := readIdentificationHeader(tt.input, &setup)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCh, setup.channels)
			assert.Equal(t, tt.wantSR, setup.sampleRate)
		})
	}
}
func TestOGG_Extensions(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, []string{".ogg", ".oga"}, d.Extensions())
}

func TestOGG_FormatName(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, "OGG", d.FormatName())
}

func TestOGG_CommentHeader(t *testing.T) {
	hdr := make([]byte, 100)
	hdr[0] = headerCommentType
	copy(hdr[1:], vorbisString)

	// Vendor string length (4)
	binary.LittleEndian.PutUint32(hdr[7:], 4)
	copy(hdr[11:], "test")

	// User comment list length (1)
	binary.LittleEndian.PutUint32(hdr[15:], 1)

	// Comment 1 length (9)
	binary.LittleEndian.PutUint32(hdr[19:], 9)
	copy(hdr[23:], "KEY=VALUE")

	// Framing bit
	hdr[32] = 0x01

	var setup vorbisSetup
	err := readCommentHeader(hdr[:33], &setup)
	require.NoError(t, err)

	err = readCommentHeader([]byte{0x03}, &setup)
	assert.Error(t, err)
}

func TestOGG_Ilog(t *testing.T) {
	tests := []struct {
		name string
		v    int
		want int
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
		{"two", 2, 2},
		{"three", 3, 2},
		{"four", 4, 3},
		{"255", 255, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ilog(tt.v))
		})
	}
}

func TestOGG_BitReader(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		readBits []int
		want     []uint32
	}{
		{
			name:     "Read 1 bit",
			data:     []byte{0x01},
			readBits: []int{1},
			want:     []uint32{1},
		},
		{
			name:     "Read 8 bits",
			data:     []byte{0xAB},
			readBits: []int{8},
			want:     []uint32{0xAB},
		},
		{
			name:     "Read 4+4 bits",
			data:     []byte{0xF3},
			readBits: []int{4, 4},
			want:     []uint32{0x03, 0x0F},
		},
		{
			name:     "Read across bytes",
			data:     []byte{0xFF, 0x01},
			readBits: []int{9},
			want:     []uint32{0x1FF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := newBitReader(tt.data)
			for i, bits := range tt.readBits {
				got := br.readBits(bits)
				assert.Equal(t, tt.want[i], got)
			}
		})
	}
}

func TestOGG_HuffmanTree(t *testing.T) {
	lengths := []uint8{2, 2, 3, 3}
	tree, err := buildHuffmanTree(lengths)
	require.NoError(t, err)
	require.NotNil(t, tree)

	lengths2 := []uint8{0, 0, 0}
	tree2, err := buildHuffmanTree(lengths2)
	require.NoError(t, err)
	require.NotNil(t, tree2)
}

func TestOGG_Float32FromBits(t *testing.T) {
	assert.InDelta(t, float32(0.0), float32frombits(0), 1e-10)
	assert.InDelta(t, float32(1.0), float32frombits(0x62800001), 1e-6)
}

func TestOGG_ExpandLookupType2(t *testing.T) {
	var setup vorbisSetup
	setup.codebooks = make([]vorbisCodebook, 2)

	cb := &setup.codebooks[0]
	cb.dimensions = 2
	cb.entries = 2
	cb.minVal = 1.0
	cb.deltaVal = 2.0
	cb.multi = []uint32{0, 1, 2, 3}
	cb.lookupVals = make([]float32, 4)
	cb.seqP = 0

	expandLookupType2(cb)

	assert.InDelta(t, float32(1.0), cb.lookupVals[0], 0.001) // 1.0 + 0*2.0
	assert.InDelta(t, float32(3.0), cb.lookupVals[1], 0.001) // 1.0 + 1*2.0
	assert.InDelta(t, float32(5.0), cb.lookupVals[2], 0.001) // 1.0 + 2*2.0
	assert.InDelta(t, float32(7.0), cb.lookupVals[3], 0.001) // 1.0 + 3*2.0

	cb.seqP = 1
	expandLookupType2(cb)

	assert.InDelta(t, float32(1.0), cb.lookupVals[0], 0.001)
	assert.InDelta(t, float32(4.0), cb.lookupVals[1], 0.001)
	assert.InDelta(t, float32(5.0), cb.lookupVals[2], 0.001)
	assert.InDelta(t, float32(12.0), cb.lookupVals[3], 0.001)
}

func BenchmarkDecode(b *testing.B) {
	data, err := os.ReadFile("../testdata/minimal.ogg")
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

func TestOGG_SynthOverlapAdd(t *testing.T) {
	prev := []float32{1, 2, 3, 4}
	curr := []float32{10, 20}
	out := make([]float32, 2)

	synthOverlapAdd(out, prev, curr, 4, 2)
	// overlap = 2
	// halfPrev = 2
	// i=0 -> idxPrev = 2 + 0 - 1 = 1 -> prev[1] (2) + curr[0] (10) = 12
	// i=1 -> idxPrev = 2 + 1 - 1 = 2 -> prev[2] (3) + curr[1] (20) = 23
	assert.Equal(t, []float32{12, 23}, out)

	curr2 := []float32{10, 20, 30, 40}
	out2 := make([]float32, 4)
	synthOverlapAdd(out2, prev, curr2, 2, 4)
	// overlap = 2
	// halfPrev = 1
	// i=0 -> idxPrev = 1 + 0 - 1 = 0 -> prev[0] + curr2[0] = 11
	// i=1 -> idxPrev = 1 + 1 - 1 = 1 -> prev[1] + curr2[1] = 22
	assert.Equal(t, float32(11), out2[0])
	assert.Equal(t, float32(22), out2[1])
}

// --- helpers ---

func makeIDHeader(channels uint8, sampleRate uint32, bs0exp, bs1exp uint8) []byte {
	buf := make([]byte, minIDHeaderSize)
	buf[0] = headerIDType
	copy(buf[1:], vorbisString)
	binary.LittleEndian.PutUint32(buf[headerStringOff:], 0)
	buf[channelsByte] = channels
	binary.LittleEndian.PutUint32(buf[sampleRateOff:sampleRateEnd], sampleRate)
	buf[blocksizeByte] = (bs1exp << blocksizeHighShift) | (bs0exp & blocksizeLowMask)
	buf[framingByte] = 0x01
	return buf
}

func TestOGG_DecodeStereo16(t *testing.T) {
	data, err := os.ReadFile("../testdata/stereo_16bit.ogg")
	if err != nil {
		t.Skip("testdata not available")
	}
	d := &Decoder{}
	scene, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioOGG, clip.Format)
	assert.Equal(t, 44100, clip.SampleRate)
	assert.Equal(t, ir.LayoutStereo, clip.Layout)
	samples, err := clip.DecodeSamples()
	require.NoError(t, err)
	assert.Greater(t, len(samples), 0)
}

func TestOGG_BuildHuffmanTree_Empty(t *testing.T) {
	tree, err := buildHuffmanTree(nil)
	assert.NotNil(t, tree)
	assert.NoError(t, err)
}

func TestOGG_DecoderProperties(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, "OGG", d.FormatName())
	assert.Contains(t, d.Extensions(), ".ogg")
	assert.Contains(t, d.Extensions(), ".oga")
}
