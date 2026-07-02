package abc

import (
	"math"
	"strings"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

func (p *ogawaParser) readCompoundPropertyNames(children []uint64) map[string]int {
	if len(children) == 0 {
		return nil
	}

	result := make(map[string]int)

	if isData(children[0]) {
		return p.readPropertyNamesFromData(children[0], result)
	}
	if isGroup(children[0]) {
		return p.readPropertyNamesFromGroup(children[0], result)
	}
	return result
}

func (p *ogawaParser) readPropertyNamesFromData(child uint64, result map[string]int) map[string]int {
	pos := childAddr(child)
	if pos <= 0 || pos >= len(p.data) || len(p.data)-pos < ogawaU64Size {
		return nil
	}
	rawSize := binread.ReadU64LE(p.data[pos:])
	if rawSize > uint64(len(p.data)) {
		return nil
	}
	size := int(rawSize) //nolint:gosec // bounded by len(p.data) above.
	hdrStart := pos + ogawaU64Size
	if hdrStart+size > len(p.data) || size < i32Size {
		return nil
	}

	offset, propIdx := hdrStart, 0
	for offset+i32Size < hdrStart+size {
		nameLen := int(binread.ReadU32LE(p.data[offset:]))
		offset += i32Size
		if nameLen <= 0 || nameLen > maxPropNameLen || offset+nameLen > hdrStart+size {
			break
		}
		result[string(p.data[offset:offset+nameLen])] = propIdx
		propIdx++
		offset += nameLen
		if offset+1 <= hdrStart+size {
			offset++
		}
	}
	return result
}

func (p *ogawaParser) readPropertyNamesFromGroup(child uint64, result map[string]int) map[string]int {
	subPos := childAddr(child)
	if subPos <= 0 {
		return result
	}
	subs, err := p.loadGroup(subPos)
	if err != nil {
		return result
	}
	for i, sub := range subs {
		if !isData(sub) {
			continue
		}
		if name := p.readPropertyName(sub); name != "" {
			result[name] = i
		}
	}
	return result
}

func (p *ogawaParser) readPropertyName(child uint64) string {
	pos := childAddr(child)
	if pos <= 0 || pos >= len(p.data) || len(p.data)-pos < ogawaU64Size {
		return ""
	}
	rawSize := binread.ReadU64LE(p.data[pos:])
	if rawSize > uint64(len(p.data)) {
		return ""
	}
	size := int(rawSize) //nolint:gosec // bounded by len(p.data) above.
	hdrStart := pos + ogawaU64Size
	if hdrStart+i32Size > len(p.data) || size < i32Size {
		return ""
	}
	nameLen := int(binread.ReadU32LE(p.data[hdrStart:]))
	nameStart := hdrStart + i32Size
	nameEnd := nameStart + nameLen
	if nameLen <= 0 || nameLen > maxPropNameLen || nameEnd > hdrStart+size || nameEnd > len(p.data) {
		return ""
	}
	return string(p.data[nameStart:nameEnd])
}

func (p *ogawaParser) tryExtractXform(offset int) ([][16]float32, bool) {
	children, err := p.loadGroup(offset)
	if err != nil || len(children) == 0 {
		return nil, false
	}

	props := p.readCompoundPropertyNames(children)
	valsIdx, ok := props[abcPropVals]
	if !ok {
		return nil, false
	}

	childIdx := valsIdx + 1
	if childIdx >= len(children) {
		return nil, false
	}

	return p.readXformSamples(children[childIdx])
}

func (p *ogawaParser) readXformSamples(child uint64) ([][16]float32, bool) {
	if isData(child) {
		mat, ok := p.readSingleMatrix(child)
		if !ok {
			return nil, false
		}
		return [][16]float32{mat}, true
	}

	if !isGroup(child) {
		return nil, false
	}
	pos := childAddr(child)
	if pos <= 0 {
		return nil, false
	}
	subs, err := p.loadGroup(pos)
	if err != nil || len(subs) < 2 {
		return nil, false
	}

	var matrices [][16]float32
	for i := 1; i < len(subs); i++ {
		if !isData(subs[i]) {
			continue
		}
		mat, ok := p.readSingleMatrix(subs[i])
		if !ok {
			continue
		}
		matrices = append(matrices, mat)
	}
	if len(matrices) == 0 {
		return nil, false
	}
	return matrices, true
}

func (p *ogawaParser) readSingleMatrix(child uint64) ([16]float32, bool) {
	buf := p.readDataPayload(child)
	if len(buf) < xformByteSize {
		return [16]float32{}, false
	}
	var mat [16]float32
	for i := range xformF64Count {
		mat[i] = float32(math.Float64frombits(binread.ReadU64LE(buf[i*f64Size:])))
	}
	return mat, true
}

func buildXformAnimation(nodeIdx int, matrices [][16]float32, startTime, timePerCycle float64) *ir.Animation {
	n := len(matrices)
	times := make([]float32, n)
	translations := make([][3]float32, n)
	rotations := make([][4]float32, n)
	scales := make([][3]float32, n)

	for i, mat := range matrices {
		times[i] = float32(startTime + float64(i)*timePerCycle)
		t, r, s := mathx.DecomposeTRS(mathx.Mat4(mat))
		translations[i] = t
		rotations[i] = mathx.QuatToArr(r)
		scales[i] = s
	}

	return &ir.Animation{
		Name:     xformAnimName,
		Duration: float64(times[n-1]),
		Channels: []ir.AnimationChannel{
			{
				NodeIndex: nodeIdx, Target: ir.TargetTranslation,
				Interpolation: ir.InterpolationLinear, Times: times, Translations: translations,
			},
			{
				NodeIndex: nodeIdx, Target: ir.TargetRotation,
				Interpolation: ir.InterpolationLinear, Times: times, Rotations: rotations,
			},
			{
				NodeIndex: nodeIdx, Target: ir.TargetScale,
				Interpolation: ir.InterpolationLinear, Times: times, Scales: scales,
			},
		},
	}
}

func (p *ogawaParser) extractCameraFromNamedProps(children []uint64, props map[string]int) {
	focal := p.readF64Property(children, props, abcPropFocalLen)
	if focal <= 0 {
		focal = p.readF64Property(children, props, abcPropFocalLen2)
	}
	if focal <= 0 {
		return
	}
	vertAp := p.readF64Property(children, props, abcPropVertAp)
	if vertAp <= 0 {
		vertAp = p.readF64Property(children, props, abcPropHorizAp)
	}
	near := p.readF64Property(children, props, abcPropNearClip)
	far := p.readF64Property(children, props, abcPropFarClip)

	focalCM := focal * 0.1 //nolint:mnd // focal length mm to cm
	fov := fovFactor * math.Atan(vertAp/(fovFactor*focalCM))

	cam := &ir.Camera{
		Name: "AlembicCamera",
		Perspective: &ir.PerspectiveCamera{
			FOV:  float32(fov),
			Near: float32(near),
			Far:  float32(far),
		},
	}
	camIdx := len(p.asset.Cameras)
	p.asset.Cameras = append(p.asset.Cameras, cam)

	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        cam.Name,
		MeshIndex:   ir.NoIndex,
		SkinIndex:   ir.NoIndex,
		CameraIndex: camIdx,
		LightIndex:  ir.NoIndex,
	}
	p.asset.Nodes = append(p.asset.Nodes, node)
	p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
}

func (p *ogawaParser) readF64Property(children []uint64, props map[string]int, name string) float64 {
	idx, ok := props[name]
	if !ok {
		return 0
	}
	childIdx := idx + 1
	if childIdx >= len(children) {
		return 0
	}
	buf := p.readPropertyData(children[childIdx])
	if len(buf) < f64Size {
		return 0
	}
	return math.Float64frombits(binread.ReadU64LE(buf))
}

func (p *ogawaParser) extractPolyMeshFromNamedProps(children []uint64, props map[string]int) {
	var positions [][3]float32
	var normals [][3]float32
	var uvs [][2]float32
	var colors [][4]float32
	var faceIndices []int32
	var faceCounts []int32

	for name := range props {
		childIdx := props[name] + 1
		if childIdx >= len(children) {
			continue
		}
		child := children[childIdx]
		switch name {
		case abcPropP:
			positions = p.readVec3Array(child)
		case abcPropN:
			normals = p.readVec3Array(child)
		case abcPropFaceIdx:
			faceIndices = p.readInt32Array(child)
		case abcPropFaceCounts:
			faceCounts = p.readInt32Array(child)
		case abcPropUV, abcPropST:
			uvs = p.readVec2Array(child)
		case abcPropCs:
			colors = p.readVec4Array(child)
		}
	}

	if len(positions) < minVertCount {
		return
	}

	indices := triangulateAlembicFaces(faceIndices, faceCounts)
	matIdx := p.tryExtractMaterialRef(children, props)
	p.addMeshNode(positions, normals, uvs, colors, indices)

	if matIdx >= 0 && len(p.asset.Meshes) > 0 {
		for i := range p.asset.Meshes[len(p.asset.Meshes)-1].Primitives {
			p.asset.Meshes[len(p.asset.Meshes)-1].Primitives[i].MaterialIndex = matIdx
		}
	}
}

func (p *ogawaParser) tryExtractMaterialRef(children []uint64, props map[string]int) int {
	arbIdx, ok := props[abcPropArbGeom]
	if !ok {
		return ir.NoIndex
	}
	childIdx := arbIdx + 1
	if childIdx >= len(children) {
		return ir.NoIndex
	}

	buf := p.readPropertyData(children[childIdx])
	name := strings.TrimRight(string(buf), "\x00")
	if name == "" {
		return ir.NoIndex
	}

	matIdx := len(p.asset.Materials)
	p.asset.Materials = append(p.asset.Materials, &ir.Material{
		Name:            name,
		BaseColorFactor: defaultBaseColor,
		RoughnessFactor: 1,
		AlphaMode:       ir.AlphaOpaque,
	})
	return matIdx
}

func readBinaryArray[T any](p *ogawaParser, child uint64, stride int, decode func([]byte) T) []T {
	buf := p.readPropertyData(child)
	if len(buf) < stride {
		return nil
	}
	n := len(buf) / stride
	result := make([]T, n)
	for i := range n {
		result[i] = decode(buf[i*stride:])
	}
	return result
}

func (p *ogawaParser) readVec3Array(child uint64) [][3]float32 {
	return readBinaryArray(p, child, vec3Stride, func(b []byte) [3]float32 {
		return [3]float32{binread.ReadF32LE(b), binread.ReadF32LE(b[f32Size:]), binread.ReadF32LE(b[f32Size*2:])}
	})
}

func (p *ogawaParser) readVec2Array(child uint64) [][2]float32 {
	return readBinaryArray(p, child, vec2Stride, func(b []byte) [2]float32 {
		return [2]float32{binread.ReadF32LE(b), binread.ReadF32LE(b[f32Size:])}
	})
}

func (p *ogawaParser) readVec4Array(child uint64) [][4]float32 {
	return readBinaryArray(p, child, vec4Stride, func(b []byte) [4]float32 {
		return [4]float32{
			binread.ReadF32LE(b),
			binread.ReadF32LE(b[f32Size:]),
			binread.ReadF32LE(b[f32Size*2:]),
			binread.ReadF32LE(b[f32Size*3:]),
		}
	})
}

func (p *ogawaParser) readInt32Array(child uint64) []int32 {
	return readBinaryArray(p, child, i32Size, func(b []byte) int32 {
		return int32(binread.ReadU32LE(b)) //nolint:gosec // Alembic stores int32 payloads as raw little-endian bits.
	})
}

func (p *ogawaParser) readPropertyData(child uint64) []byte {
	if isData(child) {
		return p.readDataPayload(child)
	}
	if !isGroup(child) {
		return nil
	}

	pos := childAddr(child)
	if pos <= 0 {
		return nil
	}
	subs, err := p.loadGroup(pos)
	if err != nil || len(subs) < 2 {
		return nil
	}
	for i := 1; i < len(subs); i++ {
		if isData(subs[i]) {
			return p.readDataPayload(subs[i])
		}
	}
	return nil
}

func (p *ogawaParser) readDataPayload(child uint64) []byte {
	pos := childAddr(child)
	if pos <= 0 || pos >= len(p.data) || len(p.data)-pos < ogawaU64Size {
		return nil
	}
	rawSize := binread.ReadU64LE(p.data[pos:])
	if rawSize > uint64(len(p.data)) {
		return nil
	}
	size := int(rawSize) //nolint:gosec // bounded by len(p.data) above.
	start := pos + ogawaU64Size
	if start+size > len(p.data) || size <= 0 {
		return nil
	}
	return p.data[start : start+size]
}

func triangulateAlembicFaces(faceIndices, faceCounts []int32) []uint32 {
	if len(faceIndices) == 0 {
		return nil
	}
	if len(faceCounts) == 0 {
		out := make([]uint32, 0, len(faceIndices))
		for _, idx := range faceIndices {
			if idx >= 0 {
				out = append(out, uint32(idx))
			}
		}
		return out
	}

	var result []uint32
	idx := 0
	for _, count := range faceCounts {
		c := int(count)
		if c < 3 || idx+c > len(faceIndices) {
			idx += c
			continue
		}
		for j := 2; j < c; j++ {
			result = append(result,
				uint32(faceIndices[idx]),
				uint32(faceIndices[idx+j-1]),
				uint32(faceIndices[idx+j]),
			)
		}
		idx += c
	}
	return result
}
