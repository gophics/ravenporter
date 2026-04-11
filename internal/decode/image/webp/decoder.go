package webp

import (
	"bytes"
	"image"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/ir"
	_ "golang.org/x/image/webp" // register WebP codec
)

const (
	webpFormatName = "WebP"
	webpName       = "webp"
	extWebP        = ".webp"
	riffWebPOffset = 8
	riffProbeSize  = 12
)

var magicWebP = []byte("RIFF")
var markerWebP = []byte("WEBP")

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatWebP, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	defer func() { _, _ = r.Seek(pos, io.SeekStart) }() //nolint:errcheck // reset pos

	var buf [riffProbeSize]byte
	n, err := r.Read(buf[:])
	if err != nil || n < riffProbeSize {
		return false
	}
	return bytes.HasPrefix(buf[:], magicWebP) && bytes.Equal(buf[riffWebPOffset:riffProbeSize], markerWebP)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(webpName, err)
	}

	if isAnimatedWebP(raw) {
		return decodeAnimatedWebP(raw, opts.MaxImagePixels)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, imgutil.DecodeErrStr(webpName, err)
	}
	if err := imgutil.CheckPixelLimit(cfg.Width, cfg.Height, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(webpName, err)
	}

	decoded := &ir.ImageAsset{
		Name:       webpName,
		Format:     ir.ImageWebP,
		Width:      cfg.Width,
		Height:     cfg.Height,
		Channels:   ir.ChannelRGBA,
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
		PixelDecode: func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
			img, _, err := image.Decode(bytes.NewReader(d.Compressed))
			if err != nil {
				return nil, err
			}
			return &ir.PixelBuffer{
				Data:     rgbaPixels(img, d.Width, d.Height),
				DataType: ir.DataTypeUint8,
				BitDepth: ir.BitDepth8,
			}, nil
		},
	}
	return imgutil.BuildAsset(decoded, ir.FormatWebP), nil
}

func (d *Decoder) Extensions() []string { return []string{extWebP} }
func (d *Decoder) FormatName() string   { return webpFormatName }
