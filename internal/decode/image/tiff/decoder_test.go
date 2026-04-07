package tiff_test

import (
	"bytes"
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

func init() {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{128, 128, 128, 255})

	err := tiff.Encode(&buf, img, nil)
	if err != nil {
		panic("unable to build valid mock tiff")
	}
	tiffData = buf.Bytes()
}

func TestTIFFProbe(t *testing.T) {
	dec := &imgtiff.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(tiffData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestTIFFDecode(t *testing.T) {
	dec := &imgtiff.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(tiffData), opts)
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	imgScene := scene.Images[0]
	assert.Equal(t, ir.ImageTIFF, imgScene.Format)
	assert.Equal(t, 1, imgScene.Width)
	assert.Equal(t, 1, imgScene.Height)

	assert.Equal(t, ir.ChannelRGBA, imgScene.Channels)
	assert.Equal(t, ir.ColorSRGB, imgScene.ColorSpace)

	pb, decErr := imgScene.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 1*1*4)
}

func BenchmarkDecode(b *testing.B) {
	dec := &imgtiff.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(tiffData), opts)
	}
}
