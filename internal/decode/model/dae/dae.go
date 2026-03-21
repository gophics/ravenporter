package dae

import (
	"encoding/xml"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "COLLADA"
	extDAE     = ".dae"
)

var (
	colladaProbe = []byte("<COLLADA")
	extensions   = []string{extDAE}
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatDAE, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return decutil.ProbeContains(r, colladaProbe) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatDAE, "size", err)
	}

	var doc collada
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, decutil.DecodeErr(ir.FormatDAE, "xml", err)
	}
	return convertDocument(opts.Context, &doc)
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }
