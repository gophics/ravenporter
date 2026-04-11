package tiff

import (
	"bytes"
	"image"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pixel"
	"github.com/gophics/ravenporter/ir"
	_ "golang.org/x/image/tiff" // register TIFF codec
)

const (
	tiffFormatName = "TIFF"
	tiffName       = "tiff"
	extTIFF        = ".tiff"
	extTIF         = ".tif"
	tiffMagicLen   = 4
)

var magicTIFFLE = []byte{0x49, 0x49, 0x2A, 0x00}
var magicTIFFBE = []byte{0x4D, 0x4D, 0x00, 0x2A}
var magicBigTIFFLE = []byte{0x49, 0x49, 0x2B, 0x00, 0x08, 0x00, 0x00, 0x00}
var magicBigTIFFBE = []byte{0x4D, 0x4D, 0x00, 0x2B, 0x00, 0x08, 0x00, 0x00}

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatTIFF, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return imgutil.ProbeBytes(r, magicTIFFLE) ||
		imgutil.ProbeBytes(r, magicTIFFBE) ||
		imgutil.ProbeBytes(r, magicBigTIFFLE) ||
		imgutil.ProbeBytes(r, magicBigTIFFBE)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(tiffName, err)
	}

	decodedBytes := raw
	if isBigTIFF(raw) {
		decodedBytes, err = rewriteBigTIFF(raw)
		if err != nil {
			return nil, imgutil.DecodeErrStr(tiffName, err)
		}
	}

	img, _, err := image.Decode(bytes.NewReader(decodedBytes))
	if err != nil {
		return nil, imgutil.DecodeErrStr(tiffName, err)
	}

	bounds := img.Bounds()
	decoded := &ir.ImageAsset{
		Name:       tiffName,
		Format:     ir.ImageTIFF,
		Width:      bounds.Dx(),
		Height:     bounds.Dy(),
		Channels:   imgutil.ChannelCount(img),
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
	}

	if err := imgutil.CheckPixelLimit(decoded.Width, decoded.Height, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(tiffName, err)
	}

	decoded.PixelDecode = func(_ *ir.ImageAsset) (*ir.PixelBuffer, error) {
		src, _, err := image.Decode(bytes.NewReader(decodedBytes))
		if err != nil {
			return nil, err
		}
		return &ir.PixelBuffer{
			Data:     pixel.ToRGBA(src, decoded.Width, decoded.Height),
			DataType: ir.DataTypeUint8,
			BitDepth: ir.BitDepth8,
		}, nil
	}

	return imgutil.BuildAsset(decoded, ir.FormatTIFF), nil
}

func (d *Decoder) Extensions() []string { return []string{extTIFF, extTIF} }
func (d *Decoder) FormatName() string   { return tiffFormatName }
