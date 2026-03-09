package ttf

import (
	"bytes"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/fntutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "TTF"
	extTTF     = ".ttf"
)

var magic = []byte{0x00, 0x01, 0x00, 0x00}

var extensions = []string{extTTF}

// Decoder implements detect.Decoder for TrueType fonts.
type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatTTF, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return decutil.ProbeBytes(r, magic) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatTTF, "size", err)
	}
	raw, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatTTF, "read", err)
	}

	if len(raw) < fntutil.SFNTHeaderSize || !bytes.Equal(raw[:4], magic) {
		return nil, decutil.DecodeErr(ir.FormatTTF, "invalid sfnt header", nil)
	}

	f := &ir.Font{
		Name:   formatName,
		Format: ir.FontTTF,
		Vector: &ir.VectorFontData{RawData: raw},
	}
	fntutil.ParseSFNTMetrics(raw, f)

	return &ir.Asset{
		Fonts:    []*ir.Font{f},
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatTTF},
	}, nil
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }
