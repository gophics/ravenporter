package obj

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName   = "OBJ"
	extOBJ       = ".obj"
	minFaceVerts = 3
)

var (
	errBadVertex  = errors.New("failed to read vertex data")
	errBadFace    = errors.New("failed to read face data")
	errNoVerts    = errors.New("OBJ has zero vertices")
	errBadIndex   = errors.New("vertex index out of range")
	errBadVp      = errors.New("bad parameter vertex")
	errBadCurv    = errors.New("bad curv statement")
	errBadCurv2   = errors.New("bad curv2 statement")
	errBadSurf    = errors.New("bad surf statement")
	errBadParm    = errors.New("bad parm statement")
	errBadTrim    = errors.New("bad trim/hole value")
	errBadCurvIdx = errors.New("bad curve vertex index")
	errBadSurfVtx = errors.New("bad surf vertex")
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatOBJ, Decoder: &Decoder{}}}
}

type Decoder struct{}

var magicV = []byte("v ")

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeContains(r, magicV)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	data, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOBJ, err.Error(), err)
	}

	parsed, err := parseOBJ(opts.Context, data)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOBJ, err.Error(), err)
	}
	if len(parsed.positions) == 0 {
		return nil, decutil.DecodeErr(ir.FormatOBJ, errNoVerts.Error(), nil)
	}

	asset := ir.NewAsset(ir.FormatOBJ)
	asset.UpAxis = ir.YUp

	if parsed.mtlLib != "" {
		if err := parseMTL(opts.FS, opts.Reporter, parsed.mtlLib, opts.MaxFileSize, asset); err != nil {
			return nil, decutil.DecodeErr(ir.FormatOBJ, err.Error(), err)
		}
	}

	matMap := buildMaterialMap(asset)

	for i := range parsed.groups {
		mesh, err := buildMesh(parsed, parsed.groups[i], matMap, opts)
		if err != nil {
			return nil, err
		}
		if mesh == nil {
			continue
		}
		asset.Meshes = append(asset.Meshes, mesh)
	}

	for i := range parsed.groups {
		if len(parsed.groups[i].lines) == 0 {
			continue
		}
		lineMesh := buildLineMesh(parsed, parsed.groups[i])
		if lineMesh != nil {
			asset.Meshes = append(asset.Meshes, lineMesh)
		}
	}

	for i := range parsed.groups {
		if len(parsed.groups[i].points) == 0 {
			continue
		}
		ptMesh := buildPointMesh(parsed, parsed.groups[i])
		if ptMesh != nil {
			asset.Meshes = append(asset.Meshes, ptMesh)
		}
	}

	for i := range parsed.curves3D {
		mesh := buildCurveMesh(parsed, parsed.curves3D[i])
		if mesh != nil {
			asset.Meshes = append(asset.Meshes, mesh)
		}
	}

	for i := range parsed.surfaces {
		mesh := buildSurfaceMesh(parsed, parsed.surfaces[i])
		if mesh != nil {
			asset.Meshes = append(asset.Meshes, mesh)
		}
	}

	if len(asset.Meshes) == 0 {
		return nil, decutil.DecodeErr(ir.FormatOBJ, errNoVerts.Error(), nil)
	}

	assignOBJNodes(asset)

	return asset, nil
}

func assignOBJNodes(asset *ir.Asset) {
	lodGroupsMap := make(map[string]int)
	for i, m := range asset.Meshes {
		nodeIdx := len(asset.Nodes)
		node := ir.Node{
			Name:          m.Name,
			ParentIndex:   ir.NoIndex,
			MeshIndex:     i,
			SkinIndex:     ir.NoIndex,
			CameraIndex:   ir.NoIndex,
			LightIndex:    ir.NoIndex,
			LODGroupIndex: ir.NoIndex,
		}

		if idx := strings.LastIndex(m.Name, "_LOD"); idx > 0 {
			prefix := m.Name[:idx]
			lodStr := m.Name[idx+4:]
			if _, err := strconv.Atoi(lodStr); err == nil {
				gIdx, ok := lodGroupsMap[prefix]
				if !ok {
					gIdx = len(asset.LODGroups)
					lodGroupsMap[prefix] = gIdx
					asset.LODGroups = append(asset.LODGroups, &ir.LODGroup{Name: prefix})
				}
				node.LODGroupIndex = gIdx
				asset.LODGroups[gIdx].Levels = append(asset.LODGroups[gIdx].Levels, ir.LODLevel{
					Threshold: 0.0,
					NodeIndex: nodeIdx,
				})
			}
		}

		asset.Nodes = append(asset.Nodes, node)
		asset.RootNodes = append(asset.RootNodes, nodeIdx)
	}
}

func (d *Decoder) Extensions() []string { return []string{extOBJ} }
func (d *Decoder) FormatName() string   { return formatName }

func buildMaterialMap(asset *ir.Asset) map[string]int {
	m := make(map[string]int, len(asset.Materials))
	for i, mat := range asset.Materials {
		m[mat.Name] = i
	}
	return m
}

func buildMesh(parsed *objData, g group, matMap map[string]int, opts detect.DecodeOptions) (*ir.Mesh, error) {
	if len(g.faces) == 0 {
		return nil, nil
	}

	estVerts := len(g.faces) * minFaceVerts
	dedup := make(map[[3]int]uint32, estVerts)
	positions := make([][3]float32, 0, estVerts)
	var normals [][3]float32
	var uvs [][2]float32
	var colors [][4]float32
	indices := make([]uint32, 0, estVerts)

	for _, f := range g.faces {
		if f.vertCount < minFaceVerts {
			continue
		}
		verts := g.vertRefs[f.vertStart : f.vertStart+f.vertCount]
		idx0, err := dedupVertex(parsed, verts[0], dedup, &positions, &normals, &uvs, &colors)
		if err != nil {
			return nil, err
		}
		prev, err := dedupVertex(parsed, verts[1], dedup, &positions, &normals, &uvs, &colors)
		if err != nil {
			return nil, err
		}
		for j := 2; j < len(verts); j++ {
			cur, err := dedupVertex(parsed, verts[j], dedup, &positions, &normals, &uvs, &colors)
			if err != nil {
				return nil, err
			}
			indices = append(indices, idx0, prev, cur)
			prev = cur
		}
	}

	if opts.MaxVertices > 0 && len(positions) > opts.MaxVertices {
		return nil, decutil.DecodeErr(ir.FormatOBJ, "vertex limit exceeded", nil)
	}

	data := ir.MeshData{
		VertexCount: len(positions),
		Positions:   positions,
		Indices:     indices,
	}
	if len(normals) == len(positions) {
		data.Normals = normals
	}
	if len(uvs) == len(positions) {
		data.TexCoord0 = uvs
	}
	if len(colors) == len(positions) {
		data.Colors0 = colors
	}

	matIdx := ir.NoIndex
	if idx, ok := matMap[g.material]; ok {
		matIdx = idx
	}

	return &ir.Mesh{
		Name: g.name,
		Primitives: []ir.Primitive{{
			Mode:          ir.Triangles,
			MaterialIndex: matIdx,
			Data:          data,
		}},
	}, nil
}

func buildLineMesh(parsed *objData, g group) *ir.Mesh {
	if len(g.lines) == 0 {
		return nil
	}
	return &ir.Mesh{
		Name: g.name + "_lines",
		Primitives: []ir.Primitive{{
			Mode:          ir.Lines,
			MaterialIndex: ir.NoIndex,
			Data: ir.MeshData{
				VertexCount: len(parsed.positions),
				Positions:   parsed.positions,
				Indices:     g.lines,
			},
		}},
	}
}

func buildPointMesh(parsed *objData, g group) *ir.Mesh {
	if len(g.points) == 0 {
		return nil
	}
	return &ir.Mesh{
		Name: g.name + "_points",
		Primitives: []ir.Primitive{{
			Mode:          ir.Points,
			MaterialIndex: ir.NoIndex,
			Data: ir.MeshData{
				VertexCount: len(parsed.positions),
				Positions:   parsed.positions,
				Indices:     g.points,
			},
		}},
	}
}

func dedupVertex(
	parsed *objData, v vertexRef, dedup map[[3]int]uint32,
	positions *[][3]float32, normals *[][3]float32, uvs *[][2]float32, colors *[][4]float32,
) (uint32, error) {
	key := [3]int{v.pos, v.uv, v.norm}
	if idx, ok := dedup[key]; ok {
		return idx, nil
	}

	pi := resolveIndex(v.pos, len(parsed.positions))
	if pi < 0 || pi >= len(parsed.positions) {
		return 0, decutil.DecodeErr(ir.FormatOBJ, errBadIndex.Error(), nil)
	}

	idx := uint32(len(*positions)) //nolint:gosec // bounded
	*positions = append(*positions, parsed.positions[pi])

	if v.norm != 0 {
		ni := resolveIndex(v.norm, len(parsed.normals))
		if ni >= 0 && ni < len(parsed.normals) {
			*normals = append(*normals, parsed.normals[ni])
		} else {
			*normals = append(*normals, [3]float32{})
		}
	}

	if v.uv != 0 {
		ui := resolveIndex(v.uv, len(parsed.texCoords))
		if ui >= 0 && ui < len(parsed.texCoords) {
			*uvs = append(*uvs, parsed.texCoords[ui])
		} else {
			*uvs = append(*uvs, [2]float32{})
		}
	}

	if pi < len(parsed.vertexColors) {
		*colors = append(*colors, parsed.vertexColors[pi])
	}

	dedup[key] = idx
	return idx, nil
}

func resolveIndex(idx, count int) int {
	if idx > 0 {
		return idx - 1
	}
	if idx < 0 {
		return count + idx
	}
	return -1
}

func decodeErr(msg string) error {
	return decutil.DecodeErr(ir.FormatOBJ, msg, nil)
}

func decodeErrCause(msg string, cause error) error {
	return decutil.DecodeErr(ir.FormatOBJ, msg, cause)
}
