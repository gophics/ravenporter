package tds

import (
	"errors"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName     = "3D Studio"
	ext3DS         = ".3ds"
	defaultMatName = "3ds_material"
	propAmbient    = "ambient"

	probeLen       = 6
	probeHdrSize   = 8
	chunkHdrSize   = 6
	vertexStride   = 12
	uvStride       = 8
	faceStride     = 8
	vertsPerTri    = 3
	matrixFloats   = 12
	lightDataSize  = 12
	spotDataSize   = 16
	cameraDataSize = 32
	colorBytes     = 3
	colorScale     = 255.0
	defaultGray    = 128
	minChunkBody   = 2
	u32Size        = 4
	fovOffset      = 28

	lumaR = 0.2126
	lumaG = 0.7152
	lumaB = 0.0722

	chunkMain     = 0x4D4D
	chunkEditor   = 0x3D3D
	chunkObject   = 0x4000
	chunkTriMesh  = 0x4100
	chunkVertices = 0x4110
	chunkFaces    = 0x4120
	chunkFaceMat  = 0x4130
	chunkTexCoord = 0x4140
	chunkSmooth   = 0x4150
	chunkLocalMat = 0x4160

	chunkMaterial    = 0xAFFF
	chunkMatName     = 0xA000
	chunkMatAmbient  = 0xA010
	chunkMatDiffuse  = 0xA020
	chunkMatSpecular = 0xA030
	chunkMatTexMap   = 0xA200
	chunkMatSpecMap  = 0xA204
	chunkMatOpacMap  = 0xA210
	chunkMatReflMap  = 0xA220
	chunkMatBumpMap  = 0xA230
	chunkMatEmisMap  = 0xA240
	chunkMatTexFile  = 0xA300
	chunkColor24     = 0x0011

	chunkLight       = 0x4600
	chunkSpotlight   = 0x4610
	chunkDirectional = 0x4613
	chunkCamera      = 0x4700

	defaultNear     = 1.0
	defaultFar      = 1000.0
	defaultRough    = 0.5
	defaultColorVal = 0.5
	degreesToRad    = 3.14159265358979323846 / 180.0
)

var errNotTDS = errors.New("not a 3DS file")

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.Format3DS, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeRead(r, probeHdrSize, func(buf []byte) bool {
		if len(buf) < probeHdrSize {
			return false
		}
		if binread.ReadU16LE(buf[:2]) != chunkMain {
			return false
		}
		size := binread.ReadU32LE(buf[2:6])
		if size < chunkHdrSize {
			return false
		}
		// Avoid collision with TIFF_BE (0x4D, 0x4D, 0x00, 0x2A)
		return buf[2] != 0x00 || buf[3] != 0x2A
	})
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	data, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.Format3DS, err.Error(), err)
	}
	if len(data) < chunkHdrSize || binread.ReadU16LE(data[:2]) != chunkMain {
		return nil, decutil.DecodeErr(ir.Format3DS, errNotTDS.Error(), errNotTDS)
	}

	asset := ir.NewAsset(ir.Format3DS)
	ctx := &parseCtx{asset: asset, matNames: map[string]int{}}
	end := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
	if end < chunkHdrSize {
		return asset, nil
	}
	walkChunks(data[chunkHdrSize:end], ctx)
	return asset, nil
}

func (d *Decoder) Extensions() []string { return []string{ext3DS} }
func (d *Decoder) FormatName() string   { return formatName }

type parseCtx struct {
	asset    *ir.Asset
	matNames map[string]int
}

type faceMaterialGroup struct {
	materialIndex int
	faceIndices   []int
}

func walkChunks(data []byte, ctx *parseCtx) {
	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			return
		}
		body := data[chunkHdrSize:size]

		switch id {
		case chunkEditor:
			walkChunks(body, ctx)
		case chunkMaterial:
			parseMaterial(body, ctx)
		case chunkObject:
			parseObject(body, ctx)
		case chunkKeyframer:
			parseKeyframer(body, ctx)
		}

		data = data[size:]
	}
}

func parseObject(data []byte, ctx *parseCtx) {
	name := binread.CString(data)
	rest := data[binread.CStringLen(data):]

	for len(rest) >= chunkHdrSize {
		id := binread.ReadU16LE(rest[:2])
		size := binread.ClampChunkSize(len(rest), binread.ReadU32LE(rest[2:6]))
		if size < chunkHdrSize {
			break
		}
		body := rest[chunkHdrSize:size]

		switch id {
		case chunkTriMesh:
			parseTriMesh(name, body, ctx)
		case chunkLight:
			parseLight(name, body, ctx)
		case chunkCamera:
			parseCameraChunk(name, body, ctx)
		}

		rest = rest[size:]
	}
}

func parseTriMesh(name string, data []byte, ctx *parseCtx) {
	md := ir.MeshData{}
	matIndex := ir.NoIndex
	var faceMats []faceMaterialGroup
	var localMatrix [matrixFloats]float32
	hasMatrix := false
	var smoothGroups []int

	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			break
		}
		body := data[chunkHdrSize:size]

		switch id {
		case chunkVertices:
			readVertices(body, &md)
		case chunkFaces:
			readFaces(body, &md, &matIndex, &faceMats, ctx)
		case chunkTexCoord:
			readUVs(body, &md)
		case chunkSmooth:
			smoothGroups = readSmoothGroups(body, len(md.Indices)/vertsPerTri)
		case chunkLocalMat:
			if len(body) >= matrixFloats*u32Size {
				rawMatrix := body[:matrixFloats*u32Size]
				for i := range matrixFloats {
					localMatrix[i] = binread.ReadF32LE(rawMatrix[i*u32Size:]) //nolint:gosec // bounded by matrixFloats
				}
				hasMatrix = true
			}
		}

		data = data[size:]
	}

	if md.VertexCount == 0 {
		return
	}

	mesh := &ir.Mesh{Name: name}
	md.SmoothGroups = smoothGroups
	base := ir.Primitive{Mode: ir.Triangles, Data: md, MaterialIndex: matIndex}
	mesh.Primitives = splitFaceMaterialGroups(base, faceMats)
	meshIdx := len(ctx.asset.Meshes)
	ctx.asset.Meshes = append(ctx.asset.Meshes, mesh)

	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   meshIdx,
		SkinIndex:   ir.NoIndex,
		CameraIndex: ir.NoIndex,
		LightIndex:  ir.NoIndex,
	}
	if hasMatrix {
		node.Transform = ir.Transform{Matrix: [16]float32(mathx.Mat4From3x4(localMatrix))}
	}
	nodeIdx := len(ctx.asset.Nodes)
	ctx.asset.Nodes = append(ctx.asset.Nodes, node)
	ctx.asset.RootNodes = append(ctx.asset.RootNodes, nodeIdx)
}

func readVertices(body []byte, md *ir.MeshData) {
	if len(body) < minChunkBody {
		return
	}
	count := int(binread.ReadU16LE(body[:2]))
	body = body[2:]
	if len(body) < count*vertexStride {
		return
	}
	md.VertexCount = count
	md.Positions = make([][3]float32, count)
	for i := range count {
		off := i * vertexStride
		md.Positions[i] = [3]float32{
			binread.ReadF32LE(body[off:]),
			binread.ReadF32LE(body[off+u32Size:]),
			binread.ReadF32LE(body[off+u32Size*2:]),
		}
	}
}

func readFaces(
	body []byte, md *ir.MeshData, matIndex *int, faceMats *[]faceMaterialGroup, ctx *parseCtx,
) {
	if len(body) < minChunkBody {
		return
	}
	count := int(binread.ReadU16LE(body[:2]))
	body = body[2:]
	if len(body) < count*faceStride {
		return
	}
	md.Indices = make([]uint32, count*vertsPerTri)
	for i := range count {
		off := i * faceStride
		md.Indices[i*vertsPerTri] = uint32(binread.ReadU16LE(body[off:]))
		md.Indices[i*vertsPerTri+1] = uint32(binread.ReadU16LE(body[off+2:]))
		md.Indices[i*vertsPerTri+2] = uint32(binread.ReadU16LE(body[off+u32Size:]))
	}

	remaining := body[count*faceStride:]
	for len(remaining) >= chunkHdrSize {
		id := binread.ReadU16LE(remaining[:2])
		size := binread.ClampChunkSize(len(remaining), binread.ReadU32LE(remaining[2:6]))
		if size < chunkHdrSize {
			break
		}
		if id == chunkFaceMat {
			matBody := remaining[chunkHdrSize:size]
			nameLen := binread.CStringLen(matBody)
			name := binread.CString(matBody)
			if idx, ok := ctx.matNames[name]; ok {
				if len(matBody) < nameLen+minChunkBody {
					*matIndex = idx
					remaining = remaining[size:]
					continue
				}
				count := int(binread.ReadU16LE(matBody[nameLen:]))
				faceBody := matBody[nameLen+minChunkBody:]
				if len(faceBody) < count*minChunkBody {
					*matIndex = idx
					remaining = remaining[size:]
					continue
				}
				group := faceMaterialGroup{
					materialIndex: idx,
					faceIndices:   make([]int, 0, count),
				}
				for i := range count {
					group.faceIndices = append(group.faceIndices, int(binread.ReadU16LE(faceBody[i*minChunkBody:])))
				}
				*faceMats = append(*faceMats, group)
				*matIndex = idx
			}
		}
		remaining = remaining[size:]
	}
}

func readUVs(body []byte, md *ir.MeshData) {
	if len(body) < minChunkBody {
		return
	}
	count := int(binread.ReadU16LE(body[:2]))
	body = body[2:]
	if len(body) < count*uvStride {
		return
	}
	md.TexCoord0 = make([][2]float32, count)
	for i := range count {
		off := i * uvStride
		md.TexCoord0[i] = [2]float32{
			binread.ReadF32LE(body[off:]),
			binread.ReadF32LE(body[off+u32Size:]),
		}
	}
}

func readSmoothGroups(body []byte, faceCount int) []int {
	if len(body) < faceCount*u32Size {
		return nil
	}
	groups := make([]int, faceCount)
	for i := range faceCount {
		groups[i] = int(binread.ReadU32LE(body[i*u32Size:]))
	}
	return groups
}

func splitFaceMaterialGroups(base ir.Primitive, groups []faceMaterialGroup) []ir.Primitive {
	if len(groups) == 0 || len(base.Data.Indices) == 0 {
		return []ir.Primitive{base}
	}

	totalFaces := len(base.Data.Indices) / vertsPerTri
	assigned := make([]bool, totalFaces)
	prims := make([]ir.Primitive, 0, len(groups)+1)

	for _, group := range groups {
		if len(group.faceIndices) == 0 {
			continue
		}

		prim := base
		prim.MaterialIndex = group.materialIndex
		prim.Data.Indices = make([]uint32, 0, len(group.faceIndices)*vertsPerTri)
		if len(base.Data.SmoothGroups) > 0 {
			prim.Data.SmoothGroups = make([]int, 0, len(group.faceIndices))
		}

		for _, faceIdx := range group.faceIndices {
			if faceIdx < 0 || faceIdx >= totalFaces {
				continue
			}
			start := faceIdx * vertsPerTri
			prim.Data.Indices = append(prim.Data.Indices, base.Data.Indices[start:start+vertsPerTri]...)
			if len(base.Data.SmoothGroups) > faceIdx {
				prim.Data.SmoothGroups = append(prim.Data.SmoothGroups, base.Data.SmoothGroups[faceIdx])
			}
			assigned[faceIdx] = true
		}

		if len(prim.Data.Indices) > 0 {
			prims = append(prims, prim)
		}
	}

	if len(prims) == 0 {
		return []ir.Primitive{base}
	}

	remainingFaces := 0
	for _, used := range assigned {
		if !used {
			remainingFaces++
		}
	}
	if remainingFaces == 0 {
		return prims
	}

	prim := base
	prim.Data.Indices = make([]uint32, 0, remainingFaces*vertsPerTri)
	if len(base.Data.SmoothGroups) > 0 {
		prim.Data.SmoothGroups = make([]int, 0, remainingFaces)
	}
	for faceIdx, used := range assigned {
		if used {
			continue
		}
		start := faceIdx * vertsPerTri
		prim.Data.Indices = append(prim.Data.Indices, base.Data.Indices[start:start+vertsPerTri]...)
		if len(base.Data.SmoothGroups) > faceIdx {
			prim.Data.SmoothGroups = append(prim.Data.SmoothGroups, base.Data.SmoothGroups[faceIdx])
		}
	}
	prims = append(prims, prim)

	return prims
}
