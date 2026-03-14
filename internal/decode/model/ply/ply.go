package ply

import (
	"bytes"
	"errors"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName   = "PLY"
	extPLY       = ".ply"
	meshName     = "PLY Mesh"
	inv255       = 1.0 / 255.0
	minFaceVerts = 3
)

var (
	magicPLY = []byte("ply")

	errBadHeader   = errors.New("invalid PLY header")
	errBadFormat   = errors.New("unsupported PLY format")
	errBadProperty = errors.New("unsupported property type")
	errBadVertex   = errors.New("failed to read vertex data")
	errBadFace     = errors.New("failed to read face data")
	errBadEdge     = errors.New("failed to read edge data")
	errNoVerts     = errors.New("PLY has zero vertices")
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatPLY, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeBytes(r, magicPLY)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	raw, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decodeErrCause("read", err)
	}

	hdr, body, err := parseHeader(raw)
	if err != nil {
		return nil, err
	}
	return decodeBody(hdr, body, opts)
}

//nolint:funlen
func decodeBody(hdr *header, body []byte, opts detect.DecodeOptions) (*ir.Asset, error) {
	if hdr.vertexCount <= 0 {
		return nil, decodeErr(errNoVerts.Error())
	}
	if opts.MaxVertices > 0 && hdr.vertexCount > opts.MaxVertices {
		return nil, decutil.DecodeErr(ir.FormatPLY, "vertex limit exceeded", nil)
	}

	maxVerts := len(body)
	if hdr.format != formatASCII && hdr.stride > 1 {
		maxVerts = len(body) / hdr.stride
	}
	if hdr.vertexCount > maxVerts {
		return nil, decodeErr(errBadVertex.Error())
	}

	vc := hdr.vertexCount
	vec3Cap := vc
	if hdr.hasNormals {
		vec3Cap += vc
	}
	arena3 := pool.NewArena[[3]float32](vec3Cap)

	positions := arena3.Alloc(vc)
	var normals [][3]float32
	var colors [][4]float32
	var texCoords [][2]float32

	if hdr.hasNormals {
		normals = arena3.Alloc(vc)
	}
	if hdr.hasColors {
		arena4 := pool.NewArena[[4]float32](vc)
		colors = arena4.Alloc(vc)
	}
	if hdr.hasTexCoord {
		arena2 := pool.NewArena[[2]float32](vc)
		texCoords = arena2.Alloc(vc)
	}

	var indices []uint32
	var edgeIndices []uint32
	var err error

	switch hdr.format {
	case formatASCII:
		sc := &decutil.LineScanner{Data: body}
		if err := readASCIIVertices(sc, hdr, positions, normals, colors, texCoords); err != nil {
			return nil, err
		}
		indices, err = readASCIIFaces(sc, hdr)
		if err != nil {
			return nil, err
		}
		edgeIndices, err = readASCIIEdges(sc, hdr)
		if err != nil {
			return nil, err
		}
	case formatBinaryLE, formatBinaryBE:
		br := bytes.NewReader(body)
		le := hdr.format == formatBinaryLE
		if err := readBinaryVertices(br, hdr, le, positions, normals, colors, texCoords); err != nil {
			return nil, err
		}
		indices, err = readBinaryFaces(br, hdr, le)
		if err != nil {
			return nil, err
		}
		edgeIndices, err = readBinaryEdges(br, hdr, le)
		if err != nil {
			return nil, err
		}
	default:
		return nil, decodeErr(errBadFormat.Error())
	}
	return buildAsset(hdr, positions, normals, colors, texCoords, indices, edgeIndices), nil
}

func buildAsset(
	hdr *header, positions, normals [][3]float32,
	colors [][4]float32, texCoords [][2]float32,
	indices, edgeIndices []uint32,
) *ir.Asset {
	data := ir.MeshData{
		VertexCount: hdr.vertexCount,
		Positions:   positions,
		Indices:     indices,
	}
	if hdr.hasNormals {
		data.Normals = normals
	}
	if hdr.hasColors {
		data.Colors0 = colors
	}
	if hdr.hasTexCoord {
		data.TexCoord0 = texCoords
	}

	primitives := []ir.Primitive{{
		Mode:          ir.Triangles,
		MaterialIndex: ir.NoIndex,
		Data:          data,
	}}
	if len(edgeIndices) > 0 {
		primitives = append(primitives, ir.Primitive{
			Mode:          ir.Lines,
			MaterialIndex: ir.NoIndex,
			Data: ir.MeshData{
				VertexCount: hdr.vertexCount,
				Positions:   positions,
				Indices:     edgeIndices,
			},
		})
	}

	asset := ir.NewAsset(ir.FormatPLY)
	asset.UpAxis = ir.YUp
	asset.Meshes = []*ir.Mesh{{
		Name:       meshName,
		Primitives: primitives,
	}}
	return asset
}

func (d *Decoder) Extensions() []string { return []string{extPLY} }
func (d *Decoder) FormatName() string   { return formatName }

func fanFromSlice(dst, face []uint32) []uint32 {
	if len(face) < minFaceVerts {
		return dst
	}
	v0 := face[0]
	for i := 2; i < len(face); i++ {
		dst = append(dst, v0, face[i-1], face[i])
	}
	return dst
}
