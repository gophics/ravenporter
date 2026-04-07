package dds_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/dds"
	"github.com/gophics/ravenporter/ir"
)

var ddsData []byte

func init() {
	var err error
	ddsData, err = os.ReadFile("../testdata/minimal.dds")
	if err != nil {
		panic("failed to load ddsData: " + err.Error())
	}
}

func TestDDSProbe(t *testing.T) {
	dec := &dds.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(ddsData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestDDSDecode(t *testing.T) {
	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(ddsData), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	img := scene.Images[0]
	assert.Equal(t, ir.ImageDDS, img.Format)
	assert.Equal(t, 64, img.Width)
	assert.Equal(t, 32, img.Height)
	assert.NotEmpty(t, img.Compressed)
}

func TestDDSCompressionAndMips(t *testing.T) {
	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(ddsData), detect.DecodeOptions{})
	require.NoError(t, err)

	img := scene.Images[0]
	assert.True(t, img.IsGPUCompressed())
	assert.GreaterOrEqual(t, img.MipLevels, 1)
}

func TestDDSSyntheticDXT5(t *testing.T) {
	data := make([]byte, 128)
	copy(data[0:4], "DDS ")

	data[12] = 128
	data[13] = 0
	data[14] = 0
	data[15] = 0
	data[16] = 0
	data[17] = 1
	data[18] = 0
	data[19] = 0
	data[28] = 3
	data[29] = 0
	data[30] = 0
	data[31] = 0
	data[80] = 4 // ddspfFourCC
	copy(data[84:84+4], "DXT5")

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	img := scene.Images[0]
	assert.Equal(t, 256, img.Width)
	assert.Equal(t, 128, img.Height)
	assert.Equal(t, 3, img.MipLevels)
	assert.Equal(t, ir.GPUCompressionBC3, img.CompressionFormat)
	assert.True(t, img.IsGPUCompressed())
}

func BenchmarkDecode(b *testing.B) {
	dec := &dds.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(ddsData), opts)
	}
}

func buildDDSHeader(w, h, mips int, flags uint32, fourCC string, bpp int, rMask, gMask, bMask, aMask uint32) []byte {
	hdr := make([]byte, 128)
	copy(hdr[0:4], "DDS ")
	hdr[4] = 124 // header size
	putU32LE(hdr[12:], uint32(h))
	putU32LE(hdr[16:], uint32(w))
	putU32LE(hdr[28:], uint32(mips))
	putU32LE(hdr[80:], flags)
	if fourCC != "" {
		copy(hdr[84:88], fourCC)
	}
	putU32LE(hdr[88:], uint32(bpp))
	putU32LE(hdr[92:], rMask)
	putU32LE(hdr[96:], gMask)
	putU32LE(hdr[100:], bMask)
	putU32LE(hdr[104:], aMask)
	return hdr
}

func putU32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func TestDDS_DX10_BC7(t *testing.T) {
	hdr := buildDDSHeader(4, 4, 3, 0x4, "DX10", 0, 0, 0, 0, 0)
	dx10 := make([]byte, 20)
	putU32LE(dx10[0:], 98) // DXGI_FORMAT_BC7_UNORM
	hdr = append(hdr, dx10...)
	hdr = append(hdr, make([]byte, 64)...)
	data := hdr

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, ir.GPUCompressionBC7, scene.Images[0].CompressionFormat)
}

func TestDDS_DX10_BC1(t *testing.T) {
	hdr := buildDDSHeader(4, 4, 1, 0x4, "DX10", 0, 0, 0, 0, 0)
	dx10 := make([]byte, 20)
	putU32LE(dx10[0:], 71) // DXGI_FORMAT_BC1_UNORM
	hdr = append(hdr, dx10...)
	data := hdr

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, ir.GPUCompressionBC1, scene.Images[0].CompressionFormat)
}

func TestDDS_DX10_BC6H(t *testing.T) {
	hdr := buildDDSHeader(4, 4, 1, 0x4, "DX10", 0, 0, 0, 0, 0)
	dx10 := make([]byte, 20)
	putU32LE(dx10[0:], 95) // DXGI_FORMAT_BC6H_UF16
	hdr = append(hdr, dx10...)
	data := hdr

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, ir.GPUCompressionBC6H, scene.Images[0].CompressionFormat)
}

func TestDDS_DX10_Truncated(t *testing.T) {
	hdr := buildDDSHeader(4, 4, 1, 0x4, "DX10", 0, 0, 0, 0, 0)
	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(hdr), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, ir.GPUCompressionNone, scene.Images[0].CompressionFormat)
}

func TestDDS_Uncompressed_32bit(t *testing.T) {
	w, h := 2, 2
	hdr := buildDDSHeader(w, h, 1, 0x40, "", 32,
		0x00FF0000, 0x0000FF00, 0x000000FF, 0xFF000000)

	pixels := make([]byte, w*h*4)
	pixels[0], pixels[1], pixels[2], pixels[3] = 0xFF, 0x00, 0x00, 0x80 // BGRA
	hdr = append(hdr, pixels...)
	data := hdr

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, w*h*4)
}

func TestDDS_Uncompressed_24bit(t *testing.T) {
	w, h := 2, 2
	hdr := buildDDSHeader(w, h, 1, 0x40, "", 24,
		0xFF0000, 0x00FF00, 0x0000FF, 0)

	pixels := make([]byte, w*h*3)
	pixels[0], pixels[1], pixels[2] = 0xFF, 0x00, 0x00
	hdr = append(hdr, pixels...)
	data := hdr

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDDS_Uncompressed_16bit(t *testing.T) {
	w, h := 2, 2
	hdr := buildDDSHeader(w, h, 1, 0x40, "", 16,
		0xF800, 0x07E0, 0x001F, 0)

	pixels := make([]byte, w*h*2)
	putU32LE(pixels[0:], 0xF800) // red pixel
	hdr = append(hdr, pixels...)
	data := hdr

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDDS_Uncompressed_8bit(t *testing.T) {
	w, h := 2, 2
	hdr := buildDDSHeader(w, h, 1, 0x20000, "", 8,
		0xFF, 0, 0, 0)

	pixels := make([]byte, w*h)
	pixels[0] = 128
	hdr = append(hdr, pixels...)
	data := hdr

	dec := &dds.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDDS_FourCC_Variants(t *testing.T) {
	tests := []struct {
		name   string
		fourCC string
		want   ir.GPUCompression
	}{
		{"DXT1", "DXT1", ir.GPUCompressionBC1},
		{"DXT3", "DXT3", ir.GPUCompressionBC2},
		{"ATI1", "ATI1", ir.GPUCompressionBC4},
		{"BC4U", "BC4U", ir.GPUCompressionBC4},
		{"ATI2", "ATI2", ir.GPUCompressionBC5},
		{"BC5U", "BC5U", ir.GPUCompressionBC5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := buildDDSHeader(4, 4, 1, 0x4, tt.fourCC, 0, 0, 0, 0, 0)
			dec := &dds.Decoder{}
			scene, err := dec.Decode(bytes.NewReader(hdr), detect.DecodeOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, scene.Images[0].CompressionFormat)
		})
	}
}
