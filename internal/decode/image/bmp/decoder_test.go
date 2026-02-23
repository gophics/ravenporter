package bmp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/bmp"
	"github.com/gophics/ravenporter/ir"
)

var (
	minimalBMP = []byte{
		0x42, 0x4D, 0x46, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x36, 0x00, 0x00, 0x00, 0x28, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x18, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x13, 0x0B,
		0x00, 0x00, 0x13, 0x0B, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xFF, 0x00, 0x00, 0xFF, 0x00, 0x00, 0xFF, 0x00,
		0x00, 0xFF, 0x00, 0x00, 0x00, 0x00,
	}

	minimalRLE8 = []byte{
		0x42, 0x4D, 0x46, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x3E, 0x00, 0x00, 0x00, 0x28, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x08, 0x00, 0x01, 0x00,
		0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x13, 0x0B,
		0x00, 0x00, 0x13, 0x0B, 0x00, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xFF, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x02, 0x01,
		0x00, 0x01, 0x02, 0x00, 0x00, 0x01,
	}

	minimalRLE4 = []byte{
		0x42, 0x4D, 0x46, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x3E, 0x00, 0x00, 0x00, 0x28, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x04, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x13, 0x0B,
		0x00, 0x00, 0x13, 0x0B, 0x00, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xFF, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x02, 0x11,
		0x00, 0x01, 0x02, 0x00, 0x00, 0x01,
	}
)

func TestDecoder_Probe(t *testing.T) {
	dec := &bmp.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(minimalBMP)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("not bmp"))))
}

func TestDecoder_Decode(t *testing.T) {
	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(minimalBMP), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, scene.Images, 1)
	img := scene.Images[0]
	assert.Equal(t, ir.ImageBMP, img.Format)
	assert.Equal(t, 2, img.Width)
	assert.Equal(t, 2, img.Height)
	assert.Equal(t, ir.ChannelRGBA, img.Channels)

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 16)
}

func TestDecoder_DecodeRLE8(t *testing.T) {
	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(minimalRLE8), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, scene.Images, 1)
	img := scene.Images[0]
	assert.Equal(t, ir.ImageBMP, img.Format)
	assert.Equal(t, 2, img.Width)
	assert.Equal(t, 2, img.Height)

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 16)
}

func TestDecoder_DecodeRLE4(t *testing.T) {
	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(minimalRLE4), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, scene.Images, 1)
	img := scene.Images[0]
	assert.Equal(t, ir.ImageBMP, img.Format)
	assert.Equal(t, 2, img.Width)
	assert.Equal(t, 2, img.Height)

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 16)
}

func TestDecoder_Extensions(t *testing.T) {
	dec := &bmp.Decoder{}
	assert.Equal(t, []string{".bmp"}, dec.Extensions())
}

func TestDecoder_Name(t *testing.T) {
	dec := &bmp.Decoder{}
	assert.Equal(t, "BMP", dec.FormatName())
}

func BenchmarkDecode(b *testing.B) {
	dec := &bmp.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(minimalBMP), opts)
	}
}

func TestDecoder_Decode32bit(t *testing.T) {
	// Build a 2x2 32-bit BMP
	data := []byte{
		0x42, 0x4D, 0x46, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x36, 0x00, 0x00, 0x00, 0x28, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x20, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x13, 0x0B,
		0x00, 0x00, 0x13, 0x0B, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// Row 0: 2 pixels * 4 bytes = 8 bytes
		0xFF, 0x00, 0x00, 0x80, 0x00, 0xFF, 0x00, 0x80,
		// Row 1:
		0x00, 0x00, 0xFF, 0x80, 0xFF, 0xFF, 0x00, 0x80,
	}
	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	img := scene.Images[0]
	assert.Equal(t, 2, img.Width)
	assert.Equal(t, 2, img.Height)
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	// 32-bit BMP should preserve alpha
	assert.Equal(t, byte(0x80), pb.Data[3]) // alpha of first pixel
}

func TestDecoder_DecodeTopDown(t *testing.T) {
	// Build a 2x1 top-down BMP (negative height)
	data := []byte{
		0x42, 0x4D, 0x3E, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x36, 0x00, 0x00, 0x00, 0x28, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0xFF, 0xFF,
		0xFF, 0xFF, 0x01, 0x00, 0x18, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x13, 0x0B,
		0x00, 0x00, 0x13, 0x0B, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// 1 row: 2 pixels * 3 bytes = 6 + 2 pad = 8
		0xFF, 0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00,
	}
	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, scene.Images[0].Width)
	assert.Equal(t, 1, scene.Images[0].Height)
}

func TestDecoder_DecodeWithoutPixels(t *testing.T) {
	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(minimalBMP), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	assert.Nil(t, scene.Images[0].Pixels())
}

func TestDecoder_DecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("BM")},
		{"bad magic", append([]byte("XX"), make([]byte, 60)...)},
	}
	dec := &bmp.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			assert.Error(t, err)
		})
	}
}

func buildBMPHeader(width, height int32, bpp uint16, compression uint32, colors int, pixelData []byte) []byte {
	dibSize := uint32(40)
	palSize := colors * 4
	pixelOff := 14 + int(dibSize) + palSize
	fileSize := pixelOff + len(pixelData)

	var buf bytes.Buffer
	buf.Write([]byte("BM"))
	writeU32LE(&buf, uint32(fileSize))
	writeU32LE(&buf, 0) // reserved
	writeU32LE(&buf, uint32(pixelOff))

	writeU32LE(&buf, dibSize)
	writeU32LE(&buf, uint32(width))
	writeU32LE(&buf, uint32(height))
	writeU16LE(&buf, 1) // planes
	writeU16LE(&buf, bpp)
	writeU32LE(&buf, compression)
	writeU32LE(&buf, uint32(len(pixelData)))
	writeU32LE(&buf, 2835)
	writeU32LE(&buf, 2835)
	writeU32LE(&buf, uint32(colors))
	writeU32LE(&buf, 0) // important colors

	return buf.Bytes()
}

func writeU32LE(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 24))
}

func writeU16LE(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
}

func TestDecoder_Palette8(t *testing.T) {
	palette := make([]byte, 256*4)
	palette[0], palette[1], palette[2], palette[3] = 0xFF, 0, 0, 0
	palette[4], palette[5], palette[6], palette[7] = 0, 0xFF, 0, 0

	rowSize := 4 // width 2, bpp 8 → 2 bytes + 2 pad
	pixelData := make([]byte, rowSize*2)
	pixelData[0] = 0
	pixelData[1] = 1
	pixelData[rowSize] = 1
	pixelData[rowSize+1] = 0

	hdr := buildBMPHeader(2, 2, 8, 0, 256, pixelData)
	data := make([]byte, 0, len(hdr)+len(palette)+len(pixelData))
	data = append(data, hdr...)
	data = append(data, palette...)
	data = append(data, pixelData...)

	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	img := scene.Images[0]
	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 16)
}

func TestDecoder_Palette4(t *testing.T) {
	palette := make([]byte, 16*4)
	palette[0], palette[1], palette[2], palette[3] = 0xFF, 0, 0, 0
	palette[4], palette[5], palette[6], palette[7] = 0, 0xFF, 0, 0

	rowSize := 4 // width 2, bpp 4 → 1 byte + 3 pad
	pixelData := make([]byte, rowSize*2)
	pixelData[0] = 0x01
	pixelData[rowSize] = 0x10

	hdr := buildBMPHeader(2, 2, 4, 0, 16, pixelData)
	data := make([]byte, 0, len(hdr)+len(palette)+len(pixelData))
	data = append(data, hdr...)
	data = append(data, palette...)
	data = append(data, pixelData...)

	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDecoder_RLE8Absolute(t *testing.T) {
	palette := make([]byte, 8)
	palette[0], palette[1], palette[2], palette[3] = 0xFF, 0, 0, 0
	palette[4], palette[5], palette[6], palette[7] = 0, 0xFF, 0, 0

	var rle bytes.Buffer
	rle.Write([]byte{0x00, 0x02, 0x00, 0x01}) // absolute mode: 2 pixels → 2 bytes padded to 2
	rle.Write([]byte{0x00, 0x01})             // EOL
	rle.Write([]byte{0x02, 0x01})             // encoded: 2 pixels of index 1
	rle.Write([]byte{0x00, 0x00})             // EOF
	rleData := rle.Bytes()

	hdr := buildBMPHeader(2, 2, 8, 1, 2, rleData)
	data := make([]byte, 0, len(hdr)+len(palette)+len(rleData))
	data = append(data, hdr...)
	data = append(data, palette...)
	data = append(data, rleData...)

	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDecoder_RLE8Delta(t *testing.T) {
	palette := make([]byte, 8)
	palette[0], palette[1], palette[2], palette[3] = 0xFF, 0, 0, 0
	palette[4], palette[5], palette[6], palette[7] = 0, 0xFF, 0, 0

	var rle bytes.Buffer
	rle.Write([]byte{0x01, 0x00})             // encoded: 1 pixel of index 0
	rle.Write([]byte{0x00, 0x02, 0x01, 0x00}) // delta: move x+1, y+0
	rle.Write([]byte{0x01, 0x01})             // encoded: 1 pixel of index 1
	rle.Write([]byte{0x00, 0x00})             // EOF
	rleData := rle.Bytes()

	hdr := buildBMPHeader(4, 1, 8, 1, 2, rleData)
	data := make([]byte, 0, len(hdr)+len(palette)+len(rleData))
	data = append(data, hdr...)
	data = append(data, palette...)
	data = append(data, rleData...)

	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDecoder_TopDownRLE8(t *testing.T) {
	palette := make([]byte, 8)
	palette[0], palette[1], palette[2], palette[3] = 0xFF, 0, 0, 0
	palette[4], palette[5], palette[6], palette[7] = 0, 0xFF, 0, 0

	var rle bytes.Buffer
	rle.Write([]byte{0x02, 0x00}) // encoded: 2 pixels of index 0
	rle.Write([]byte{0x00, 0x01}) // EOL
	rle.Write([]byte{0x02, 0x01}) // encoded: 2 pixels of index 1
	rle.Write([]byte{0x00, 0x00}) // EOF
	rleData := rle.Bytes()

	// negative height for top-down
	hdr := buildBMPHeader(2, -2, 8, 1, 2, rleData)
	data := make([]byte, 0, len(hdr)+len(palette)+len(rleData))
	data = append(data, hdr...)
	data = append(data, palette...)
	data = append(data, rleData...)

	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDecoder_RLE4Absolute(t *testing.T) {
	palette := make([]byte, 8)
	palette[0], palette[1], palette[2], palette[3] = 0xFF, 0, 0, 0
	palette[4], palette[5], palette[6], palette[7] = 0, 0xFF, 0, 0

	var rle bytes.Buffer
	rle.Write([]byte{0x00, 0x04, 0x01, 0x10, 0x00, 0x00}) // absolute: 4 nibbles from 0x01,0x10, padded to 4 bytes → already even
	rle.Write([]byte{0x00, 0x00})                         // EOF
	rleData := rle.Bytes()

	hdr := buildBMPHeader(4, 1, 4, 2, 2, rleData)
	data := make([]byte, 0, len(hdr)+len(palette)+len(rleData))
	data = append(data, hdr...)
	data = append(data, palette...)
	data = append(data, rleData...)

	dec := &bmp.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
}

func TestDecoder_MaxImagePixelsRejection(t *testing.T) {
	dec := &bmp.Decoder{}
	opts := detect.DecodeOptions{MaxImagePixels: 1}
	_, err := dec.Decode(bytes.NewReader(minimalBMP), opts)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bmp")
	inner := errors.Unwrap(err)
	require.NotNil(t, inner)
	assert.Contains(t, inner.Error(), "exceed pixel limit")
}
