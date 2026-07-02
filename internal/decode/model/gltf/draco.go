package gltf

import (
	"context"
	"fmt"

	draco "github.com/gophics/go-draco"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/valyala/fastjson"
)

const (
	dracoDecodedNoteKey         = "gltf.draco"
	dracoDecodedNoteValue       = "decoded"
	triangleStripSharedVertices = 2
	maxJointIndex               = int32(^uint16(0))
	maxDracoUniqueID            = uint64(^uint32(0))
)

type dracoSlot uint8

const (
	dracoSlotPosition dracoSlot = iota
	dracoSlotNormal
	dracoSlotTangent
	dracoSlotTex0
	dracoSlotTex1
	dracoSlotTex2
	dracoSlotTex3
	dracoSlotColor0
	dracoSlotJoints0
	dracoSlotJoints1
	dracoSlotWeights0
	dracoSlotWeights1
	dracoSlotCount = int(dracoSlotWeights1) + 1
)

type dracoSemantic struct {
	name      string
	slot      dracoSlot
	elemCount int
}

type dracoAttrRef struct {
	uniqueID uint32
	present  bool
}

type dracoAttributeSet [dracoSlotCount]dracoAttrRef

type dracoPrimitiveExt struct {
	bufferView int
	attrs      dracoAttributeSet
}

var dracoSemantics = [...]dracoSemantic{
	{name: attrPosition, slot: dracoSlotPosition, elemCount: elemVec3},
	{name: attrNormal, slot: dracoSlotNormal, elemCount: elemVec3},
	{name: attrTangent, slot: dracoSlotTangent, elemCount: elemVec4},
	{name: attrTexCoord0, slot: dracoSlotTex0, elemCount: elemVec2},
	{name: attrTexCoord1, slot: dracoSlotTex1, elemCount: elemVec2},
	{name: attrTexCoord2, slot: dracoSlotTex2, elemCount: elemVec2},
	{name: attrTexCoord3, slot: dracoSlotTex3, elemCount: elemVec2},
	{name: attrColor0, slot: dracoSlotColor0, elemCount: elemVec4},
	{name: attrJoints0, slot: dracoSlotJoints0, elemCount: elemVec4},
	{name: attrJoints1, slot: dracoSlotJoints1, elemCount: elemVec4},
	{name: attrWeights0, slot: dracoSlotWeights0, elemCount: elemVec4},
	{name: attrWeights1, slot: dracoSlotWeights1, elemCount: elemVec4},
}

func parseDracoPrimitiveExt(ext *fastjson.Value) (dracoPrimitiveExt, error) {
	bufferViewVal := ext.Get(keyBufferView)
	if bufferViewVal == nil {
		return dracoPrimitiveExt{}, dracoError("missing bufferView", nil)
	}
	if bufferViewVal.Type() != fastjson.TypeNumber {
		return dracoPrimitiveExt{}, dracoError("bufferView must be a number", nil)
	}
	bufferView := bufferViewVal.GetInt()
	if bufferView < 0 {
		return dracoPrimitiveExt{}, dracoError("bufferView must be non-negative", nil)
	}

	attrs := ext.Get(keyAttributes)
	if attrs == nil || attrs.Type() != fastjson.TypeObject {
		return dracoPrimitiveExt{}, dracoError("missing attributes object", nil)
	}

	out := dracoPrimitiveExt{bufferView: bufferView}
	var parseErr error
	attrs.GetObject().Visit(func(key []byte, val *fastjson.Value) {
		if parseErr != nil {
			return
		}
		name := decutil.Bstr(key)
		sem, ok := dracoSemanticByName(name)
		if !ok {
			return
		}
		if val.Type() != fastjson.TypeNumber {
			parseErr = dracoError("attribute "+name+" unique id must be a number", nil)
			return
		}
		id := val.GetInt()
		if id < 0 {
			parseErr = dracoError("attribute "+name+" unique id must be non-negative", nil)
			return
		}
		if uint64(id) > maxDracoUniqueID {
			parseErr = dracoError("attribute "+name+" unique id exceeds uint32", nil)
			return
		}
		out.attrs[sem.slot] = dracoAttrRef{uniqueID: uint32(id), present: true}
	})
	if parseErr != nil {
		return dracoPrimitiveExt{}, parseErr
	}
	return out, nil
}

func dracoSemanticByName(name string) (dracoSemantic, bool) {
	for i := range dracoSemantics {
		sem := dracoSemantics[i]
		if sem.name == name {
			return sem, true
		}
	}
	return dracoSemantic{}, false
}

func (s dracoAttributeSet) has(name string) bool {
	sem, ok := dracoSemanticByName(name)
	return ok && s[sem.slot].present
}

func (d *doc) convertDracoPrimitive(p *fastjson.Value, ext dracoPrimitiveExt) (ir.MeshData, error) {
	src, err := d.bufs.bufferViewSlice(ext.bufferView)
	if err != nil {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, err.Error(), err)
	}
	if len(src) == 0 {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, "empty compressed payload", nil)
	}

	mesh, err := d.decodeDracoMesh(src)
	if err != nil {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, "decode failed", err)
	}
	pointCount := mesh.PointCount()
	if pointCount <= 0 {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, "mesh has no points", nil)
	}
	if d.opts.MaxVertices > 0 && pointCount > d.opts.MaxVertices {
		return ir.MeshData{}, decutil.DecodeErr(ir.FormatGLTF, "vertex limit exceeded", nil)
	}

	data, err := d.readAttributesFiltered(p, ext.attrs)
	if err != nil {
		return ir.MeshData{}, err
	}
	if err := d.readDracoAttributes(p, mesh, ext.attrs, pointCount, &data); err != nil {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, err.Error(), err)
	}
	if err := validateDracoVertexCount(data, pointCount); err != nil {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, err.Error(), nil)
	}
	if err := validateDracoAttributeCounts(data); err != nil {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, err.Error(), nil)
	}

	data.Indices, err = readDracoIndices(mesh, pointCount)
	if err != nil {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, err.Error(), err)
	}
	if err := d.validateDracoIndicesAccessor(p, len(data.Indices)); err != nil {
		return ir.MeshData{}, dracoBufferViewError(ext.bufferView, err.Error(), nil)
	}

	d.reportDracoDecoded()
	return data, nil
}

func (d *doc) readDracoAttributes(
	p *fastjson.Value,
	mesh *draco.Mesh,
	attrs dracoAttributeSet,
	pointCount int,
	data *ir.MeshData,
) error {
	var f32Scratch []float32
	var i32Scratch []int32

	for i := range dracoSemantics {
		sem := dracoSemantics[i]
		ref := attrs[sem.slot]
		if !ref.present {
			continue
		}

		var err error
		f32Scratch, i32Scratch, err = readDracoSemantic(mesh, sem, ref, pointCount, data, f32Scratch, i32Scratch)
		if err != nil {
			return err
		}
	}
	return d.validateDracoAttributeAccessors(p, attrs, pointCount)
}

func readDracoSemantic(
	mesh *draco.Mesh,
	sem dracoSemantic,
	ref dracoAttrRef,
	pointCount int,
	data *ir.MeshData,
	f32Scratch []float32,
	i32Scratch []int32,
) (f32Out []float32, i32Out []int32, err error) {
	switch sem.slot {
	case dracoSlotColor0:
		colors, scratch, err := readDracoColor(mesh, ref, pointCount, f32Scratch)
		data.Colors0 = colors
		return scratch, i32Scratch, err
	case dracoSlotJoints0, dracoSlotJoints1:
		joints, scratch, err := readDracoJoints(mesh, ref, sem.name, pointCount, i32Scratch)
		if sem.slot == dracoSlotJoints0 {
			data.Joints0 = joints
		} else {
			data.Joints1 = joints
		}
		return f32Scratch, scratch, err
	default:
		flat, err := appendDracoFloat32(mesh, ref, sem, pointCount, f32Scratch)
		if err != nil {
			return f32Scratch, i32Scratch, err
		}
		assignDracoFloatAttribute(data, sem.slot, pointCount, flat)
		return flat[:0], i32Scratch, nil
	}
}

func assignDracoFloatAttribute(data *ir.MeshData, slot dracoSlot, pointCount int, flat []float32) {
	switch slot {
	case dracoSlotPosition:
		data.Positions = make([][3]float32, pointCount)
		packVec3(data.Positions, flat)
		data.VertexCount = len(data.Positions)
	case dracoSlotNormal:
		data.Normals = make([][3]float32, pointCount)
		packVec3(data.Normals, flat)
	case dracoSlotTangent:
		data.Tangents = make([][4]float32, pointCount)
		packVec4(data.Tangents, flat)
	case dracoSlotTex0:
		data.TexCoord0 = make([][2]float32, pointCount)
		packVec2(data.TexCoord0, flat)
	case dracoSlotTex1:
		data.TexCoord1 = make([][2]float32, pointCount)
		packVec2(data.TexCoord1, flat)
	case dracoSlotTex2:
		data.TexCoord2 = make([][2]float32, pointCount)
		packVec2(data.TexCoord2, flat)
	case dracoSlotTex3:
		data.TexCoord3 = make([][2]float32, pointCount)
		packVec2(data.TexCoord3, flat)
	case dracoSlotWeights0:
		data.Weights0 = make([][4]float32, pointCount)
		packVec4(data.Weights0, flat)
	case dracoSlotWeights1:
		data.Weights1 = make([][4]float32, pointCount)
		packVec4(data.Weights1, flat)
	}
}

func packVec2(dst [][2]float32, flat []float32) {
	for i := range dst {
		base := i * elemVec2
		dst[i] = [2]float32{flat[base], flat[base+1]}
	}
}

func packVec3(dst [][3]float32, flat []float32) {
	for i := range dst {
		base := i * elemVec3
		dst[i] = [3]float32{flat[base], flat[base+1], flat[base+2]}
	}
}

func packVec4(dst [][4]float32, flat []float32) {
	for i := range dst {
		base := i * elemVec4
		dst[i] = [4]float32{flat[base], flat[base+1], flat[base+2], flat[base+3]}
	}
}

func (d *doc) decodeDracoMesh(src []byte) (*draco.Mesh, error) {
	if d.dracoDec == nil {
		dec, err := draco.NewDecoder()
		if err != nil {
			return nil, err
		}
		d.dracoDec = dec
	}
	ctx := d.opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return d.dracoDec.DecodeMesh(ctx, src)
}

func readDracoColor(
	mesh *draco.Mesh,
	ref dracoAttrRef,
	pointCount int,
	scratch []float32,
) (colors [][4]float32, scratchOut []float32, err error) {
	attID, err := dracoMappedAttributeID(mesh, ref, attrColor0)
	if err != nil {
		return nil, scratch, err
	}
	scratch = ensureFloatScratch(scratch, pointCount*elemVec4)
	flat, err := mesh.AppendMappedFloat32(attID, scratch)
	if err != nil {
		return nil, scratch, err
	}
	if len(flat) != pointCount*elemVec3 && len(flat) != pointCount*elemVec4 {
		return nil, flat[:0], fmt.Errorf("%s has %d components per point", attrColor0, len(flat)/pointCount)
	}

	components := len(flat) / pointCount
	out := make([][4]float32, pointCount)
	for i := range pointCount {
		base := i * components
		out[i][0] = flat[base]
		out[i][1] = flat[base+1]
		out[i][2] = flat[base+2]
		if components == elemVec4 {
			out[i][3] = flat[base+3]
		} else {
			out[i][3] = 1
		}
	}
	return out, flat[:0], nil
}

func readDracoJoints(
	mesh *draco.Mesh,
	ref dracoAttrRef,
	semantic string,
	pointCount int,
	scratch []int32,
) (joints [][4]uint16, scratchOut []int32, err error) {
	attID, err := dracoMappedAttributeID(mesh, ref, semantic)
	if err != nil {
		return nil, scratch, err
	}
	scratch = ensureIntScratch(scratch, pointCount*elemVec4)
	flat, err := mesh.AppendMappedInt32(attID, scratch)
	if err != nil {
		return nil, scratch, err
	}
	if len(flat) != pointCount*elemVec4 {
		return nil, flat[:0], fmt.Errorf("%s has %d components per point", semantic, len(flat)/pointCount)
	}

	out := make([][4]uint16, pointCount)
	for i := range pointCount {
		base := i * elemVec4
		for j := range elemVec4 {
			value := flat[base+j]
			if value < 0 || value > maxJointIndex {
				return nil, flat[:0], fmt.Errorf("%s joint index %d out of uint16 range", semantic, value)
			}
			out[i][j] = uint16(value)
		}
	}
	return out, flat[:0], nil
}

func appendDracoFloat32(
	mesh *draco.Mesh,
	ref dracoAttrRef,
	sem dracoSemantic,
	pointCount int,
	scratch []float32,
) ([]float32, error) {
	attID, err := dracoMappedAttributeID(mesh, ref, sem.name)
	if err != nil {
		return nil, err
	}
	scratch = ensureFloatScratch(scratch, pointCount*sem.elemCount)
	flat, err := mesh.AppendMappedFloat32(attID, scratch)
	if err != nil {
		return nil, err
	}
	if len(flat) != pointCount*sem.elemCount {
		return nil, fmt.Errorf("%s has %d components per point", sem.name, len(flat)/pointCount)
	}
	return flat, nil
}

func dracoMappedAttributeID(mesh *draco.Mesh, ref dracoAttrRef, semantic string) (int, error) {
	attID := mesh.AttributeIDByUniqueID(ref.uniqueID)
	if attID < 0 {
		return -1, fmt.Errorf("%s unique id %d not found in Draco mesh", semantic, ref.uniqueID)
	}
	return attID, nil
}

func ensureFloatScratch(scratch []float32, count int) []float32 {
	if cap(scratch) < count {
		return make([]float32, 0, count)
	}
	return scratch[:0]
}

func ensureIntScratch(scratch []int32, count int) []int32 {
	if cap(scratch) < count {
		return make([]int32, 0, count)
	}
	return scratch[:0]
}

func readDracoIndices(mesh *draco.Mesh, pointCount int) ([]uint32, error) {
	faceCount := mesh.FaceCount()
	if faceCount <= 0 {
		return nil, fmt.Errorf("mesh has no faces")
	}
	if faceCount > maxInt()/elemVec3 {
		return nil, fmt.Errorf("decoded face count %d overflows index count", faceCount)
	}
	indexCount := faceCount * elemVec3
	indices := make([]uint32, indexCount)
	for i := range faceCount {
		face, err := mesh.Face(i)
		if err != nil {
			return nil, err
		}
		base := i * elemVec3
		for j := range elemVec3 {
			index := face[j]
			if int64(index) >= int64(pointCount) {
				return nil, fmt.Errorf("face %d index %d out of range", i, index)
			}
			indices[base+j] = index
		}
	}
	return indices, nil
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func validateDracoVertexCount(data ir.MeshData, pointCount int) error {
	if len(data.Positions) == 0 {
		return fmt.Errorf("POSITION attribute is required")
	}
	if data.VertexCount == 0 {
		return fmt.Errorf("vertex count is missing")
	}
	if data.VertexCount != pointCount {
		return fmt.Errorf("vertex count %d does not match Draco point count %d", data.VertexCount, pointCount)
	}
	return nil
}

func validateDracoAttributeCounts(data ir.MeshData) error {
	count := data.VertexCount
	for i := range dracoSemantics {
		sem := dracoSemantics[i]
		attrCount := dracoAttributeCount(data, sem.slot)
		if attrCount != 0 && attrCount != count {
			return fmt.Errorf("%s count %d does not match vertex count %d", sem.name, attrCount, count)
		}
	}
	return nil
}

func dracoAttributeCount(data ir.MeshData, slot dracoSlot) int {
	switch slot {
	case dracoSlotPosition:
		return len(data.Positions)
	case dracoSlotNormal:
		return len(data.Normals)
	case dracoSlotTangent:
		return len(data.Tangents)
	case dracoSlotTex0:
		return len(data.TexCoord0)
	case dracoSlotTex1:
		return len(data.TexCoord1)
	case dracoSlotTex2:
		return len(data.TexCoord2)
	case dracoSlotTex3:
		return len(data.TexCoord3)
	case dracoSlotColor0:
		return len(data.Colors0)
	case dracoSlotJoints0:
		return len(data.Joints0)
	case dracoSlotJoints1:
		return len(data.Joints1)
	case dracoSlotWeights0:
		return len(data.Weights0)
	case dracoSlotWeights1:
		return len(data.Weights1)
	default:
		return 0
	}
}

func (d *doc) validateDracoAttributeAccessors(p *fastjson.Value, attrs dracoAttributeSet, pointCount int) error {
	for i := range dracoSemantics {
		sem := dracoSemantics[i]
		if !attrs[sem.slot].present {
			continue
		}
		if err := d.validateDracoAccessor(p, sem, pointCount); err != nil {
			return err
		}
	}
	return nil
}

func (d *doc) validateDracoAccessor(p *fastjson.Value, sem dracoSemantic, count int) error {
	attrVal := p.Get(keyAttributes, sem.name)
	if attrVal == nil {
		return nil
	}
	a := d.getAccessor(attrVal.GetInt())
	if a.count != count {
		return fmt.Errorf("%s accessor count %d does not match Draco point count %d", sem.name, a.count, count)
	}
	allowVec3 := sem.slot == dracoSlotColor0
	if a.elemCount != sem.elemCount && (!allowVec3 || a.elemCount != elemVec3) {
		return fmt.Errorf("%s accessor has %d components, expected %d", sem.name, a.elemCount, sem.elemCount)
	}
	if componentSize(a.componentType) == 0 {
		return fmt.Errorf("%s accessor has invalid componentType %d", sem.name, a.componentType)
	}
	return nil
}

func (d *doc) validateDracoIndicesAccessor(p *fastjson.Value, indexCount int) error {
	idxVal := p.Get(keyIndices)
	if idxVal == nil {
		return nil
	}
	a := d.getAccessor(idxVal.GetInt())
	expectedCount, err := expectedDracoDecodedIndexCount(p, a.count)
	if err != nil {
		return err
	}
	if expectedCount != indexCount {
		return fmt.Errorf("indices accessor count %d does not match decoded index count %d", a.count, indexCount)
	}
	if a.elemCount != 1 {
		return fmt.Errorf("indices accessor must be SCALAR")
	}
	if componentSize(a.componentType) == 0 {
		return fmt.Errorf("indices accessor has invalid componentType %d", a.componentType)
	}
	return nil
}

func expectedDracoDecodedIndexCount(p *fastjson.Value, accessorCount int) (int, error) {
	modeVal := p.Get(keyMode)
	if modeVal == nil || modeVal.GetInt() == modeTriangles {
		return accessorCount, nil
	}
	if modeVal.GetInt() != modeTriangleStrip {
		return accessorCount, nil
	}
	if accessorCount < elemVec3 {
		return 0, fmt.Errorf("triangle strip indices accessor count %d must be at least 3", accessorCount)
	}
	triangleCount := accessorCount - triangleStripSharedVertices
	if triangleCount > maxInt()/elemVec3 {
		return 0, fmt.Errorf("triangle strip indices accessor count %d overflows decoded index count", accessorCount)
	}
	return triangleCount * elemVec3, nil
}

func validateDracoPrimitiveMode(p *fastjson.Value) error {
	modeVal := p.Get(keyMode)
	if modeVal == nil {
		return nil
	}
	switch modeVal.GetInt() {
	case modeTriangles, modeTriangleStrip:
		return nil
	default:
		return dracoError(fmt.Sprintf("unsupported primitive mode %d", modeVal.GetInt()), nil)
	}
}

func (bs *bufferSet) bufferViewSlice(index int) ([]byte, error) {
	if index < 0 || index >= len(bs.views) {
		return nil, fmt.Errorf("bufferView index %d out of bounds", index)
	}
	view := bs.views[index]
	return bs.slice(view.buffer, view.byteOffset, view.byteLength)
}

func (d *doc) reportDracoDecoded() {
	if d.dracoReported || d.opts.Reporter == nil {
		return
	}
	d.opts.Reporter.AddProvenanceNote(dracoDecodedNoteKey, dracoDecodedNoteValue)
	d.dracoReported = true
}

func dracoError(detail string, err error) error {
	return decutil.DecodeErr(ir.FormatID(gltfName), extDraco+": "+detail, err)
}

func dracoBufferViewError(index int, detail string, err error) error {
	return dracoError(fmt.Sprintf("bufferView %d: %s", index, detail), err)
}
