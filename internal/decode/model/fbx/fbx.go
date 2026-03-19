package fbx

import (
	"bytes"
	"context"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName     = "FBX"
	extFBX         = ".fbx"
	headerSize     = 27
	versionOff     = 23
	v7500          = 7500
	asciiProbLen   = 64
	asciiMagicLine = "; FBX"
)

var (
	magicFBX   = []byte("Kaydara FBX Binary  \x00")
	extensions = []string{extFBX}
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatFBX, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	if decutil.ProbeBytes(r, magicFBX) {
		return true
	}
	return decutil.ProbeContains(r, []byte(asciiMagicLine))
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatFBX, "size", err)
	}

	data, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatFBX, "read", err)
	}

	if isBinaryFBX(data) {
		return decodeBinaryFBX(opts.Context, data)
	}
	return decodeASCIIFBX(opts.Context, data)
}

func isBinaryFBX(data []byte) bool {
	return len(data) >= len(magicFBX) && bytes.Equal(data[:len(magicFBX)], magicFBX)
}

func decodeBinaryFBX(sysCtx context.Context, data []byte) (*ir.Asset, error) {
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	if len(data) < headerSize {
		return nil, decutil.DecodeErr(ir.FormatFBX, "header", errTooShort)
	}
	version := binread.ReadU32LE(data[versionOff:])
	ctx := parseCtx{
		nodes: pool.NewArena[fbxNode](len(data) / 128), //nolint:mnd // ~1 node per 128 bytes
		props: pool.NewArena[fbxProp](len(data) / 64),  //nolint:mnd // ~1 prop per 64 bytes
	}
	nodes, err := parseNodesAt(sysCtx, data, headerSize, version, 0, &ctx)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatFBX, "parse", err)
	}
	return convertFBX(nodes, version), nil
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }
