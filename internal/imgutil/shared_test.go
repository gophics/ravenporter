package imgutil

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestBuildAssetAndCheckPixelLimit(t *testing.T) {
	img := &ir.ImageAsset{Name: "Sprite"}
	asset := BuildAsset(img, ir.FormatPNG)
	require.Len(t, asset.Images, 1)
	assert.Equal(t, ir.FormatPNG, asset.Images[0].SourceFormat)
	assert.Equal(t, ir.FormatPNG, asset.Metadata.SourceFormat)

	assert.NoError(t, CheckPixelLimit(2, 2, 4))
	assert.Error(t, CheckPixelLimit(3, 2, 4))
}

func TestChannelCount(t *testing.T) {
	assert.Equal(t, ir.ChannelGray, ChannelCount(image.NewGray(image.Rect(0, 0, 1, 1))))
	assert.Equal(t, ir.ChannelRGBA, ChannelCount(image.NewRGBA(image.Rect(0, 0, 1, 1))))
}

func TestDecodeStdlibImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	src.SetRGBA(1, 0, color.RGBA{G: 255, A: 255})

	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, src))

	asset, err := DecodeStdlibImage(
		bytes.NewReader(encoded.Bytes()),
		detect.DecodeOptions{},
		"atlas",
		ir.ImagePNG,
		ir.FormatPNG,
	)
	require.NoError(t, err)
	require.Len(t, asset.Images, 1)

	decoded := asset.Images[0]
	assert.Equal(t, 2, decoded.Width)
	assert.Equal(t, 1, decoded.Height)
	assert.Equal(t, ir.ChannelRGBA, decoded.Channels)

	pixels, err := decoded.DecodePixels()
	require.NoError(t, err)
	require.NotNil(t, pixels)
	assert.Len(t, pixels.Data, 8)
}

func TestDecodeStdlibImageErrors(t *testing.T) {
	_, err := DecodeStdlibImage(bytes.NewReader([]byte("bad")), detect.DecodeOptions{}, "bad", ir.ImagePNG, ir.FormatPNG)
	assert.Error(t, err)

	_, err = ReadAllBytes(bytes.NewReader([]byte("abcd")), 2)
	assert.Error(t, err)
}

func TestProbeBytes(t *testing.T) {
	assert.True(t, ProbeBytes(bytes.NewReader([]byte("\x89PNG\r\n\x1a\nbody")), []byte("\x89PNG\r\n\x1a\n")))
}

func TestImageFormatFromPath(t *testing.T) {
	assert.Equal(t, ir.ImagePNG, ImageFormatFromPath("albedo.PNG"))
	assert.Equal(t, ir.ImageJPEG, ImageFormatFromPath("albedo.JpEg"))
	assert.Equal(t, ir.ImageWebP, ImageFormatFromPath("albedo.webp"))
	assert.Equal(t, ir.ImageKTX, ImageFormatFromPath("albedo.KTX2"))
	assert.Equal(t, ir.ImageDDS, ImageFormatFromPath("albedo.dds"))
	assert.Equal(t, ir.ImageBMP, ImageFormatFromPath("albedo.bmp"))
	assert.Equal(t, ir.ImageTGA, ImageFormatFromPath("albedo.tga"))
	assert.Equal(t, ir.ImageHDR, ImageFormatFromPath("albedo.hdr"))
	assert.Equal(t, ir.ImagePSD, ImageFormatFromPath("albedo.psd"))
	assert.Equal(t, ir.ImageTIFF, ImageFormatFromPath("albedo.TIF"))
	assert.Equal(t, ir.ImageEXR, ImageFormatFromPath("albedo.exr"))
	assert.Empty(t, ImageFormatFromPath("albedo.bin"))
}
