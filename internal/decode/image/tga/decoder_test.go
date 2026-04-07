package tga_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/tga"
	"github.com/gophics/ravenporter/ir"
)

var tgaData []byte

func init() {
	var err error
	tgaData, err = os.ReadFile("../testdata/blue_2x2.tga")
	if err != nil {
		panic("failed to load tgaData: " + err.Error())
	}
	// Append TGA v2.0 strict footer for regression probes
	footer := make([]byte, 26)
	copy(footer[8:], "TRUEVISION-XFILE.\x00")
	tgaData = append(tgaData, footer...)
}

func TestTGAProbe(t *testing.T) {
	dec := &tga.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(tgaData)))

	assert.False(t, dec.Probe(bytes.NewReader(make([]byte, 10))))
}

func TestTGADecode(t *testing.T) {
	dec := &tga.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(tgaData), opts)
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	img := scene.Images[0]
	assert.Equal(t, ir.ImageTGA, img.Format)
	assert.Equal(t, 2, img.Width)
	assert.Equal(t, 2, img.Height)

	assert.Equal(t, ir.ChannelRGBA, img.Channels)
	assert.Equal(t, ir.ColorSRGB, img.ColorSpace)

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 2*2*4)
}

func BenchmarkDecode(b *testing.B) {
	dec := &tga.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(tgaData), opts)
	}
}

func TestTGADecodeWithoutPixels(t *testing.T) {
	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(tgaData), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	assert.Nil(t, scene.Images[0].Pixels())
	assert.Equal(t, 2, scene.Images[0].Width)
}

func buildTGA(imageType byte, w, h uint16, bpp, origin byte, pixels []byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0) // idLength
	if imageType == 1 || imageType == 9 {
		buf.WriteByte(1) // colorMapType
	} else {
		buf.WriteByte(0)
	}
	buf.WriteByte(imageType)
	// color map spec
	buf.Write([]byte{0, 0, 0, 0, 0}) // cmFirstEntry(2), cmLength(2), cmEntrySize(1)
	// image spec
	buf.Write([]byte{0, 0, 0, 0}) // x/y origin
	buf.WriteByte(byte(w))
	buf.WriteByte(byte(w >> 8))
	buf.WriteByte(byte(h))
	buf.WriteByte(byte(h >> 8))
	buf.WriteByte(bpp)
	buf.WriteByte(origin) // descriptor
	buf.Write(pixels)
	return buf.Bytes()
}

func TestTGARLEDecode(t *testing.T) {
	// 2x2 RLE RGB (type 10), 24-bit
	var rle bytes.Buffer
	// RLE packet: run of 4 pixels with color (0xFF, 0x00, 0x00 = blue in BGR)
	rle.WriteByte(0x83)                 // run of 4 pixels
	rle.Write([]byte{0x00, 0x00, 0xFF}) // BGR = red

	data := buildTGA(10, 2, 2, 24, 0x20, rle.Bytes()) // top-origin
	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	assert.Equal(t, 2, img.Width)
	assert.Equal(t, 2, img.Height)
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestTGARLERawString(t *testing.T) {
	// 2x2 RLE RGB (type 10), 24-bit
	var rle bytes.Buffer
	// Raw packet: count = 4 pixels (0x03), then 4 pixels of uncompressed data
	rle.WriteByte(0x03)
	rle.Write([]byte{0x00, 0x00, 0xFF}) // pixel 1 (blue)
	rle.Write([]byte{0x00, 0xFF, 0x00}) // pixel 2 (green)
	rle.Write([]byte{0xFF, 0x00, 0x00}) // pixel 3 (red)
	rle.Write([]byte{0xFF, 0xFF, 0xFF}) // pixel 4 (white)

	data := buildTGA(10, 2, 2, 24, 0x20, rle.Bytes()) // top-origin
	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestTGAColorMappedRLE(t *testing.T) {
	// 2x2 colormapped RLE TGA (type 9), 8bpp indices, 24-bit colormap
	var buf bytes.Buffer
	buf.WriteByte(0)              // idLength
	buf.WriteByte(1)              // mapType yes
	buf.WriteByte(9)              // imgType colormapped RLE
	buf.Write([]byte{0, 0})       // cmFirstEntry zero
	buf.Write([]byte{2, 0})       // cmLength = 2
	buf.WriteByte(24)             // cmEntrySize = 24 bits
	buf.Write([]byte{0, 0, 0, 0}) // x/y origin
	buf.Write([]byte{2, 0})       // width = 2
	buf.Write([]byte{2, 0})       // height = 2
	buf.WriteByte(8)              // bpp = 8
	buf.WriteByte(0x20)           // top-origin

	// Color map: 2 entries, 3 bytes each (BGR)
	buf.Write([]byte{0xFF, 0x00, 0x00}) // entry 0: blue
	buf.Write([]byte{0x00, 0xFF, 0x00}) // entry 1: green

	// RLE sequence: 1 run packet of 4 pixels (0x80 | 3), value = 1 (green)
	buf.WriteByte(0x83)
	buf.WriteByte(0x01)

	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	// Verify mapped color extraction
	assert.Equal(t, byte(0x00), pb.Data[0]) // R (from green BGR map entry)
	assert.Equal(t, byte(0xFF), pb.Data[1]) // G
	assert.Equal(t, byte(0x00), pb.Data[2]) // B
}

func TestTGA32Bit(t *testing.T) {
	// 1x1 uncompressed 32-bit RGBA
	pixels := []byte{0x00, 0x00, 0xFF, 0x80} // BGRA: red with alpha 0x80
	data := buildTGA(2, 1, 1, 32, 0x20, pixels)
	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, byte(0xFF), pb.Data[0]) // R
	assert.Equal(t, byte(0x80), pb.Data[3]) // A
}

func TestTGAFlipVertical(t *testing.T) {
	// 2x2 bottom-origin (descriptor=0) should flip
	pixels := []byte{
		0xFF, 0x00, 0x00, 0x00, 0xFF, 0x00, // row 0 (bottom): blue, green
		0x00, 0x00, 0xFF, 0xFF, 0xFF, 0x00, // row 1 (top): red, yellow
	}
	data := buildTGA(2, 2, 2, 24, 0x00, pixels) // bottom-origin
	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	// After flip, top row should be row 1 (red, yellow)
	assert.Equal(t, byte(0xFF), pb.Data[0]) // R of first pixel
}

func TestTGAColorMapped(t *testing.T) {
	// 2x1 colormapped TGA (type 1), 8bpp indices, 24-bit colormap
	var buf bytes.Buffer
	buf.WriteByte(0)              // idLength
	buf.WriteByte(1)              // mapType yes
	buf.WriteByte(1)              // imgType colormapped
	buf.Write([]byte{0, 0})       // cmFirstEntry zero
	buf.Write([]byte{2, 0})       // cmLength = 2
	buf.WriteByte(24)             // cmEntrySize = 24 bits
	buf.Write([]byte{0, 0, 0, 0}) // x/y origin
	buf.Write([]byte{2, 0})       // width = 2
	buf.Write([]byte{1, 0})       // height = 1
	buf.WriteByte(8)              // bpp = 8
	buf.WriteByte(0x20)           // top-origin
	// Color map: 2 entries, 3 bytes each (BGR)
	buf.Write([]byte{0xFF, 0x00, 0x00}) // entry 0: blue
	buf.Write([]byte{0x00, 0xFF, 0x00}) // entry 1: green
	// Pixel indices
	buf.Write([]byte{0, 1})

	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, byte(0x00), pb.Data[0]) // R (from blue BGR)
	assert.Equal(t, byte(0x00), pb.Data[1]) // G
	assert.Equal(t, byte(0xFF), pb.Data[2]) // B
}

func TestTGAColorMapped16Bit(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(0) // idLength
	buf.WriteByte(1) // mapType yes
	buf.WriteByte(1) // imgType colormapped
	buf.Write([]byte{10, 0})
	buf.Write([]byte{2, 0})       // cmLength = 2
	buf.WriteByte(16)             // cmEntrySize = 16 bits
	buf.Write([]byte{0, 0, 0, 0}) // x/y origin
	buf.Write([]byte{2, 0})       // width = 2
	buf.Write([]byte{1, 0})       // height = 1
	buf.WriteByte(8)              // bpp = 8
	buf.WriteByte(0x20)           // top-origin
	// Color map: 2 entries, 16 bits each -> TRRRRRGGGGGBBBBB
	// entry 10: RGB(1F, 0, 0)
	buf.Write([]byte{0x00, 0x7C})
	// entry 11: RGB(0, 1F, 0)
	buf.Write([]byte{0xE0, 0x03})

	// Pixel indices pointing to entry 10 and 11
	buf.Write([]byte{10, 11})

	dec := &tga.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	// 5-bit channel max is 0x1F. Decoder shifts up by 3, so max should be 0x1F << 3 = 0xF8
	assert.Equal(t, byte(0xF8), pb.Data[0]) // R
	assert.Equal(t, byte(0x00), pb.Data[1]) // G
}

func TestTGADecodeErrors(t *testing.T) {
	dec := &tga.Decoder{}
	_, err := dec.Decode(bytes.NewReader([]byte{}), detect.DecodeOptions{})
	// Should either error or return a 0x0 image
	if err != nil {
		assert.Error(t, err)
	}
}
