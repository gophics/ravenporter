package fbx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMorphTargets(t *testing.T) {
	const (
		geoID     = 100
		bsID      = 200 // BlendShape deformer
		bscID     = 300 // BlendShapeChannel deformer
		shapeID   = 400 // Shape geometry
		modelID   = 500
		positions = 3 // 3 vertices
	)

	verts := make([]float64, positions*3)
	for i := range verts {
		verts[i] = float64(i)
	}

	shapeVerts := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9}

	objects := fbxNode{
		name: nodeObjects,
		children: []fbxNode{
			obj(objModel, modelID, "Cube"),
			{
				name:       objGeometry,
				properties: []fbxProp{intProp(geoID), strProp("CubeMesh"), strProp("Mesh")},
				children: []fbxNode{
					{name: geoVertices, properties: []fbxProp{f64Prop(verts)}},
					{name: geoPolyIndices, properties: []fbxProp{i32Prop([]int32{0, 1, -3})}},
				},
			},
			{
				name:       objGeometry,
				properties: []fbxProp{intProp(shapeID), strProp("SmileShape"), strProp(geoSubtypeShape)},
				children: []fbxNode{
					{name: geoVertices, properties: []fbxProp{f64Prop(shapeVerts)}},
				},
			},
			objTyped(bsID, "BlendShape", "BlendShape"),
			objTyped(bscID, "SmileChannel", "BlendShapeChannel"),
		},
	}

	connections := fbxNode{
		name: nodeConnections,
		children: []fbxNode{
			connOONode(geoID, modelID), // Geometry → Model
			connOONode(bsID, geoID),    // BlendShape → Geometry
			connOONode(bscID, bsID),    // BlendShapeChannel → BlendShape
			connOONode(shapeID, bscID), // Shape → BlendShapeChannel
		},
	}

	asset := convertFBX([]fbxNode{objects, connections}, 7400)

	require.GreaterOrEqual(t, len(asset.Meshes), 1)
	mesh := asset.Meshes[0]
	require.GreaterOrEqual(t, len(mesh.Primitives), 1)
	require.Len(t, mesh.Primitives[0].MorphTargets, 1, "should have 1 morph target")

	mt := mesh.Primitives[0].MorphTargets[0]
	assert.Equal(t, "SmileShape", mt.Name)
	assert.Len(t, mt.Positions, positions)
}

func TestResolveMorphTargetsNoConnection(t *testing.T) {
	const (
		geoID   = 100
		shapeID = 200
		modelID = 300
	)

	verts := []float64{0, 0, 0, 1, 0, 0, 0, 1, 0}

	objects := fbxNode{
		name: nodeObjects,
		children: []fbxNode{
			obj(objModel, modelID, "Cube"),
			{
				name:       objGeometry,
				properties: []fbxProp{intProp(geoID), strProp("CubeMesh"), strProp("Mesh")},
				children: []fbxNode{
					{name: geoVertices, properties: []fbxProp{f64Prop(verts)}},
					{name: geoPolyIndices, properties: []fbxProp{i32Prop([]int32{0, 1, -3})}},
				},
			},
			{
				name:       objGeometry,
				properties: []fbxProp{intProp(shapeID), strProp("ShapeKey"), strProp(geoSubtypeShape)},
				children: []fbxNode{
					{name: geoVertices, properties: []fbxProp{f64Prop(verts)}},
				},
			},
		},
	}

	connections := fbxNode{
		name: nodeConnections,
		children: []fbxNode{
			connOONode(geoID, modelID),
		},
	}

	asset := convertFBX([]fbxNode{objects, connections}, 7400)

	require.GreaterOrEqual(t, len(asset.Meshes), 1)
	for _, mesh := range asset.Meshes {
		for _, prim := range mesh.Primitives {
			assert.Empty(t, prim.MorphTargets, "no morph targets without full connection chain")
		}
	}
}

func TestResolveMorphTargetsMultiple(t *testing.T) {
	const (
		geoID    = 100
		bsID     = 200
		bscID1   = 301
		bscID2   = 302
		shapeID1 = 401
		shapeID2 = 402
		modelID  = 500
	)

	verts := []float64{0, 0, 0, 1, 0, 0, 0, 1, 0}
	delta1 := []float64{0.1, 0, 0, 0, 0.1, 0, 0, 0, 0.1}
	delta2 := []float64{0.2, 0, 0, 0, 0.2, 0, 0, 0, 0.2}

	objects := fbxNode{
		name: nodeObjects,
		children: []fbxNode{
			obj(objModel, modelID, "Cube"),
			{
				name:       objGeometry,
				properties: []fbxProp{intProp(geoID), strProp("CubeMesh"), strProp("Mesh")},
				children: []fbxNode{
					{name: geoVertices, properties: []fbxProp{f64Prop(verts)}},
					{name: geoPolyIndices, properties: []fbxProp{i32Prop([]int32{0, 1, -3})}},
				},
			},
			{
				name:       objGeometry,
				properties: []fbxProp{intProp(shapeID1), strProp("Smile"), strProp(geoSubtypeShape)},
				children:   []fbxNode{{name: geoVertices, properties: []fbxProp{f64Prop(delta1)}}},
			},
			{
				name:       objGeometry,
				properties: []fbxProp{intProp(shapeID2), strProp("Frown"), strProp(geoSubtypeShape)},
				children:   []fbxNode{{name: geoVertices, properties: []fbxProp{f64Prop(delta2)}}},
			},
			objTyped(bsID, "BlendShape", "BlendShape"),
			objTyped(bscID1, "SmileChan", "BlendShapeChannel"),
			objTyped(bscID2, "FrownChan", "BlendShapeChannel"),
		},
	}

	connections := fbxNode{
		name: nodeConnections,
		children: []fbxNode{
			connOONode(geoID, modelID),
			connOONode(bsID, geoID),
			connOONode(bscID1, bsID),
			connOONode(bscID2, bsID),
			connOONode(shapeID1, bscID1),
			connOONode(shapeID2, bscID2),
		},
	}

	asset := convertFBX([]fbxNode{objects, connections}, 7400)

	require.GreaterOrEqual(t, len(asset.Meshes), 1)
	mesh := asset.Meshes[0]
	require.GreaterOrEqual(t, len(mesh.Primitives), 1)
	require.Len(t, mesh.Primitives[0].MorphTargets, 2, "should have 2 morph targets")

	names := []string{mesh.Primitives[0].MorphTargets[0].Name, mesh.Primitives[0].MorphTargets[1].Name}
	assert.Contains(t, names, "Smile")
	assert.Contains(t, names, "Frown")
}
