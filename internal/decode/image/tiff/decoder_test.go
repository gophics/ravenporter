package tiff_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	imgtiff "github.com/gophics/ravenporter/internal/decode/image/tiff"
	"github.com/gophics/ravenporter/ir"
	"golang.org/x/image/tiff"
)

var tiffData []byte
var bigTIFFData []byte

func init() {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{128, 128, 128, 255})

	err := tiff.Encode(&buf, img, nil)
	if err != nil {
		panic("unable to build valid mock tiff")
	}
	tiffData = buf.Bytes()
	bigTIFFData = buildBigTIFF()
}

func TestTIFFProbe(t *testing.T) {
	dec := &imgtiff.Decoder{}
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "classic", data: tiffData, want: true},
		{name: "big", data: bigTIFFData, want: true},
		{name: "invalid", data: []byte("no"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dec.Probe(bytes.NewReader(tt.data)))
		})
	}
}

func TestTIFFDecode(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		channels ir.ChannelCount
		pixels   []byte
	}{
		{
			name:     "classic",
			data:     tiffData,
			channels: ir.ChannelRGBA,
			pixels:   []byte{128, 128, 128, 255},
		},
		{
			name:     "big",
			data:     bigTIFFData,
			channels: ir.ChannelGray,
			pixels:   []byte{128, 128, 128, 255},
		},
	}

	dec := &imgtiff.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			require.NoError(t, err)
			require.Len(t, scene.Images, 1)

			imgScene := scene.Images[0]
			assert.Equal(t, ir.ImageTIFF, imgScene.Format)
			assert.Equal(t, 1, imgScene.Width)
			assert.Equal(t, 1, imgScene.Height)
			assert.Equal(t, tt.channels, imgScene.Channels)
			assert.Equal(t, ir.ColorSRGB, imgScene.ColorSpace)

			pb, decErr := imgScene.DecodePixels()
			require.NoError(t, decErr)
			require.NotNil(t, pb)
			assert.Equal(t, tt.pixels, pb.Data)
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	dec := &imgtiff.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(tiffData), opts)
	}
}

func BenchmarkDecodeBigTIFF(b *testing.B) {
	dec := &imgtiff.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(bigTIFFData), opts)
	}
}

func buildBigTIFF() []byte {
	const (
		headerSize    = 16
		entryCount    = 8
		entrySize     = 20
		ifdCountSize  = 8
		nextIFDSize   = 8
		firstIFD      = 16
		pixelByteSize = 1
	)

	entryTableEnd := firstIFD + ifdCountSize + entryCount*entrySize + nextIFDSize
	pixelOffset := uint64(entryTableEnd)
	buf := make([]byte, entryTableEnd+pixelByteSize)

	copy(buf[:4], []byte{0x49, 0x49, 0x2B, 0x00})
	binary.LittleEndian.PutUint16(buf[4:], 8)
	binary.LittleEndian.PutUint64(buf[8:], firstIFD)
	binary.LittleEndian.PutUint64(buf[firstIFD:], entryCount)

	entries := []struct {
		tag   uint16
		typ   uint16
		count uint64
		value uint64
	}{
		{tag: 256, typ: 4, count: 1, value: 1},
		{tag: 257, typ: 4, count: 1, value: 1},
		{tag: 258, typ: 3, count: 1, value: 8},
		{tag: 259, typ: 3, count: 1, value: 1},
		{tag: 262, typ: 3, count: 1, value: 1},
		{tag: 273, typ: 16, count: 1, value: pixelOffset},
		{tag: 278, typ: 4, count: 1, value: 1},
		{tag: 279, typ: 16, count: 1, value: 1},
	}

	off := firstIFD + ifdCountSize
	for _, entry := range entries {
		binary.LittleEndian.PutUint16(buf[off:], entry.tag)
		binary.LittleEndian.PutUint16(buf[off+2:], entry.typ)
		binary.LittleEndian.PutUint64(buf[off+4:], entry.count)
		binary.LittleEndian.PutUint64(buf[off+12:], entry.value)
		off += entrySize
	}

	buf[int(pixelOffset)] = 128
	return buf
}
