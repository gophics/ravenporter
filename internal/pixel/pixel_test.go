package pixel

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToRGBA(t *testing.T) {
	nrgba := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	nrgba.SetNRGBA(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 4})
	assert.Equal(t, []byte{1, 2, 3, 4}, ToRGBA(nrgba, 1, 1))

	rgba := image.NewRGBA(image.Rect(0, 0, 1, 1))
	rgba.SetRGBA(0, 0, color.RGBA{R: 5, G: 6, B: 7, A: 8})
	assert.Equal(t, []byte{5, 6, 7, 8}, ToRGBA(rgba, 1, 1))

	ycbcr := image.NewYCbCr(image.Rect(0, 0, 1, 1), image.YCbCrSubsampleRatio444)
	ycbcr.Y[0] = 128
	ycbcr.Cb[0] = 128
	ycbcr.Cr[0] = 128
	assert.Len(t, ToRGBA(ycbcr, 1, 1), 4)

	gray := image.NewGray(image.Rect(0, 0, 1, 1))
	gray.SetGray(0, 0, color.Gray{Y: 9})
	assert.Equal(t, []byte{9, 9, 9, 255}, ToRGBA(gray, 1, 1))
}

func TestRGBEToFloat(t *testing.T) {
	r, g, b := RGBEToFloat(0, 0, 0, 0)
	assert.Zero(t, r)
	assert.Zero(t, g)
	assert.Zero(t, b)

	r, g, b = RGBEToFloat(128, 64, 32, 129)
	assert.Greater(t, r, float32(0))
	assert.Greater(t, g, float32(0))
	assert.Greater(t, b, float32(0))
}

func TestFloat16to32(t *testing.T) {
	assert.Equal(t, float32(0), Float16to32(0))
	assert.Equal(t, float32(1), Float16to32(0x3C00))
	assert.True(t, Float16to32(0x7C00) > 1e10)
	assert.Greater(t, Float16to32(0x0001), float32(0))
}
