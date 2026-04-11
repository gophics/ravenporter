// Package imgutil provides shared utilities for isolated image decoders.
package imgutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pixel"
	"github.com/gophics/ravenporter/ir"
)

// Shared animation metadata keys used by animated image decoders.
const (
	MetaKeyDelayNum = "DelayNum"
	MetaKeyDelayDen = "DelayDen"
)

// DecodeErrStr wraps a decode failure into a formatted error using a string format name.
func DecodeErrStr(name string, cause error) error {
	return decutil.DecodeErr(ir.FormatID(name), "failed to decode image", cause)
}

// BuildAsset wraps an ImageAsset into an Asset.
func BuildAsset(img *ir.ImageAsset, format ir.FormatID) *ir.Asset {
	img.NormalizeTopology()
	img.SourceFormat = format
	asset := ir.NewAsset(format)
	asset.Images = []*ir.ImageAsset{img}
	return asset
}

func CheckPixelLimit(w, h, maxPixels int) error {
	if maxPixels <= 0 {
		return nil
	}
	if w > 0 && h > 0 && w*h > maxPixels {
		return fmt.Errorf("image dimensions %dx%d exceed pixel limit %d", w, h, maxPixels)
	}
	return nil
}

// DecodeStdlibImage is the shared decode path for formats using Go's standard image.Decode
// (e.g., WebP, TIFF). Reads raw bytes, decodes dimensions, and extracts pixels.
func DecodeStdlibImage(
	r detect.ReadSeekerAt,
	opts detect.DecodeOptions,
	name string,
	imgFmt ir.ImageFormat,
	fmtID ir.FormatID,
) (*ir.Asset, error) {
	raw, err := ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, DecodeErrStr(name, err)
	}

	decoded := &ir.ImageAsset{
		Name:       name,
		Format:     imgFmt,
		Channels:   ir.ChannelRGBA,
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
	}

	img, _, decErr := image.Decode(bytes.NewReader(raw))
	if decErr != nil {
		return nil, DecodeErrStr(name, decErr)
	}
	bounds := img.Bounds()
	decoded.Width = bounds.Dx()
	decoded.Height = bounds.Dy()
	decoded.Channels = ChannelCount(img)

	if err := CheckPixelLimit(decoded.Width, decoded.Height, opts.MaxImagePixels); err != nil {
		return nil, DecodeErrStr(name, err)
	}

	decoded.PixelDecode = func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
		srcImg, _, err := image.Decode(bytes.NewReader(d.Compressed))
		if err != nil {
			return nil, err
		}
		return &ir.PixelBuffer{
			Data:     pixel.ToRGBA(srcImg, d.Width, d.Height),
			DataType: ir.DataTypeUint8,
			BitDepth: ir.BitDepth8,
		}, nil
	}

	return BuildAsset(decoded, fmtID), nil
}

// ChannelCount infers the channel count based on Go's internal ColorModel.
func ChannelCount(img image.Image) ir.ChannelCount {
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		return ir.ChannelGray
	default:
		return ir.ChannelRGBA
	}
}

// ProbeBytes safely checks a magic header against an I/O stream.
func ProbeBytes(r io.ReadSeeker, magic []byte) bool {
	return decutil.ProbeBytes(r, magic)
}

// ReadAllBytes checks file size against maxSize, then reads the full blob.
func ReadAllBytes(r detect.ReadSeekerAt, maxSize int64) ([]byte, error) {
	if err := decutil.CheckStreamSize(r, maxSize); err != nil {
		return nil, err
	}
	return decutil.ReadAll(r)
}
