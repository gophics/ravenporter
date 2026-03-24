package abc

import (
	"math"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/ir"
)

func (p *ogawaParser) tryExtractMeshFromProperties(offset, depth int) {
	if depth > maxRecurseDepth {
		return
	}
	children, err := p.loadGroup(offset)
	if err != nil || len(children) == 0 {
		return
	}

	if props := p.readCompoundPropertyNames(children); len(props) > 0 {
		if _, ok := props[abcPropFocalLen]; ok {
			p.extractCameraFromNamedProps(children, props)
			return
		}
		if _, ok := props[abcPropFocalLen2]; ok {
			p.extractCameraFromNamedProps(children, props)
			return
		}
		p.extractPolyMeshFromNamedProps(children, props)
		return
	}

	for _, child := range children {
		if isData(child) {
			p.tryExtractFromDataChild(child)
		} else if isGroup(child) {
			if subPos := childAddr(child); subPos > 0 {
				p.tryExtractMeshFromProperties(subPos, depth+1)
			}
		}
	}
}

func (p *ogawaParser) tryExtractFromDataChild(child uint64) {
	pos := childAddr(child)
	if pos <= 0 || pos >= len(p.data) || len(p.data)-pos < ogawaU64Size {
		return
	}
	rawSize := binread.ReadU64LE(p.data[pos:])
	if rawSize == 0 || rawSize > uint64(len(p.data)) {
		return
	}
	size := int(rawSize) //nolint:gosec // bounded above
	dataStart := pos + ogawaU64Size
	if dataStart+size > len(p.data) {
		return
	}
	p.tryParseVertexArray(dataStart, size)
}

func (p *ogawaParser) tryParseVertexArray(dataStart, size int) {
	if size < vec3Stride {
		return
	}

	limit := dataStart + min(size, vertScanWindow)
	for scan := dataStart; scan+vec3Stride <= limit; scan += f32Size {
		remaining := (dataStart + size) - scan
		if remaining < vec3Stride {
			break
		}

		floatCount := remaining / f32Size
		if floatCount < minVertCount || floatCount > maxVertCount || floatCount%vec3Floats != 0 {
			continue
		}

		if !looksLikeVertexData(p.data, scan, min(floatCount, vertValidCount)) {
			continue
		}

		vertCount := floatCount / vec3Floats
		positions := make([][3]float32, vertCount)
		for i := range vertCount {
			off := scan + i*vec3Stride
			positions[i] = [3]float32{
				binread.ReadF32LE(p.data[off:]),
				binread.ReadF32LE(p.data[off+f32Size:]),
				binread.ReadF32LE(p.data[off+f32Size*2:]),
			}
		}

		p.addMeshNode(positions, nil, nil, nil, nil)
		return
	}
}

func (p *ogawaParser) addMeshNode(positions, normals [][3]float32, uvs [][2]float32, colors [][4]float32, indices []uint32) {
	data := ir.MeshData{
		Positions:   positions,
		VertexCount: len(positions),
		Indices:     indices,
	}
	if len(normals) > 0 {
		data.Normals = normals
	}
	if len(uvs) > 0 {
		data.TexCoord0 = uvs
	}
	if len(colors) > 0 {
		data.Colors0 = colors
	}

	mesh := &ir.Mesh{
		Name:       defaultMeshName,
		Primitives: []ir.Primitive{{Mode: ir.Triangles, Data: data}},
	}
	p.asset.Meshes = append(p.asset.Meshes, mesh)

	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        defaultMeshName,
		MeshIndex:   len(p.asset.Meshes) - 1,
		SkinIndex:   ir.NoIndex,
		CameraIndex: ir.NoIndex,
		LightIndex:  ir.NoIndex,
	}
	p.asset.Nodes = append(p.asset.Nodes, node)
	p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
}

func looksLikeVertexData(data []byte, offset, count int) bool {
	for i := range count {
		f := binread.ReadF32LE(data[offset+i*f32Size:])
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			return false
		}
		if f < -vertCoordMax || f > vertCoordMax {
			return false
		}
	}
	return true
}

func (p *ogawaParser) parseMetadata(meta string) {
	for kv := range strings.SplitSeq(meta, ";") {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == abcMetaUpAxis && strings.TrimSpace(val) == abcMetaUpZ {
			p.asset.UpAxis = ir.ZUp
		}
	}
}

func isGroup(child uint64) bool { return child&ogawaDataFlag == 0 }
func isData(child uint64) bool  { return !isGroup(child) }
func childAddr(child uint64) int {
	addr := child & ogawaAddrMask
	if addr > uint64(math.MaxInt) {
		return 0
	}
	return int(addr)
}

func versionString(v uint32) string {
	major := v / abcVersionDiv
	minor := v % abcVersionDiv
	return strconv.Itoa(int(major)) + "." + strconv.Itoa(int(minor))
}
