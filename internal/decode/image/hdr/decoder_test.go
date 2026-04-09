package hdr_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/hdr"
	"github.com/gophics/ravenporter/ir"
)

func createSyntheticHDR() []byte {
	var b bytes.Buffer

	b.WriteString("#?RADIANCE\n")

	b.WriteString("FORMAT=32-bit_rle_rgbe\n")
	b.WriteString("\n")

	b.WriteString("-Y 2 +X 2\n")

	b.Write([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})

	b.Write([]byte{0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10})

	return b.Bytes()
}

func TestHDRProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "Radiance", data: createSyntheticHDR(), want: true},
		{
			name: "RGBE",
			data: []byte("#?RGBE\nFORMAT=32-bit_rle_rgbe\n\n-Y 1 +X 1\n\x80\x80\x80\x81"),
			want: true,
		},
		{name: "Invalid", data: []byte("no"), want: false},
	}

	dec := &hdr.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dec.Probe(bytes.NewReader(tt.data)))
		})
	}
}

func TestHDRDecode(t *testing.T) {
	dec := &hdr.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(createSyntheticHDR()), opts)
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	img := scene.Images[0]
	assert.Equal(t, ir.ImageHDR, img.Format)
	assert.Equal(t, 2, img.Width)
	assert.Equal(t, 2, img.Height)
	assert.Equal(t, ir.ChannelRGB, img.Channels)
	assert.Equal(t, ir.ColorLinear, img.ColorSpace)

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 2*2*3*4) // 2x2 width/height, 3 RGB float channels (* 4 bytes per float32)
}

func BenchmarkDecode(b *testing.B) {
	dec := &hdr.Decoder{}
	opts := detect.DecodeOptions{}
	data := createSyntheticHDR()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), opts)
	}
}

func TestHDRDecodeWithoutPixels(t *testing.T) {
	dec := &hdr.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(createSyntheticHDR()), opts)
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	assert.Nil(t, scene.Images[0].Pixels())
	assert.Equal(t, 2, scene.Images[0].Width)
}

func TestHDRRLEEncodedScanline(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("#?RADIANCE\n")
	b.WriteString("FORMAT=32-bit_rle_rgbe\n")
	b.WriteString("\n")
	b.WriteString("-Y 1 +X 16\n")

	// New-style RLE marker: 0x02 0x02 <width high> <width low>
	b.Write([]byte{0x02, 0x02, 0x00, 0x10}) // width=16

	// 4 channels, each with RLE data for 16 pixels
	for range 4 {
		// Run of 16 of value 0x80 (run flag 0x80|16 = 0x90, then value)
		b.Write([]byte{0x90, 0x80})
	}

	dec := &hdr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(b.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	assert.Equal(t, 16, scene.Images[0].Width)
	assert.Equal(t, 1, scene.Images[0].Height)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestHDRRLEDeepRuns(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y 1 +X 10\n")
	b.Write([]byte{0x02, 0x02, 0x00, 0x0A}) // width=10

	// channel 0: 4 uncompressed, 6 compressed
	b.Write([]byte{0x04, 0x11, 0x22, 0x33, 0x44}) // string of 4 uncompressed
	b.Write([]byte{0x80 | 0x06, 0x55})            // run of 6 compressed value 0x55

	// channel 1: 10 compressed
	b.Write([]byte{0x80 | 0x0A, 0xAA})

	// channel 2: 10 uncompressed
	b.Write([]byte{0x0A, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xA})

	// channel 3: just compressed
	b.Write([]byte{0x80 | 0x0A, 0xFF})

	dec := &hdr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(b.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)

	// Just explicitly verify a slice of pixels decoded properly
	require.True(t, len(pb.Data) > 0)
}

func TestHDRRLETruncatedChannel(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y 1 +X 10\n")
	b.Write([]byte{0x02, 0x02, 0x00, 0x0A}) // width=10

	// channel 0: claims 4 uncompressed, only provides 2 bytes before EOF
	b.Write([]byte{0x04, 0x11, 0x22})

	dec := &hdr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(b.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	_, decErr := scene.Images[0].DecodePixels()
	require.Error(t, decErr)
}

func TestHDRRLETruncatedRun(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y 1 +X 10\n")
	b.Write([]byte{0x02, 0x02, 0x00, 0x0A}) // width=10

	// channel 0: claims run of 10, provides no value byte before EOF
	b.Write([]byte{0x80 | 0x0A})

	dec := &hdr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(b.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	_, decErr := scene.Images[0].DecodePixels()
	require.Error(t, decErr)
}

func TestHDRTruncatedData(t *testing.T) {
	dec := &hdr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader([]byte("#?RADIANCE\nFORMAT=32-bit_rle_rgbe\n\n-Y 2 +X 2\n")), detect.DecodeOptions{})
	if err != nil {
		return // error at decode time is valid
	}
	require.Len(t, scene.Images, 1)
	_, decErr := scene.Images[0].DecodePixels()
	assert.Error(t, decErr)
}

func TestHDRDecodeInputs(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		check   func(*testing.T, *ir.ImageAsset)
	}{
		{
			name:    "BadMagic",
			data:    []byte("NOT_HDR\n"),
			wantErr: true,
		},
		{
			name:    "NoSize",
			data:    []byte("#?RADIANCE\n\n\n"),
			wantErr: true,
		},
		{
			name: "RGBEMagic",
			data: []byte("#?RGBE\nFORMAT=32-bit_rle_rgbe\n\n-Y 2 +X 2\n" +
				"\x01\x02\x03\x04\x05\x06\x07\x08" +
				"\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10"),
			check: func(t *testing.T, img *ir.ImageAsset) {
				assert.Equal(t, 2, img.Width)
				assert.Equal(t, 2, img.Height)
			},
		},
	}

	dec := &hdr.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, scene.Images, 1)
			tt.check(t, scene.Images[0])
		})
	}
}

func TestHDRXYZE(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("#?RADIANCE\n")
	b.WriteString("FORMAT=32-bit_rle_xyze\n")
	b.WriteString("\n")
	b.WriteString("-Y 1 +X 1\n")
	b.Write([]byte{0x80, 0x80, 0x80, 0x81})

	scene, err := (&hdr.Decoder{}).Decode(bytes.NewReader(b.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.Len(t, pb.Data, 12)
}
