package stl

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

const (
	headerSize      = 80
	triangleSize    = 50
	triCountSize    = 4
	attrByteOff     = 48
	vertsPerTri     = 3
	floatsPerTri    = 12
	normalFloats    = 3
	floatSize       = 4
	maxTriangles    = 50_000_000
	formatName      = "STL"
	extSTL          = ".stl"
	defaultMeshName = "STL Mesh"
	minVertexFields = 4
	minNormalFields = 5

	colorBitValid = 1 << 15
	inv31         = 1.0 / 31.0
	bitsPerChan   = 5
	chanMask      = 0x1F

	asciiPrefix    = "solid"
	asciiProbeSize = 6
	asciiFacet     = "facet"
	asciiNormal    = "normal"
	asciiVertex    = "vertex"
)

var (
	errHeader    = errors.New("failed to read header")
	errTriCount  = errors.New("failed to read triangle count")
	errZeroTris  = errors.New("STL has zero triangles")
	errBadVertex = errors.New("invalid ASCII vertex data")
	errTooMany   = errors.New("triangle count exceeds limit")
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatSTL, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	probed := isBinary(r)
	if !probed {
		probed = isASCII(r)
	}
	if _, err := r.Seek(pos, io.SeekStart); err != nil {
		return false
	}
	return probed
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	if isBinary(r) {
		return decodeBinary(r, opts)
	}
	return decodeASCII(r, opts)
}

func (d *Decoder) Extensions() []string { return []string{extSTL} }
func (d *Decoder) FormatName() string   { return formatName }

func isASCII(r io.ReadSeeker) bool {
	buf := pool.GetBuffer(asciiProbeSize)
	defer pool.PutBuffer(buf)
	n, err := r.Read(buf[:6])
	if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
		return false
	}
	return err == nil && n >= len(asciiPrefix) && decutil.Bstr(buf[:len(asciiPrefix)]) == asciiPrefix
}

func isBinary(r io.ReadSeeker) bool {
	buf := pool.GetBuffer(headerSize + triCountSize)
	defer pool.PutBuffer(buf)

	n, err := r.Read(buf[:headerSize+triCountSize])
	if err != nil && err != io.EOF {
		if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
			return false
		}
		return false
	}
	if n < headerSize+triCountSize {
		if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
			return false
		}
		return false
	}

	triCount := binread.ReadU32LE(buf[headerSize:])
	if triCount == 0 || triCount > maxTriangles {
		if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
			return false
		}
		return false
	}

	size, err := r.Seek(0, io.SeekEnd)
	if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
		return false
	}
	if err != nil {
		return false
	}

	expectedSize := int64(headerSize+triCountSize) + int64(triCount)*triangleSize
	return size == expectedSize
}

func decodeBinary(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, decodeErrCause(errHeader.Error(), err)
	}
	solidName := strings.TrimSpace(binread.CString(header[:]))

	var countBuf [triCountSize]byte
	if _, err := io.ReadFull(r, countBuf[:]); err != nil {
		return nil, decodeErrCause(errTriCount.Error(), err)
	}
	triCount := int(binread.ReadU32LE(countBuf[:]))

	if err := validateTriCount(triCount, opts); err != nil {
		return nil, err
	}

	vertCount := triCount * vertsPerTri
	positions := make([][3]float32, vertCount)
	normals := make([][3]float32, vertCount)
	indices := make([]uint32, vertCount)

	hasColors := false
	var colors [][4]float32

	const chunkTris = 1024
	chunkBufSize := chunkTris * triangleSize
	buf := pool.GetBuffer(chunkBufSize)
	defer pool.PutBuffer(buf)

	for i := 0; i < triCount; i += chunkTris {
		n := min(chunkTris, triCount-i)
		if _, err := io.ReadFull(r, buf[:n*triangleSize]); err != nil {
			return nil, decodeErrCause(errTriCount.Error(), err)
		}

		for j := range n {
			tri := buf[j*triangleSize:]
			normal := readFloat3(tri)

			attr := binread.ReadU16LE(tri[attrByteOff:])
			if !hasColors && attr&colorBitValid != 0 {
				hasColors = true
				colors = make([][4]float32, vertCount)
			}

			base := (i + j) * vertsPerTri
			for v := range vertsPerTri {
				off := (normalFloats + v*normalFloats) * floatSize
				positions[base+v] = readFloat3(tri[off:])
				normals[base+v] = normal
				indices[base+v] = uint32(base + v) //nolint:gosec // bounded
			}

			if hasColors {
				c := rgb555ToColor(attr)
				colors[base] = c
				colors[base+1] = c
				colors[base+2] = c
			}
		}
	}

	colors = applyMagicsColor(hasColors, header[:], colors, vertCount)

	name := defaultMeshName
	if solidName != "" {
		name = solidName
	}

	return buildAsset(name, positions, normals, colors, indices), nil
}

func decodeASCII(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	body, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decodeErrCause(errHeader.Error(), err)
	}

	sc := &decutil.LineScanner{Data: body}
	var positions [][3]float32
	var normals [][3]float32
	var currentNormal [3]float32
	var fields []string
	solidName := ""

	first := sc.Next()
	if first != nil {
		firstLine := decutil.Bstr(first)
		if strings.HasPrefix(firstLine, asciiPrefix) {
			solidName = strings.TrimSpace(firstLine[len(asciiPrefix):])
		}
	}

	for {
		line := sc.Next()
		if line == nil {
			break
		}
		fields = decutil.SplitFields(decutil.Bstr(line), fields)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case asciiFacet:
			if len(fields) >= minNormalFields && fields[1] == asciiNormal {
				currentNormal = parseFloat3Fields(fields[2], fields[3], fields[4])
			}
		case asciiVertex:
			if len(fields) < minVertexFields {
				return nil, decodeErr(errBadVertex.Error())
			}
			pos := parseFloat3Fields(fields[1], fields[2], fields[3])
			positions = append(positions, pos)
			normals = append(normals, currentNormal)
		}
	}

	if len(positions) == 0 {
		return nil, decodeErr(errZeroTris.Error())
	}
	if opts.MaxVertices > 0 && len(positions) > opts.MaxVertices {
		return nil, decodeErr("vertex limit exceeded")
	}

	indices := make([]uint32, len(positions))
	for i := range indices {
		indices[i] = uint32(i) //nolint:gosec // bounded
	}

	name := defaultMeshName
	if solidName != "" {
		name = solidName
	}

	return buildAsset(name, positions, normals, nil, indices), nil
}

func buildAsset(name string, positions, normals [][3]float32, colors [][4]float32, indices []uint32) *ir.Asset {
	asset := ir.NewAsset(ir.FormatSTL)
	asset.UpAxis = ir.YUp
	asset.Meshes = []*ir.Mesh{{
		Name: name,
		Primitives: []ir.Primitive{{
			Mode:          ir.Triangles,
			MaterialIndex: ir.NoIndex,
			Data: ir.MeshData{
				VertexCount: len(positions),
				Positions:   positions,
				Normals:     normals,
				Colors0:     colors,
				Indices:     indices,
			},
		}},
	}}
	return asset
}

func rgb555ToColor(attr uint16) [4]float32 {
	b := float32(attr&chanMask) * inv31
	g := float32((attr>>bitsPerChan)&chanMask) * inv31
	r := float32((attr>>(bitsPerChan*2))&chanMask) * inv31 //nolint:mnd // 2 shifts for R channel
	return [4]float32{r, g, b, 1.0}
}

var magicsColorTag = []byte("COLOR=") //nolint:gochecknoglobals // Magics header marker

func applyMagicsColor(hasColors bool, header []byte, colors [][4]float32, vertCount int) [][4]float32 {
	if hasColors {
		return colors
	}
	mc := parseMagicsHeaderColor(header)
	if mc == nil {
		return colors
	}
	colors = make([][4]float32, vertCount)
	for i := range colors {
		colors[i] = *mc
	}
	return colors
}

const inv255 = 1.0 / 255.0

func parseMagicsHeaderColor(header []byte) *[4]float32 {
	idx := bytes.Index(header, magicsColorTag)
	if idx < 0 || idx+len(magicsColorTag)+4 > len(header) {
		return nil
	}
	base := idx + len(magicsColorTag)
	c := [4]float32{
		float32(header[base]) * inv255,
		float32(header[base+1]) * inv255,
		float32(header[base+2]) * inv255,
		float32(header[base+3]) * inv255,
	}
	return &c
}

func parseFloat3Fields(a, b, c string) [3]float32 {
	return [3]float32{decutil.ParseF32(a), decutil.ParseF32(b), decutil.ParseF32(c)}
}

func decodeErr(msg string) error {
	return decutil.DecodeErr(ir.FormatSTL, msg, nil)
}

func decodeErrCause(msg string, cause error) error {
	return decutil.DecodeErr(ir.FormatSTL, msg, cause)
}

func readFloat3(b []byte) [3]float32 {
	return [3]float32{
		binread.ReadF32LE(b),
		binread.ReadF32LE(b[floatSize:]),
		binread.ReadF32LE(b[floatSize*2:]),
	}
}

func validateTriCount(triCount int, opts detect.DecodeOptions) error {
	if triCount == 0 {
		return decodeErr(errZeroTris.Error())
	}
	if triCount > maxTriangles {
		return decodeErr(errTooMany.Error() + " " + strconv.Itoa(triCount))
	}
	if opts.MaxVertices > 0 && triCount*vertsPerTri > opts.MaxVertices {
		return decodeErr("vertex limit exceeded")
	}
	return nil
}
