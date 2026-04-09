package fbx

import (
	"strings"

	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

func convertGeometry(node *fbxNode) *ir.Mesh { //nolint:funlen // geometry extraction
	var vertices []float64
	var polyIndices []int32
	var normals *layerElement
	var uvs *layerElement
	var uv1 *layerElement
	var colors *layerElement
	var tangents *layerElement
	var binormals *layerElement
	var smoothGroups []int32
	uvCount := 0

	for i := range node.children {
		child := &node.children[i]
		switch child.name {
		case geoVertices:
			if len(child.properties) > 0 {
				vertices = child.properties[0].arrF64
			}
		case geoPolyIndices:
			if len(child.properties) > 0 {
				polyIndices = child.properties[0].arrI32
			}
		case layerNormal:
			normals = parseLayerElement(child)
		case layerUV:
			switch uvCount {
			case 0:
				uvs = parseLayerElement(child)
			case 1:
				uv1 = parseLayerElement(child)
			}
			uvCount++
		case layerColor:
			colors = parseLayerElement(child)
		case layerTangent:
			tangents = parseLayerElement(child)
		case layerBinormal:
			binormals = parseLayerElement(child)
		case layerSmoothing:
			smoothGroups = parseSmoothingGroups(child)
		}
	}

	if len(vertices) == 0 || len(polyIndices) == 0 {
		return nil
	}

	controlPts := f64ToVec3(vertices)

	var expandedNormals [][3]float32
	var expandedUVs [][2]float32
	var expandedUV1 [][2]float32
	var expandedColors [][4]float32
	var expandedTangents [][4]float32
	var expandedBinormals [][3]float32
	if normals != nil {
		expandedNormals = expandNormals(normals, polyIndices)
	}
	if uvs != nil {
		expandedUVs = expandUVs(uvs, polyIndices)
	}
	if uv1 != nil {
		expandedUV1 = expandUVs(uv1, polyIndices)
	}
	if colors != nil {
		expandedColors = expandColors(colors, polyIndices)
	}
	if tangents != nil {
		expandedTangents = expandTangents(tangents, polyIndices)
	}
	if binormals != nil {
		expandedBinormals = expandBinormals(binormals, polyIndices)
	}
	applyBinormalHandedness(expandedTangents, expandedNormals, expandedBinormals)

	attrCount := len(controlPts)
	if len(expandedNormals) > attrCount {
		attrCount = len(expandedNormals)
	}
	if len(expandedUVs) > attrCount {
		attrCount = len(expandedUVs)
	}

	var positions [][3]float32
	var indices []uint32
	if attrCount > len(controlPts) {
		positions = expandPositions(controlPts, polyIndices)
		indices = triangulateExpanded(polyIndices)
	} else {
		positions = controlPts
		indices = triangulatePolygons(polyIndices)
	}

	if expandedNormals == nil && len(smoothGroups) > 0 && len(positions) > 0 && len(indices) > 0 {
		expandedNormals = computeNormalsFromSmoothing(positions, indices, smoothGroups)
	}

	data := ir.MeshData{
		VertexCount: len(positions),
		Positions:   positions,
		Indices:     indices,
		Normals:     expandedNormals,
		Tangents:    expandedTangents,
		TexCoord0:   expandedUVs,
		TexCoord1:   expandedUV1,
		Colors0:     expandedColors,
	}

	return &ir.Mesh{
		Name: extractName(node),
		Primitives: []ir.Primitive{{
			Mode: ir.Triangles,
			Data: data,
		}},
	}
}

func expandPositions(controlPts [][3]float32, polyIndices []int32) [][3]float32 {
	out := make([][3]float32, len(polyIndices))
	for i, idx := range polyIndices {
		ci := idx
		if ci < 0 {
			ci = ^ci
		}
		if int(ci) < len(controlPts) {
			out[i] = controlPts[ci]
		}
	}
	return out
}

func triangulateExpanded(polyIndices []int32) []uint32 {
	indices := make([]uint32, 0, len(polyIndices)*2) //nolint:mnd // triangle index estimate
	faceStart := 0

	for i, idx := range polyIndices {
		if idx < 0 {
			faceLen := i - faceStart + 1
			for j := 2; j < faceLen; j++ {
				indices = append(indices,
					uint32(faceStart),     //nolint:gosec // bounded
					uint32(faceStart+j-1), //nolint:gosec // bounded
					uint32(faceStart+j))   //nolint:gosec // bounded
			}
			faceStart = i + 1
		}
	}
	return indices
}

func convertModel(node *fbxNode) ir.Node {
	t := ir.Transform{
		Rotation: mathx.IdentityQuat,
		Scale:    mathx.IdentityScale,
	}

	p70 := findNode(node.children, nodeProperties70)
	if p70 != nil {
		t = readModelTransform(p70)
	}

	name := extractName(node)
	isCollision := false
	nameUpper := strings.ToUpper(name)
	if strings.HasPrefix(nameUpper, "UCX_") || strings.HasPrefix(nameUpper, "UBX_") ||
		strings.HasPrefix(nameUpper, "USP_") || strings.HasPrefix(nameUpper, "UCP_") {

		isCollision = true
	}

	return ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   ir.NoIndex,
		SkinIndex:   ir.NoIndex,
		CameraIndex: ir.NoIndex,
		LightIndex:  ir.NoIndex,
		IsCollision: isCollision,
		Transform:   t,
	}
}

func readModelTransform(p70 *fbxNode) ir.Transform {
	t := ir.Transform{
		Scale:    mathx.IdentityScale,
		Rotation: mathx.IdentityQuat,
	}

	var preRot [4]float32
	hasPreRot := false
	var geomT [3]float32
	var geomR [3]float64
	var geomS = mathx.IdentityScale
	hasGeom := false

	for _, p := range p70.children {
		if p.name != nodeP || len(p.properties) <= propMinIndex+2 {
			continue
		}
		name := p.properties[0].strVal
		x := float32(p.properties[propMinIndex].floatVal)
		y := float32(p.properties[propMinIndex+1].floatVal)
		z := float32(p.properties[propMinIndex+2].floatVal)

		switch name {
		case propLclTranslate:
			t.Translation = [3]float32{x, y, z}
		case propLclRotation:
			t.Rotation = mathx.EulerToQuat(float64(x)*degToRad, float64(y)*degToRad, float64(z)*degToRad)
		case propLclScaling:
			t.Scale = [3]float32{x, y, z}
		case propPreRotation:
			preRot = mathx.EulerToQuat(float64(x)*degToRad, float64(y)*degToRad, float64(z)*degToRad)
			hasPreRot = true
		case propGeomTranslate:
			geomT = [3]float32{x, y, z}
			hasGeom = true
		case propGeomRotation:
			geomR = [3]float64{float64(x), float64(y), float64(z)}
			hasGeom = true
		case propGeomScaling:
			geomS = [3]float32{x, y, z}
			hasGeom = true
		}
	}

	if hasPreRot {
		t.Rotation = mathx.QuatMulArr(preRot, t.Rotation)
	}

	if hasGeom {
		nodeMat := mathx.ComposeTRS(t.Translation, mathx.ArrToQuat(t.Rotation), t.Scale)
		geomRot := mathx.EulerToQuat(geomR[0]*degToRad, geomR[1]*degToRad, geomR[2]*degToRad)
		geomMat := mathx.ComposeTRS(geomT, mathx.ArrToQuat(geomRot), geomS)
		t.Matrix = nodeMat.Mul4(geomMat)
	}

	return t
}
