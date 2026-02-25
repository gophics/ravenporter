package tiff

import (
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/imgutil"
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

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatTIFF, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return imgutil.ProbeBytes(r, magicTIFFLE) || imgutil.ProbeBytes(r, magicTIFFBE)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	return imgutil.DecodeStdlibImage(r, opts, tiffName, ir.ImageTIFF, ir.FormatTIFF)
}

func (d *Decoder) Extensions() []string { return []string{extTIFF, extTIF} }
func (d *Decoder) FormatName() string   { return tiffFormatName }
