package usda

import (
	"math"
	"strings"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	cubeHalfDiv      = 2
	cylinderHalfDiv  = 2
	coneHalfDiv      = 2
	cylinderStride   = 2
	cylinderIdxShift = 3
)

type geomSubset struct {
	faceIndices []int
	matName     string
}

func (p *usdaParser) parseGeomSubset() geomSubset {
	var gs geomSubset
	isFace := false
	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				if !isFace {
					gs.faceIndices = nil
				}
				return gs
			}
		case strings.Contains(line, usdaSubsetElement):
			if strings.Contains(line, usdaElementFace) {
				isFace = true
			}
		case strings.Contains(line, usdaSubsetIndices):
			gs.faceIndices = parseIntArray(p.collectArray(line))
		case strings.Contains(line, usdaMatBinding):
			gs.matName = extractMatBindingName(line)
		}
	}
	return gs
}

func (p *usdaParser) parseBlendShape(defLine string) ir.MorphTarget {
	mt := ir.MorphTarget{Name: extractQuotedName(defLine)}
	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				return mt
			}
		case strings.Contains(line, usdaBlendOffsets):
			mt.Positions = parseVec3Array(p.collectArray(line))
		case strings.Contains(line, usdaBlendPointIdx):
			mt.Indices = parseUint32Array(p.collectArray(line))
		case strings.Contains(line, usdaBlendNormalOff):
			mt.Normals = parseVec3Array(p.collectArray(line))
		}
	}
	return mt
}

const triVerts = 3

func splitBySubsets(base ir.Primitive, subsets []geomSubset, mats []*ir.Material) []ir.Primitive {
	if len(base.Data.Indices) == 0 {
		return []ir.Primitive{base}
	}
	totalTris := len(base.Data.Indices) / triVerts
	var result []ir.Primitive
	for _, gs := range subsets {
		if len(gs.faceIndices) == 0 {
			continue
		}
		var indices []uint32
		for _, fi := range gs.faceIndices {
			if fi >= 0 && fi < totalTris {
				start := fi * triVerts
				indices = append(indices, base.Data.Indices[start:start+triVerts]...)
			}
		}
		if len(indices) == 0 {
			continue
		}
		sp := base
		sp.Data.Indices = indices
		sp.MaterialIndex = ir.NoIndex
		if gs.matName != "" {
			for i, m := range mats {
				if m.Name == gs.matName {
					sp.MaterialIndex = i
					break
				}
			}
		}
		result = append(result, sp)
	}
	if len(result) == 0 {
		return []ir.Primitive{base}
	}
	return result
}

func (p *usdaParser) parseSkeletonPrim(defLine string) {
	name := extractQuotedName(defLine)
	skel := &ir.Skeleton{Name: name}

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				p.asset.Skeletons = append(p.asset.Skeletons, skel)
				return
			}
		case strings.Contains(line, usdaSkelJoints):
			joints := parseTokenArray(p.collectArray(line))
			pathToNode := make(map[string]int, len(joints))
			for _, j := range joints {
				jname := j
				if idx := strings.LastIndex(j, "/"); idx >= 0 {
					jname = j[idx+1:]
				}
				jnode := ir.Node{LODGroupIndex: ir.NoIndex,
					Name:        jname,
					IsJoint:     true,
					MeshIndex:   ir.NoIndex,
					SkinIndex:   ir.NoIndex,
					CameraIndex: ir.NoIndex,
					LightIndex:  ir.NoIndex,
				}
				p.asset.Nodes = append(p.asset.Nodes, jnode)
				nodeIdx := len(p.asset.Nodes) - 1
				skel.Joints = append(skel.Joints, nodeIdx)
				pathToNode[j] = nodeIdx
			}
			for _, j := range joints {
				if idx := strings.LastIndex(j, "/"); idx > 0 {
					parentPath := j[:idx]
					if parentIdx, ok := pathToNode[parentPath]; ok {
						childIdx := pathToNode[j]
						p.asset.Nodes[parentIdx].Children = append(p.asset.Nodes[parentIdx].Children, childIdx)
					}
				}
			}
		case strings.Contains(line, usdaSkelBind):
			skel.InverseBindMatrices = parseMatrix4dArray(p.collectArray(line))
		}
	}
}

func (p *usdaParser) parseProceduralPrim(defLine string) {
	name := extractQuotedName(defLine)
	size := float64(cubeHalfDiv)
	radius := float64(1)
	height := float64(cubeHalfDiv)

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				var prim ir.Primitive
				prim.Mode = ir.Triangles
				prim.MaterialIndex = ir.NoIndex

				switch {
				case strings.HasPrefix(defLine, usdaDefCube):
					prim.Data = genCubeMesh(float32(size))
				case strings.HasPrefix(defLine, usdaDefSphere):
					prim.Data = genSphereMesh(float32(radius), proceduralSegs, proceduralRings)
				case strings.HasPrefix(defLine, usdaDefCylinder):
					prim.Data = genCylinderMesh(float32(radius), float32(height), proceduralSegs)
				case strings.HasPrefix(defLine, usdaDefCone):
					prim.Data = genConeMesh(float32(radius), float32(height), proceduralSegs)
				case strings.HasPrefix(defLine, usdaDefCapsule):
					prim.Data = genCapsuleMesh(float32(radius), float32(height), proceduralSegs, proceduralHemiRings)
				}

				if len(prim.Data.Positions) == 0 {
					return
				}
				prim.Data.VertexCount = len(prim.Data.Positions)
				mesh := &ir.Mesh{
					Name:       name,
					Primitives: []ir.Primitive{prim},
				}
				p.asset.Meshes = append(p.asset.Meshes, mesh)
				node := ir.Node{LODGroupIndex: ir.NoIndex,
					Name:      name,
					MeshIndex: len(p.asset.Meshes) - 1,
					SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
				}
				p.asset.Nodes = append(p.asset.Nodes, node)
				p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
				return
			}
		case strings.Contains(line, usdaPrimSize):
			size = float64(parseF32(p.extractValue(line)))
		case strings.Contains(line, usdaPrimRadius):
			radius = float64(parseF32(p.extractValue(line)))
		case strings.Contains(line, usdaPrimHeight):
			height = float64(parseF32(p.extractValue(line)))
		}
	}
}

const (
	proceduralSegs      = 32
	proceduralRings     = 16
	proceduralHemiRings = 8
)

func parseTokenArray(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func genCubeMesh(size float32) ir.MeshData {
	h := size / cubeHalfDiv
	positions := [][3]float32{
		{-h, -h, h}, {h, -h, h}, {h, h, h}, {-h, h, h},
		{-h, -h, -h}, {-h, h, -h}, {h, h, -h}, {h, -h, -h},
		{-h, h, -h}, {-h, h, h}, {h, h, h}, {h, h, -h},
		{-h, -h, -h}, {h, -h, -h}, {h, -h, h}, {-h, -h, h},
		{h, -h, -h}, {h, h, -h}, {h, h, h}, {h, -h, h},
		{-h, -h, -h}, {-h, -h, h}, {-h, h, h}, {-h, h, -h},
	}
	indices := []uint32{
		0, 1, 2, 0, 2, 3,
		4, 5, 6, 4, 6, 7,
		8, 9, 10, 8, 10, 11,
		12, 13, 14, 12, 14, 15,
		16, 17, 18, 16, 18, 19,
		20, 21, 22, 20, 22, 23,
	}
	return ir.MeshData{Positions: positions, Indices: indices}
}

func genSphereMesh(radius float32, segs, rings int) ir.MeshData {
	var positions [][3]float32
	var indices []uint32

	for y := range rings + 1 {
		phi := math.Pi * float64(y) / float64(rings)
		for x := range segs + 1 {
			theta := 2 * math.Pi * float64(x) / float64(segs)
			px := radius * float32(math.Sin(phi)*math.Cos(theta))
			py := radius * float32(math.Cos(phi))
			pz := radius * float32(math.Sin(phi)*math.Sin(theta))
			positions = append(positions, [3]float32{px, py, pz})
		}
	}

	for y := range rings {
		for x := range segs {
			a := uint32(y*(segs+1) + x) //nolint:gosec // small positive loop indices
			b := a + uint32(segs) + 1   //nolint:gosec // small positive loop indices
			indices = append(indices, a, b, a+1, b, b+1, a+1)
		}
	}
	return ir.MeshData{Positions: positions, Indices: indices}
}

func genCylinderMesh(radius, height float32, segs int) ir.MeshData {
	var positions [][3]float32
	var indices []uint32
	halfH := height / cylinderHalfDiv

	for i := range segs + 1 {
		theta := 2 * math.Pi * float64(i) / float64(segs)
		x := radius * float32(math.Cos(theta))
		z := radius * float32(math.Sin(theta))
		positions = append(positions, [3]float32{x, halfH, z}, [3]float32{x, -halfH, z})
	}

	for i := range segs {
		a := uint32(i * cylinderStride) //nolint:gosec // small positive loop index
		indices = append(indices, a, a+1, a+cylinderStride, a+1, a+cylinderIdxShift, a+cylinderStride)
	}

	topC := uint32(len(positions)) //nolint:gosec // positions length fits uint32
	positions = append(positions, [3]float32{0, halfH, 0})
	botC := uint32(len(positions)) //nolint:gosec // positions length fits uint32
	positions = append(positions, [3]float32{0, -halfH, 0})

	for i := range segs {
		a := uint32(i * cylinderStride) //nolint:gosec // small positive loop index
		indices = append(indices, topC, a+cylinderStride, a, botC, a+1, a+cylinderIdxShift)
	}
	return ir.MeshData{Positions: positions, Indices: indices}
}

func genConeMesh(radius, height float32, segs int) ir.MeshData {
	var positions [][3]float32
	var indices []uint32
	halfH := height / coneHalfDiv

	apex := uint32(0)
	positions = append(positions, [3]float32{0, halfH, 0})

	for i := range segs + 1 {
		theta := 2 * math.Pi * float64(i) / float64(segs)
		x := radius * float32(math.Cos(theta))
		z := radius * float32(math.Sin(theta))
		positions = append(positions, [3]float32{x, -halfH, z})
	}

	for i := 1; i <= segs; i++ {
		indices = append(indices, apex, uint32(i), uint32(i+1)) //nolint:gosec // small positive loop index
	}

	botC := uint32(len(positions)) //nolint:gosec // positions length fits uint32
	positions = append(positions, [3]float32{0, -halfH, 0})
	for i := 1; i <= segs; i++ {
		indices = append(indices, botC, uint32(i+1), uint32(i)) //nolint:gosec // small positive loop index
	}
	return ir.MeshData{Positions: positions, Indices: indices}
}

func genCapsuleMesh(radius, height float32, segs, hemiRings int) ir.MeshData {
	cyl := genCylinderMesh(radius, height, segs)
	top := genSphereMesh(radius, segs, hemiRings)
	halfH := height / cylinderHalfDiv

	offset := uint32(len(cyl.Positions)) //nolint:gosec // positions length fits uint32
	for i := range top.Positions {
		top.Positions[i][1] += halfH
	}
	cyl.Positions = append(cyl.Positions, top.Positions...)
	for _, idx := range top.Indices {
		cyl.Indices = append(cyl.Indices, idx+offset)
	}

	bot := genSphereMesh(radius, segs, hemiRings)
	offset = uint32(len(cyl.Positions)) //nolint:gosec // positions length fits uint32
	for i := range bot.Positions {
		bot.Positions[i][1] -= halfH
	}
	cyl.Positions = append(cyl.Positions, bot.Positions...)
	for _, idx := range bot.Indices {
		cyl.Indices = append(cyl.Indices, idx+offset)
	}

	return cyl
}

func flipWindingOrder(indices []uint32) {
	for i := 0; i+2 < len(indices); i += 3 {
		indices[i+1], indices[i+2] = indices[i+2], indices[i+1]
	}
}

func parseFloatArray(s string) []float32 {
	result := make([]float32, 0, strings.Count(s, ",")+1)
	for s != "" {
		idx := strings.IndexByte(s, ',')
		var tok string
		if idx < 0 {
			tok = strings.TrimSpace(s)
			s = ""
		} else {
			tok = strings.TrimSpace(s[:idx])
			s = s[idx+1:]
		}
		if tok == "" {
			continue
		}
		result = append(result, parseF32(tok))
	}
	return result
}

func extractQuotedToken(line string) string {
	_, val, ok := strings.Cut(line, "=")
	if !ok {
		return ""
	}
	v := strings.TrimSpace(val)
	v = strings.Trim(v, "\"")
	return v
}

func mapWrapMode(s string) ir.TextureWrap {
	switch s {
	case wrapClamp:
		return ir.WrapClamp
	case wrapMirror:
		return ir.WrapMirror
	default:
		return ir.WrapRepeat
	}
}

const jointStride = 4

func parseJointIndices(s string) [][4]uint16 {
	raw := parseIntArray(s)
	if len(raw) == 0 {
		return nil
	}
	result := make([][4]uint16, (len(raw)+jointStride-1)/jointStride)
	for i, v := range raw {
		result[i/jointStride][i%jointStride] = uint16(v) //nolint:gosec // joint index fits uint16
	}
	return result
}

func parseJointWeights(s string) [][4]float32 {
	raw := parseFloatArray(s)
	if len(raw) == 0 {
		return nil
	}
	result := make([][4]float32, (len(raw)+jointStride-1)/jointStride)
	for i, v := range raw {
		result[i/jointStride][i%jointStride] = v
	}
	return result
}

func parseMatrix4dArray(s string) [][16]float32 {
	var result [][16]float32
	var m [16]float32
	elem := 0
	for s != "" {
		c := s[0]
		if c == '(' || c == ')' || c == ' ' || c == '\t' {
			s = s[1:]
			continue
		}
		if c == ',' {
			s = s[1:]
			continue
		}
		end := strings.IndexAny(s, ",) ")
		if end < 0 {
			end = len(s)
		}
		m[elem%mat4Elems] = parseF32(s[:end]) //nolint:gosec // elem modulo bounds idx
		elem++
		if elem%mat4Elems == 0 {
			result = append(result, m)
			m = [16]float32{}
		}
		s = s[end:]
	}
	return result
}

func (p *usdaParser) parseBasisCurvesPrim(defLine string) {
	name := extractQuotedName(defLine)
	var positions [][3]float32
	var vertCounts []int

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				if len(positions) == 0 {
					return
				}
				var prim ir.Primitive
				prim.Mode = ir.LineStrip
				prim.MaterialIndex = ir.NoIndex
				prim.Data.Positions = positions
				prim.Data.VertexCount = len(positions)
				if vertCounts != nil {
					prim.Mode = ir.Lines
					prim.Data.Indices = curvesToLineIndices(vertCounts)
				}
				mesh := &ir.Mesh{Name: name, Primitives: []ir.Primitive{prim}}
				p.asset.Meshes = append(p.asset.Meshes, mesh)
				node := ir.Node{LODGroupIndex: ir.NoIndex, Name: name, MeshIndex: len(p.asset.Meshes) - 1,
					SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex}
				p.asset.Nodes = append(p.asset.Nodes, node)
				p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
				return
			}
		case strings.Contains(line, usdaCurvePoints):
			positions = parseVec3Array(p.collectArray(line))
		case strings.Contains(line, usdaCurveVertCounts):
			vertCounts = parseIntArray(p.collectArray(line))
		}
	}
}

func curvesToLineIndices(vertCounts []int) []uint32 {
	var indices []uint32
	offset := uint32(0)
	for _, c := range vertCounts {
		for i := 0; i < c-1; i++ {
			indices = append(indices, offset+uint32(i), offset+uint32(i+1)) //nolint:gosec // bounded
		}
		offset += uint32(c) //nolint:gosec // bounded
	}
	return indices
}

func (p *usdaParser) parsePointsPrim(defLine string) {
	name := extractQuotedName(defLine)
	var positions [][3]float32

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				if len(positions) == 0 {
					return
				}
				var prim ir.Primitive
				prim.Mode = ir.Points
				prim.MaterialIndex = ir.NoIndex
				prim.Data.Positions = positions
				prim.Data.VertexCount = len(positions)
				mesh := &ir.Mesh{Name: name, Primitives: []ir.Primitive{prim}}
				p.asset.Meshes = append(p.asset.Meshes, mesh)
				node := ir.Node{LODGroupIndex: ir.NoIndex, Name: name, MeshIndex: len(p.asset.Meshes) - 1,
					SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex}
				p.asset.Nodes = append(p.asset.Nodes, node)
				p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
				return
			}
		case strings.Contains(line, usdaPropPoints):
			positions = parseVec3Array(p.collectArray(line))
		}
	}
}

func (p *usdaParser) parseDefaultPrim(line string) {
	name := extractQuotedToken(line)
	if name == "" {
		return
	}
	p.asset.Name = name
}

func (p *usdaParser) parseVariantSets() {
	var names []string
	depth := 0
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch { //nolint:staticcheck // depth-conditional } case prevents tagged switch
		case line == "{":
			depth++
		case line == "}":
			if depth <= 0 {
				if len(names) > 0 {
					if p.asset.Metadata.ExtraProperties == nil {
						p.asset.Metadata.ExtraProperties = make(map[string]string)
					}
					p.asset.Metadata.ExtraProperties["variantSets"] = strings.Join(names, ",")
				}
				return
			}
			depth--
		default:
			if depth == 0 {
				n := extractQuotedName(line)
				if n != "" {
					names = append(names, n)
				}
			}
		}
	}
}
