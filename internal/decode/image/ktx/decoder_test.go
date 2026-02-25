package ktx_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/ktx"
	"github.com/gophics/ravenporter/ir"
)

var ktxData []byte

func init() {
	var err error
	ktxData, err = os.ReadFile("../testdata/minimal.ktx")
	if err != nil {
		panic("failed to load ktxData: " + err.Error())
	}
}

func putU32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func TestKTXProbe(t *testing.T) {
	dec := &ktx.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(ktxData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestKTXDecode(t *testing.T) {
	dec := &ktx.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(ktxData), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	img := scene.Images[0]
	assert.Equal(t, ir.ImageKTX, img.Format)
	assert.Equal(t, 128, img.Width)
	assert.Equal(t, 64, img.Height)
}

func TestKTXExtensions(t *testing.T) {
	dec := &ktx.Decoder{}
	exts := dec.Extensions()
	assert.Contains(t, exts, ".ktx")
	assert.Contains(t, exts, ".ktx2")
}

func TestKTXFormatName(t *testing.T) {
	dec := &ktx.Decoder{}
	assert.Equal(t, "KTX", dec.FormatName())
}

func TestKTX2SyntheticHeader(t *testing.T) {
	data := make([]byte, 80)
	copy(data[0:], "\xAB\x4B\x54\x58\x20\x32\x30\xBB\x0D\x0A\x1A\x0A")
	putU32LE(data[12:], 157)
	putU32LE(data[20:], 512)
	putU32LE(data[24:], 256)
	putU32LE(data[44:], 5)

	dec := &ktx.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	img := scene.Images[0]
	assert.Equal(t, 512, img.Width)
	assert.Equal(t, 256, img.Height)
	assert.Equal(t, 5, img.MipLevels)
	assert.Equal(t, ir.GPUCompressionASTC4x4, img.CompressionFormat)
	assert.True(t, img.IsGPUCompressed())
}

func TestKTX2ZstdSupercompression(t *testing.T) {
	data := make([]byte, 80)
	copy(data[0:], "\xAB\x4B\x54\x58\x20\x32\x30\xBB\x0D\x0A\x1A\x0A")

	putU32LE(data[12:], 157) // ASTC
	putU32LE(data[20:], 256)
	putU32LE(data[24:], 128)
	putU32LE(data[40:], 2) // Zstd supercompression
	putU32LE(data[44:], 1) // 1 mip level

	dec := &ktx.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	img := scene.Images[0]
	assert.Equal(t, 256, img.Width)
	assert.Equal(t, 128, img.Height)
	assert.True(t, img.IsGPUCompressed())
}

func BenchmarkDecode(b *testing.B) {
	dec := &ktx.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(ktxData), opts)
	}
}

func TestKTX1SyntheticHeader(t *testing.T) {
	data := make([]byte, 68)
	copy(data[0:], "\xAB\x4B\x54\x58\x20\x31\x31\xBB\x0D\x0A\x1A\x0A")
	putU32LE(data[36:], 64)
	putU32LE(data[40:], 32)

	dec := &ktx.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	assert.Equal(t, 64, img.Width)
	assert.Equal(t, 32, img.Height)
}

func TestKTXTruncatedHeader(t *testing.T) {
	dec := &ktx.Decoder{}
	_, err := dec.Decode(bytes.NewReader([]byte{0xAB, 0x4B}), detect.DecodeOptions{})
	assert.Error(t, err) // Should error on truncated file
}

func TestKTX2AllCompressionFormats(t *testing.T) {
	formats := []struct {
		vkFmt uint32
		want  ir.GPUCompression
	}{
		{131, ir.GPUCompressionBC1}, {132, ir.GPUCompressionBC1},
		{135, ir.GPUCompressionBC2}, {136, ir.GPUCompressionBC2},
		{137, ir.GPUCompressionBC3}, {138, ir.GPUCompressionBC3},
		{139, ir.GPUCompressionBC4}, {141, ir.GPUCompressionBC5},
		{143, ir.GPUCompressionBC6H}, {144, ir.GPUCompressionBC6H},
		{145, ir.GPUCompressionBC7}, {146, ir.GPUCompressionBC7},
		{147, ir.GPUCompressionETC2}, {148, ir.GPUCompressionETC2},
		{0, ir.GPUCompressionNone},
	}
	for _, f := range formats {
		data := make([]byte, 80)
		copy(data[0:], "\xAB\x4B\x54\x58\x20\x32\x30\xBB\x0D\x0A\x1A\x0A")
		putU32LE(data[12:], f.vkFmt)
		putU32LE(data[20:], 16)
		putU32LE(data[24:], 16)

		dec := &ktx.Decoder{}
		scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
		require.NoError(t, err)
		assert.Equal(t, f.want, scene.Images[0].CompressionFormat)
	}
}

func TestKTX1AllCompressionFormats(t *testing.T) {
	buildKTX1 := func(glFormat, glInternalFormat uint32) []byte {
		data := make([]byte, 68)
		copy(data[0:], "\xAB\x4B\x54\x58\x20\x31\x31\xBB\x0D\x0A\x1A\x0A")
		putU32LE(data[24:], glFormat)
		putU32LE(data[28:], glInternalFormat)
		putU32LE(data[36:], 16) // width
		putU32LE(data[40:], 16) // height
		return data
	}

	tests := []struct {
		name     string
		glFmt    uint32
		glIntFmt uint32
		want     ir.GPUCompression
	}{
		{"DXT1_RGB", 0, 0x83F0, ir.GPUCompressionBC1},
		{"DXT1_RGBA", 0, 0x83F1, ir.GPUCompressionBC1},
		{"DXT1_sRGB", 0, 0x8C4C, ir.GPUCompressionBC1},
		{"DXT3_RGBA", 0, 0x83F2, ir.GPUCompressionBC2},
		{"DXT3_sRGBA", 0, 0x8C4D, ir.GPUCompressionBC2},
		{"DXT5_RGBA", 0, 0x83F3, ir.GPUCompressionBC3},
		{"DXT5_sRGBA", 0, 0x8C4E, ir.GPUCompressionBC3},
		{"BPTC_BC6H_SF", 0, 0x8E8E, ir.GPUCompressionBC6H},
		{"BPTC_BC6H_UF", 0, 0x8E8F, ir.GPUCompressionBC6H},
		{"BPTC_BC7_UNORM", 0, 0x8E8C, ir.GPUCompressionBC7},
		{"BPTC_BC7_sRGB", 0, 0x8E8D, ir.GPUCompressionBC7},
		{"ETC2_RGB8", 0, 0x9274, ir.GPUCompressionETC2},
		{"ETC2_sRGB8", 0, 0x9275, ir.GPUCompressionETC2},
		{"ETC2_RGBA8", 0, 0x9278, ir.GPUCompressionETC2},
		{"ETC2_sRGBA8", 0, 0x9279, ir.GPUCompressionETC2},
		{"ASTC_4x4", 0, 0x93B0, ir.GPUCompressionASTC4x4},
		{"ASTC_4x4_sRGB", 0, 0x93D0, ir.GPUCompressionASTC4x4},
		{"unknown_compressed", 0, 0xFFFF, ir.GPUCompressionASTC4x4},
		{"uncompressed_rgba", 0x1908, 0x8058, ir.GPUCompressionNone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := buildKTX1(tc.glFmt, tc.glIntFmt)
			dec := &ktx.Decoder{}
			scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
			require.NoError(t, err)
			assert.Equal(t, tc.want, scene.Images[0].CompressionFormat)
		})
	}
}
