// Package pixel provides image-to-RGBA conversion utilities for image decoders.
package pixel

import (
	"image"
	"math"
)

const (
	rgbaStride = 4

	// colorShift converts 16-bit RGBA values to 8-bit by right-shifting.
	colorShift = 8

	// RGBE constants for Radiance HDR decoding.
	rgbeExpBias = 128
	rgbeHalfPx  = 0.5
)

// ToRGBA converts any image.Image to raw RGBA8 bytes.
// Fast path for *image.NRGBA and *image.RGBA via direct row copy.
func ToRGBA(img image.Image, w, h int) []byte {
	pixels := make([]byte, w*h*rgbaStride)

	switch src := img.(type) {
	case *image.NRGBA:
		copyDirect(pixels, src.Pix, src.Stride, w, h)
		return pixels
	case *image.RGBA:
		copyDirect(pixels, src.Pix, src.Stride, w, h)
		return pixels
	case *image.YCbCr:
		bounds := src.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, a := src.YCbCrAt(x, y).RGBA()
				off := ((y-bounds.Min.Y)*w + (x - bounds.Min.X)) * rgbaStride
				pixels[off] = uint8(r >> colorShift)   //nolint:gosec // 16->8 bit
				pixels[off+1] = uint8(g >> colorShift) //nolint:gosec // 16->8 bit
				pixels[off+2] = uint8(b >> colorShift) //nolint:gosec // 16->8 bit
				pixels[off+3] = uint8(a >> colorShift) //nolint:gosec // 16->8 bit
			}
		}
		return pixels
	}

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			off := ((y-bounds.Min.Y)*w + (x - bounds.Min.X)) * rgbaStride
			pixels[off] = uint8(r >> colorShift)   //nolint:gosec // 16â†’8 bit
			pixels[off+1] = uint8(g >> colorShift) //nolint:gosec // 16â†’8 bit
			pixels[off+2] = uint8(b >> colorShift) //nolint:gosec // 16â†’8 bit
			pixels[off+3] = uint8(a >> colorShift) //nolint:gosec // 16â†’8 bit
		}
	}
	return pixels
}

// RGBEToFloat converts a Radiance RGBE pixel (r, g, b, exponent) to
// three float32 linear values. Reusable across HDR and EXR decoders.
func RGBEToFloat(r, g, b, e byte) (fr, fg, fb float32) {
	if e == 0 {
		return 0, 0, 0
	}
	exp := float32(math.Ldexp(1.0, int(e)-rgbeExpBias-colorShift))
	return (float32(r) + rgbeHalfPx) * exp,
		(float32(g) + rgbeHalfPx) * exp,
		(float32(b) + rgbeHalfPx) * exp
}

func copyDirect(dst, src []byte, srcStride, w, h int) {
	rowBytes := w * rgbaStride
	for y := range h {
		dstOff := y * rowBytes
		srcOff := y * srcStride
		copy(dst[dstOff:dstOff+rowBytes], src[srcOff:srcOff+rowBytes])
	}
}

const (
	f16SignShift     = 15
	f16ExpMask       = 0x1F
	f16ExpShift      = 10
	f16MantissaMask  = 0x3FF
	f32SignShift     = 31
	f32ExpShift      = 23
	f32ExpBias       = 127
	f16ExpBias       = 15
	f32MantissaShift = 13 // 23 - 10 = shift mantissa from half to single
)

// Float16to32 converts an IEEE 754 half-precision float to float32.
func Float16to32(h uint16) float32 {
	sign := uint32(h>>f16SignShift) & 1
	exp := uint32(h>>f16ExpShift) & f16ExpMask
	mant := uint32(h) & f16MantissaMask

	var f uint32
	switch {
	case exp == 0 && mant == 0:
		f = sign << f32SignShift
	case exp == 0:
		exp = 1
		for mant&(f16MantissaMask+1) == 0 {
			mant <<= 1
			exp--
		}
		mant &= f16MantissaMask
		f = sign<<f32SignShift | (exp+f32ExpBias-f16ExpBias)<<f32ExpShift | mant<<f32MantissaShift
	case exp == f16ExpMask:
		f = sign<<f32SignShift | 0xFF<<f32ExpShift | mant<<f32MantissaShift
	default:
		f = sign<<f32SignShift | (exp+f32ExpBias-f16ExpBias)<<f32ExpShift | mant<<f32MantissaShift
	}
	return math.Float32frombits(f)
}
