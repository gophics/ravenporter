package threemf

import (
	"bytes"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/hpinc/go3mf"
)

const (
	formatName  = "3MF"
	ext3MF      = ".3mf"
	probeLen    = 2
	probeWindow = 512
)

var (
	magicPK = []byte("PK\x03\x04")
	ctXML   = []byte("[Content_Types].xml")
	ct3D    = []byte("3D/3dmodel.model")
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.Format3MF, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeRead(r, probeWindow, func(buf []byte) bool {
		if !bytes.Equal(buf[:min(4, len(buf))], magicPK) { //nolint:mnd // zip magic
			return false
		}
		return bytes.Contains(buf, ctXML) || bytes.Contains(buf, ct3D)
	})
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.Format3MF, err.Error(), err)
	}

	model := new(go3mf.Model)
	dec := go3mf.NewDecoder(bytes.NewReader(data), int64(len(data)))
	if err = dec.Decode(model); err != nil {
		return nil, decutil.DecodeErr(ir.Format3MF, err.Error(), err)
	}

	return convertModel(model), nil
}

func (d *Decoder) Extensions() []string { return []string{ext3MF} }
func (d *Decoder) FormatName() string   { return formatName }
