package gltf

import (
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "glTF 2.0"
	gltfName   = "gltf"
	extGLTF    = ".gltf"
	extGLB     = ".glb"
)

var magicGLB = []byte("glTF")

func Registrations() []detect.Registration {
	return []detect.Registration{
		{Format: ir.FormatGLB, Decoder: &Decoder{}},
		{Format: ir.FormatGLTF, Decoder: &Decoder{}},
	}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeBytes(r, magicGLB)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	return decodeStream(opts.Context, r, opts)
}

func (d *Decoder) Extensions() []string { return []string{extGLTF, extGLB} }
func (d *Decoder) FormatName() string   { return formatName }
