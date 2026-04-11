package usda

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildFakeCrate constructs a crateReader with the given tokens and a data
// buffer large enough for inline reads. Fields token indices reference
// cr.tokens by position.
func buildFakeCrate(tokens []string) *crateReader {
	return &crateReader{
		data:   make([]byte, 4096),
		tokens: tokens,
	}
}

// makeInlineFloat encodes a float32 as an inline value representation.
func makeInlineFloat(v float32) uint64 {
	return uint64(math.Float32bits(v)) << valueRepPayShift
}

// makeInlineToken encodes a token index as an inline value representation.
func makeInlineToken(idx int32) uint64 {
	return uint64(idx) << valueRepPayShift
}

// makeInlineBool encodes a bool as an inline value representation.
func makeInlineBoolVR(v bool) uint64 {
	if v {
		return 1 << valueRepPayShift
	}
	return 0
}

func makeInlineStringVR(idx int32) uint64 {
	return uint64(idx) << valueRepPayShift
}

func field(cr *crateReader, name string, vr uint64) crateField {
	for i, t := range cr.tokens {
		if t == name {
			return crateField{tokenIdx: int32(i), valueRep: vr}
		}
	}
	idx := int32(len(cr.tokens))
	cr.tokens = append(cr.tokens, name)
	return crateField{tokenIdx: idx, valueRep: vr}
}

// writeVec3fVR writes N vec3f values at offset and returns a valueRep.
func writeVec3fVR(cr *crateReader, off int, vecs [][3]float32) uint64 {
	binary.LittleEndian.PutUint64(cr.data[off:], uint64(len(vecs)))
	for i, v := range vecs {
		base := off + 8 + i*12
		binary.LittleEndian.PutUint32(cr.data[base:], math.Float32bits(v[0]))
		binary.LittleEndian.PutUint32(cr.data[base+4:], math.Float32bits(v[1]))
		binary.LittleEndian.PutUint32(cr.data[base+8:], math.Float32bits(v[2]))
	}
	return uint64(off) << valueRepPayShift
}

// writeTokenArrayVR writes token indices at offset and returns a valueRep.
func writeTokenArrayVR(cr *crateReader, off int, indices []int32) uint64 {
	binary.LittleEndian.PutUint64(cr.data[off:], uint64(len(indices)))
	for i, idx := range indices {
		binary.LittleEndian.PutUint32(cr.data[off+8+i*4:], uint32(idx))
	}
	return uint64(off) << valueRepPayShift
}

// writeIntArrayVR writes int32 values at offset and returns a valueRep.
func writeIntArrayVR(cr *crateReader, off int, vals []int32) uint64 {
	binary.LittleEndian.PutUint64(cr.data[off:], uint64(len(vals)))
	for i, v := range vals {
		binary.LittleEndian.PutUint32(cr.data[off+8+i*4:], uint32(v))
	}
	return uint64(off) << valueRepPayShift
}

func writeTimeSampledVec3VR(cr *crateReader, off int, times []float64, frames [][][3]float32) uint64 {
	binary.LittleEndian.PutUint64(cr.data[off:], uint64(len(times)))
	entryOff := off + 8
	frameOff := entryOff + len(times)*16
	for i := range times {
		binary.LittleEndian.PutUint64(cr.data[entryOff+i*16:], math.Float64bits(times[i]))
		frameVR := writeVec3fVR(cr, frameOff, frames[i])
		binary.LittleEndian.PutUint64(cr.data[entryOff+i*16+8:], frameVR)
		frameOff += 8 + len(frames[i])*12
	}
	return uint64(off) << valueRepPayShift
}

func TestConvertCrateScope(t *testing.T) {
	asset := &ir.Asset{}
	convertCrateScope("MyScope", asset)

	require.Len(t, asset.Nodes, 1)
	assert.Equal(t, "MyScope", asset.Nodes[0].Name)
	assert.Equal(t, ir.NoIndex, asset.Nodes[0].MeshIndex)
	require.Len(t, asset.RootNodes, 1)
}

func TestConvertCrateSkeleton(t *testing.T) {
	cr := buildFakeCrate([]string{tokJoints, tokBindXforms, "Root", "Root/Arm", "Root/Arm/Hand"})
	asset := &ir.Asset{}

	tokVR := writeTokenArrayVR(cr, 100, []int32{2, 3, 4}) // "Root", "Root/Arm", "Root/Arm/Hand"

	fields := []crateField{
		field(cr, tokJoints, tokVR),
	}

	convertCrateSkeleton(cr, fields, "TestSkel", asset)

	require.Len(t, asset.Skeletons, 1)
	skel := asset.Skeletons[0]
	assert.Equal(t, "TestSkel", skel.Name)
	require.Len(t, skel.Joints, 3)

	assert.Equal(t, "Root", asset.Nodes[skel.Joints[0]].Name)
	assert.Equal(t, "Arm", asset.Nodes[skel.Joints[1]].Name)
	assert.Equal(t, "Hand", asset.Nodes[skel.Joints[2]].Name)
	assert.True(t, asset.Nodes[skel.Joints[0]].IsJoint)

	// Check parent-child wiring
	assert.Contains(t, asset.Nodes[skel.Joints[0]].Children, skel.Joints[1])
	assert.Contains(t, asset.Nodes[skel.Joints[1]].Children, skel.Joints[2])
}

func TestConvertCrateSkelAnim(t *testing.T) {
	cr := buildFakeCrate([]string{tokJoints, tokAnimTranslations, tokAnimRotations, "Root", "Root/Arm"})
	// buildSkelAnim needs matching joint nodes in scene
	asset := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "Root", IsJoint: true, MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
			{Name: "Arm", IsJoint: true, MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
	}

	tokVR := writeTokenArrayVR(cr, 100, []int32{3, 4})
	transVR := writeVec3fVR(cr, 200, [][3]float32{{0, 0, 0}, {0, 1, 0}})

	// Write quaternion array (4 floats per quat)
	off := 400
	binary.LittleEndian.PutUint64(cr.data[off:], 2) // count=2
	for i := range 2 {
		base := off + 8 + i*16
		binary.LittleEndian.PutUint32(cr.data[base:], math.Float32bits(1))    // w
		binary.LittleEndian.PutUint32(cr.data[base+4:], math.Float32bits(0))  // x
		binary.LittleEndian.PutUint32(cr.data[base+8:], math.Float32bits(0))  // y
		binary.LittleEndian.PutUint32(cr.data[base+12:], math.Float32bits(0)) // z
	}
	rotVR := uint64(off) << valueRepPayShift

	fields := []crateField{
		field(cr, tokJoints, tokVR),
		field(cr, tokAnimTranslations, transVR),
		field(cr, tokAnimRotations, rotVR),
	}

	convertCrateSkelAnim(cr, fields, "WalkAnim", asset)

	require.Len(t, asset.Animations, 1)
	assert.Equal(t, "WalkAnim", asset.Animations[0].Name)
}

func TestConvertCrateMaterial(t *testing.T) {
	cr := buildFakeCrate([]string{tokDiffuseColor, tokMetallic, tokRoughness, tokDoubleSided, tokClearcoat, tokIOR, tokOpacityThr})
	asset := &ir.Asset{}

	fields := []crateField{
		field(cr, tokMetallic, makeInlineFloat(0.9)),
		field(cr, tokRoughness, makeInlineFloat(0.3)),
		field(cr, tokClearcoat, makeInlineFloat(0.5)),
		field(cr, tokIOR, makeInlineFloat(1.5)),
		field(cr, tokOpacityThr, makeInlineFloat(0.5)),
	}

	convertCrateMaterial(cr, fields, "Chrome", asset)

	require.Len(t, asset.Materials, 1)
	mat := asset.Materials[0]
	assert.Equal(t, "Chrome", mat.Name)
	assert.InDelta(t, float32(0.9), mat.MetallicFactor, 0.01)
	assert.InDelta(t, float32(0.3), mat.RoughnessFactor, 0.01)
	assert.Equal(t, ir.AlphaMask, mat.AlphaMode)
	require.NotNil(t, mat.Properties)
	assert.InDelta(t, float32(0.5), mat.Properties[propClearcoat], 0.01)
	assert.InDelta(t, float32(1.5), mat.Properties[propIOR], 0.01)
}

func TestConvertCrateBasisCurves(t *testing.T) {
	cr := buildFakeCrate([]string{tokPoints, tokCurveVertCounts})
	asset := &ir.Asset{}

	ptsVR := writeVec3fVR(cr, 100, [][3]float32{{0, 0, 0}, {1, 1, 0}, {2, 0, 0}})
	countsVR := writeIntArrayVR(cr, 300, []int32{3})

	fields := []crateField{
		field(cr, tokPoints, ptsVR),
		field(cr, tokCurveVertCounts, countsVR),
	}

	convertCrateBasisCurves(cr, fields, "Hair", asset)

	require.Len(t, asset.Meshes, 1)
	assert.Equal(t, "Hair", asset.Meshes[0].Name)
	assert.Equal(t, ir.Lines, asset.Meshes[0].Primitives[0].Mode)
	assert.Len(t, asset.Meshes[0].Primitives[0].Data.Positions, 3)
	require.Len(t, asset.Nodes, 1)
}

func TestConvertCratePoints(t *testing.T) {
	cr := buildFakeCrate([]string{tokPoints})
	asset := &ir.Asset{}

	ptsVR := writeVec3fVR(cr, 100, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}})
	fields := []crateField{
		field(cr, tokPoints, ptsVR),
	}

	convertCratePoints(cr, fields, "Cloud", asset)

	require.Len(t, asset.Meshes, 1)
	assert.Equal(t, "Cloud", asset.Meshes[0].Name)
	assert.Equal(t, ir.Points, asset.Meshes[0].Primitives[0].Mode)
	assert.Len(t, asset.Meshes[0].Primitives[0].Data.Positions, 3)
	require.Len(t, asset.Nodes, 1)
}

func TestConvertCrateMesh_Skinned(t *testing.T) {
	cr := buildFakeCrate([]string{
		tokPoints, tokFaceIdx, tokFaceCounts, tokNormals, tokST,
		tokDisplayColor, tokDisplayOpacity, tokOrientation,
		tokDoubleSided, tokJointIndices, tokJointWeights,
		tokMatBinding, tokSkelBinding,
	})
	asset := &ir.Asset{
		Skeletons: []*ir.Skeleton{{Name: "MySkel"}},
		Materials: []*ir.Material{{Name: "MyMat"}},
	}

	ptsVR := writeVec3fVR(cr, 100, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}})

	// Write face vertex indices
	idxOff := 300
	binary.LittleEndian.PutUint64(cr.data[idxOff:], 3)
	for i, v := range []uint32{0, 1, 2} {
		binary.LittleEndian.PutUint32(cr.data[idxOff+8+i*4:], v)
	}
	idxVR := uint64(idxOff) << valueRepPayShift

	// Write joint indices (4 per vertex * 3 verts = 12 ints)
	jiOff := 500
	binary.LittleEndian.PutUint64(cr.data[jiOff:], 12)
	jointVals := []int32{0, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0}
	for i, v := range jointVals {
		binary.LittleEndian.PutUint32(cr.data[jiOff+8+i*4:], uint32(v))
	}
	jiVR := uint64(jiOff) << valueRepPayShift

	// Write joint weights (4 per vertex * 3 verts = 12 floats)
	jwOff := 700
	binary.LittleEndian.PutUint64(cr.data[jwOff:], 12)
	weightVals := []float32{1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}
	for i, v := range weightVals {
		binary.LittleEndian.PutUint32(cr.data[jwOff+8+i*4:], math.Float32bits(v))
	}
	jwVR := uint64(jwOff) << valueRepPayShift

	fields := []crateField{
		field(cr, tokPoints, ptsVR),
		field(cr, tokFaceIdx, idxVR),
		field(cr, tokDoubleSided, makeInlineBoolVR(true)),
		field(cr, tokJointIndices, jiVR),
		field(cr, tokJointWeights, jwVR),
		field(cr, tokMatBinding, makeInlineToken(int32(len(cr.tokens)))),
		field(cr, tokSkelBinding, makeInlineToken(int32(len(cr.tokens)+1))),
	}
	cr.tokens = append(cr.tokens, "MyMat", "MySkel")

	convertCrateMesh(cr, fields, "SkinnedMesh", asset)

	require.Len(t, asset.Meshes, 1)
	assert.Equal(t, "SkinnedMesh", asset.Meshes[0].Name)
	prim := asset.Meshes[0].Primitives[0]
	assert.Len(t, prim.Data.Positions, 3)
	assert.Len(t, prim.Data.Joints0, 3)
	assert.Len(t, prim.Data.Weights0, 3)

	require.Len(t, asset.Nodes, 1)
	assert.Equal(t, 0, asset.Nodes[0].MeshIndex)
	assert.Equal(t, 0, asset.Nodes[0].SkinIndex)
}

// writeVec3dVR writes N vec3d values (float64) at offset and returns a valueRep.
func writeVec3dVR(cr *crateReader, off int, vecs [][3]float64) uint64 {
	binary.LittleEndian.PutUint64(cr.data[off:], uint64(len(vecs)))
	for i, v := range vecs {
		base := off + 8 + i*24
		binary.LittleEndian.PutUint64(cr.data[base:], math.Float64bits(v[0]))
		binary.LittleEndian.PutUint64(cr.data[base+8:], math.Float64bits(v[1]))
		binary.LittleEndian.PutUint64(cr.data[base+16:], math.Float64bits(v[2]))
	}
	return uint64(off) << valueRepPayShift
}

func TestConvertCrateXform(t *testing.T) {
	cr := &crateReader{data: make([]byte, 4096), tokens: []string{tokTranslate, tokXformScale, tokTransform}}
	asset := &ir.Asset{}

	// convertCrateXform tries readVec3dArray first, so write vec3d data
	transVR := writeVec3dVR(cr, 100, [][3]float64{{5, 10, 15}})
	scaleVR := writeVec3dVR(cr, 200, [][3]float64{{0.5, 0.5, 0.5}})

	fields := []crateField{
		field(cr, tokTranslate, transVR),
		field(cr, tokXformScale, scaleVR),
	}

	convertCrateXform(cr, fields, "Mover", asset)

	require.Len(t, asset.Nodes, 1)
	assert.Equal(t, "Mover", asset.Nodes[0].Name)
	assert.Equal(t, [3]float32{5, 10, 15}, asset.Nodes[0].Transform.Translation)
	assert.Equal(t, [3]float32{0.5, 0.5, 0.5}, asset.Nodes[0].Transform.Scale)
}

func TestConvertCrateProceduralPrim(t *testing.T) {
	types := []string{tokCube, tokSphere, tokCylinder, tokCone, tokCapsule}
	for _, tt := range types {
		t.Run(tt, func(t *testing.T) {
			cr := buildFakeCrate([]string{tokSize, tokRadius, tokHeight})
			asset := &ir.Asset{}
			fields := []crateField{
				field(cr, tokSize, makeInlineFloat(2)),
				field(cr, tokRadius, makeInlineFloat(1)),
				field(cr, tokHeight, makeInlineFloat(3)),
			}
			convertCrateProceduralPrim(cr, fields, "Prim_"+tt, tt, asset)
			require.Len(t, asset.Meshes, 1)
			assert.True(t, len(asset.Meshes[0].Primitives[0].Data.Positions) > 0)
			require.Len(t, asset.Nodes, 1)
		})
	}
}

func TestConvertCrateMesh_DisplayColorOpacity(t *testing.T) {
	cr := buildFakeCrate([]string{tokPoints, tokFaceIdx, tokDisplayColor, tokDisplayOpacity, tokOrientation, tokFaceCounts})
	asset := &ir.Asset{}

	ptsVR := writeVec3fVR(cr, 100, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}})

	idxOff := 300
	binary.LittleEndian.PutUint64(cr.data[idxOff:], 4)
	for i, v := range []uint32{0, 1, 2, 3} {
		binary.LittleEndian.PutUint32(cr.data[idxOff+8+i*4:], v)
	}
	idxVR := uint64(idxOff) << valueRepPayShift

	colorsVR := writeVec3fVR(cr, 500, [][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 1, 0}})

	opOff := 700
	binary.LittleEndian.PutUint64(cr.data[opOff:], 4)
	for i, v := range []float32{0.5, 0.5, 0.5, 0.5} {
		binary.LittleEndian.PutUint32(cr.data[opOff+8+i*4:], math.Float32bits(v))
	}
	opVR := uint64(opOff) << valueRepPayShift

	faceCountsVR := writeIntArrayVR(cr, 900, []int32{4})

	fields := []crateField{
		field(cr, tokPoints, ptsVR),
		field(cr, tokFaceIdx, idxVR),
		field(cr, tokDisplayColor, colorsVR),
		field(cr, tokDisplayOpacity, opVR),
		field(cr, tokOrientation, makeInlineToken(int32(len(cr.tokens)))),
		field(cr, tokFaceCounts, faceCountsVR),
	}
	cr.tokens = append(cr.tokens, usdaOrientLeft)

	convertCrateMesh(cr, fields, "ColoredQuad", asset)

	require.Len(t, asset.Meshes, 1)
	prim := asset.Meshes[0].Primitives[0]
	assert.Len(t, prim.Data.Colors0, 4)
	assert.InDelta(t, float32(0.5), prim.Data.Colors0[0][3], 0.01) // opacity merged
	assert.True(t, len(prim.Data.Indices) > 0)                     // triangulated
}

func TestConvertCrateMesh_OpacityOnly(t *testing.T) {
	cr := buildFakeCrate([]string{tokPoints, tokFaceIdx, tokDisplayOpacity})
	asset := &ir.Asset{}

	ptsVR := writeVec3fVR(cr, 100, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}})

	idxOff := 300
	binary.LittleEndian.PutUint64(cr.data[idxOff:], 3)
	for i, v := range []uint32{0, 1, 2} {
		binary.LittleEndian.PutUint32(cr.data[idxOff+8+i*4:], v)
	}
	idxVR := uint64(idxOff) << valueRepPayShift

	opOff := 700
	binary.LittleEndian.PutUint64(cr.data[opOff:], 3)
	for i, v := range []float32{0.8, 0.8, 0.8} {
		binary.LittleEndian.PutUint32(cr.data[opOff+8+i*4:], math.Float32bits(v))
	}
	opVR := uint64(opOff) << valueRepPayShift

	fields := []crateField{
		field(cr, tokPoints, ptsVR),
		field(cr, tokFaceIdx, idxVR),
		field(cr, tokDisplayOpacity, opVR),
	}

	convertCrateMesh(cr, fields, "OpQuad", asset)

	require.Len(t, asset.Meshes, 1)
	prim := asset.Meshes[0].Primitives[0]
	require.Len(t, prim.Data.Colors0, 3)
	assert.InDelta(t, float32(1), prim.Data.Colors0[0][0], 0.01)   // white
	assert.InDelta(t, float32(0.8), prim.Data.Colors0[0][3], 0.01) // opacity
}

func TestConvertCrateMesh_TimeSamples(t *testing.T) {
	cr := buildFakeCrate([]string{tokTimeSamples, tokFaceIdx})
	asset := &ir.Asset{}

	pointsVR := writeTimeSampledVec3VR(cr, 100, []float64{0, 1}, [][][3]float32{
		{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		{{0.1, 0, 0}, {1.1, 0, 0}, {0.1, 1, 0}},
	})

	idxOff := 400
	binary.LittleEndian.PutUint64(cr.data[idxOff:], 3)
	for i, v := range []uint32{0, 1, 2} {
		binary.LittleEndian.PutUint32(cr.data[idxOff+8+i*4:], v)
	}
	idxVR := uint64(idxOff) << valueRepPayShift

	fields := []crateField{
		field(cr, tokTimeSamples, pointsVR),
		field(cr, tokFaceIdx, idxVR),
	}

	convertCrateMesh(cr, fields, "AnimatedMesh", asset)

	require.Len(t, asset.Meshes, 1)
	prim := asset.Meshes[0].Primitives[0]
	require.Len(t, prim.Data.Positions, 3)
	require.Len(t, prim.MorphTargets, 1)
	assert.Equal(t, "frame_1", prim.MorphTargets[0].Name)
	assert.InDelta(t, float32(0.1), prim.MorphTargets[0].Positions[0][0], 0.001)
}

func BenchmarkConvertCrateMesh_TimeSamples(b *testing.B) {
	cr := buildFakeCrate([]string{tokTimeSamples, tokFaceIdx})
	pointsVR := writeTimeSampledVec3VR(cr, 100, []float64{0, 1}, [][][3]float32{
		{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		{{0.1, 0, 0}, {1.1, 0, 0}, {0.1, 1, 0}},
	})
	idxOff := 400
	binary.LittleEndian.PutUint64(cr.data[idxOff:], 3)
	for i, v := range []uint32{0, 1, 2} {
		binary.LittleEndian.PutUint32(cr.data[idxOff+8+i*4:], v)
	}
	idxVR := uint64(idxOff) << valueRepPayShift
	fields := []crateField{
		field(cr, tokTimeSamples, pointsVR),
		field(cr, tokFaceIdx, idxVR),
	}

	b.ReportAllocs()
	for b.Loop() {
		asset := &ir.Asset{}
		convertCrateMesh(cr, fields, "AnimatedMesh", asset)
	}
}

type recordingReporter struct {
	deps []struct {
		kind, path, relation, reportedBy string
	}
}

func (r *recordingReporter) AddDependency(kind, path, relation, reportedBy string) {
	r.deps = append(r.deps, struct {
		kind, path, relation, reportedBy string
	}{kind: kind, path: path, relation: relation, reportedBy: reportedBy})
}

func (*recordingReporter) AddProvenanceNote(string, string) {}

func TestResolveCrateExternalRefs(t *testing.T) {
	refMesh := "#usda 1.0\n\n" +
		"def Mesh \"RefBox\" {\n" +
		"    point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2]\n" +
		"}\n"
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create("ref.usda")
	require.NoError(t, err)
	_, err = w.Write([]byte(refMesh))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	zr, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	require.NoError(t, err)

	reporter := &recordingReporter{}
	cr := buildFakeCrate([]string{tokReferences})
	cr.strings = []string{"@./ref.usda@</Model>"}
	fields := []crateField{{tokenIdx: 0, valueRep: makeInlineStringVR(0)}}
	asset := ir.NewAsset(ir.FormatUSD)

	err = resolveCrateExternalRefs(cr, crateDecodeContext{
		archive:     zr.File,
		maxFileSize: 1 << 20,
		reporter:    reporter,
	}, asset, fields, tokReferences, "reference")
	require.NoError(t, err)
	require.Len(t, reporter.deps, 1)
	assert.Equal(t, "ref.usda", reporter.deps[0].path)
	require.Len(t, asset.Meshes, 1)
	assert.Equal(t, "RefBox", asset.Meshes[0].Name)
}

func TestResolveInheritedNodes(t *testing.T) {
	asset := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "Base", MeshIndex: 2, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
			{Name: "Derived", MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
	}
	resolveInheritedNodes(asset, []inheritArc{{nodeIdx: 1, basePath: "Base"}})
	assert.Equal(t, 2, asset.Nodes[1].MeshIndex)
}

func TestMergeRefScene(t *testing.T) {
	dst := &ir.Asset{
		Meshes:    []*ir.Mesh{{Name: "dst_mesh", Primitives: []ir.Primitive{{MaterialIndex: ir.NoIndex}}}},
		Materials: []*ir.Material{{Name: "dst_mat"}},
		Nodes: []ir.Node{
			{Name: "root", MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
		RootNodes: []int{0},
	}
	src := &ir.Asset{
		Meshes:    []*ir.Mesh{{Name: "src_mesh", Primitives: []ir.Primitive{{MaterialIndex: 0}}}},
		Materials: []*ir.Material{{Name: "src_mat"}},
		Textures:  []*ir.Texture{{Name: "src_tex"}},
		Nodes: []ir.Node{
			{Name: "src_root", MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
		RootNodes: []int{0},
	}

	mergeRefScene(dst, src)

	assert.Len(t, dst.Meshes, 2)
	assert.Len(t, dst.Materials, 2)
	assert.Len(t, dst.Textures, 1)
	assert.Len(t, dst.Nodes, 2)
	assert.Len(t, dst.RootNodes, 2)
	assert.Equal(t, 1, dst.Nodes[1].MeshIndex)                    // offset by meshOff=1
	assert.Equal(t, 1, dst.Meshes[1].Primitives[0].MaterialIndex) // offset
}

func TestParseInheritArc(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Xform \"Base\" {\n" +
		"    def Mesh \"Body\" {\n" +
		"        point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"        int[] faceVertexIndices = [0,1,2]\n" +
		"    }\n" +
		"}\n" +
		"def Xform \"Derived\" (\n" +
		"    inherits = </Base>\n" +
		") {\n" +
		"}\n"

	dec := &Decoder{}
	sc, err := dec.Decode(
		bytes.NewReader([]byte(input)),
		detect.DecodeOptions{},
	)
	require.NoError(t, err)
	require.True(t, len(sc.Meshes) >= 1)
}

func TestParseVariantSetsExercise(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Mesh \"Obj\" {\n" +
		"    point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2]\n" +
		"    variantSet \"look\" = {\n" +
		"        \"shiny\" {\n" +
		"        }\n" +
		"        \"matte\" {\n" +
		"        }\n" +
		"    }\n" +
		"}\n"

	dec := &Decoder{}
	sc, err := dec.Decode(
		bytes.NewReader([]byte(input)),
		detect.DecodeOptions{},
	)
	require.NoError(t, err)
	require.Len(t, sc.Meshes, 1)
}

func TestWireDeferredGeomSubsets(t *testing.T) {
	cr := &crateReader{
		data:   make([]byte, 4096),
		tokens: []string{tokElementType, tokFamilyName, tokSubsetIndices, tokMatBinding},
		paths:  []cratePath{{parentIdx: 0}, {parentIdx: 0}},
	}

	// Build a scene with one mesh that has 2 triangles (6 indices)
	asset := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Name: "TestMesh",
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					Positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}},
					Indices:   []uint32{0, 1, 2, 1, 3, 2}, // 2 triangles
				},
				MaterialIndex: ir.NoIndex,
			}},
		}},
		Materials: []*ir.Material{{Name: "RedMat"}, {Name: "BlueMat"}},
	}

	meshPathToIdx := map[int32]int{0: 0}

	// Build geom subset data
	indicesVR := writeIntArrayVR(cr, 100, []int32{0})  // face 0
	indicesVR2 := writeIntArrayVR(cr, 200, []int32{1}) // face 1

	gsubsets := []deferredGeomSubset{
		{
			pathIdx: 1,
			fields: []crateField{
				field(cr, tokElementType, makeInlineToken(int32(len(cr.tokens)))),
				field(cr, tokSubsetIndices, indicesVR),
				field(cr, tokMatBinding, makeInlineToken(int32(len(cr.tokens)+1))),
			},
		},
		{
			pathIdx: 1,
			fields: []crateField{
				field(cr, tokElementType, makeInlineToken(int32(len(cr.tokens)))),
				field(cr, tokSubsetIndices, indicesVR2),
				field(cr, tokMatBinding, makeInlineToken(int32(len(cr.tokens)+2))),
			},
		},
	}
	cr.tokens = append(cr.tokens, usdaElementFace, "RedMat", "BlueMat")

	wireDeferredGeomSubsets(cr, asset, meshPathToIdx, gsubsets)

	assert.True(t, len(asset.Meshes[0].Primitives) >= 1)
}

func TestWireDeferredBlendShapes(t *testing.T) {
	cr := &crateReader{
		data:   make([]byte, 4096),
		tokens: []string{tokBlendOffsets, tokBlendPointIdx, tokBlendNormalOff},
		paths:  []cratePath{{parentIdx: 0}, {parentIdx: 0}},
	}

	asset := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Name: "TestMesh",
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					Positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Indices:   []uint32{0, 1, 2},
				},
				MaterialIndex: ir.NoIndex,
			}},
		}},
	}

	meshPathToIdx := map[int32]int{0: 0}

	offsetsVR := writeVec3fVR(cr, 100, [][3]float32{{0.1, 0, 0}, {0, 0.1, 0}, {0, 0, 0.1}})
	indicesVR := writeIntArrayVR(cr, 300, []int32{0, 1, 2})

	// Also set the path name for the blend shape
	cr.paths = append(cr.paths, cratePath{parentIdx: 0})

	bshapes := []deferredBlendShape{
		{
			pathIdx: 1,
			fields: []crateField{
				field(cr, tokBlendOffsets, offsetsVR),
				field(cr, tokBlendPointIdx, indicesVR),
			},
		},
	}

	wireDeferredBlendShapes(cr, asset, meshPathToIdx, bshapes)

	require.Len(t, asset.Meshes[0].Primitives[0].MorphTargets, 1)
	mt := asset.Meshes[0].Primitives[0].MorphTargets[0]
	assert.Len(t, mt.Positions, 3)
	assert.Len(t, mt.Indices, 3)
}
