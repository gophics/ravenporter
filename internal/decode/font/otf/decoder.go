package otf

import (
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/fntutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "OTF"
	extOTF     = ".otf"
	extOTC     = ".otc"
)

var magic = []byte("OTTO")

var extensions = []string{extOTF, extOTC}

// Decoder implements detect.Decoder for OpenType fonts.
type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatOTF, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return decutil.ProbeBytes(r, magic) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatOTF, "size", err)
	}
	raw, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOTF, "read", err)
	}

	fonts, err := fntutil.BuildFonts(raw, ir.FontOTF, formatName)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOTF, "invalid font collection", err)
	}

	return &ir.Asset{
		Fonts:    fonts,
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatOTF},
	}, nil
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }
