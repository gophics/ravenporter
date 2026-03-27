package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixUpAxis(t *testing.T) {
	scene := &ir.Asset{
		UpAxis: ir.ZUp,
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{1, 2, 3}},
				},
			}},
		}},
	}

	opts := process.Options{TargetUpAxis: ir.YUp}
	require.NoError(t, process.Apply(scene, process.PPFixUpAxis, opts))

	assert.Equal(t, ir.YUp, scene.UpAxis)
	assert.Equal(t, [3]float32{1, 3, 2}, scene.Meshes[0].Primitives[0].Data.Positions[0])
}

func TestMakeLeftHanded(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}},
					Normals:     [][3]float32{{0, 0, 1}, {0, 0, -1}, {0, 1, 0}},
					Tangents:    [][4]float32{{1, 0, 0.5, 1}, {0, 1, -0.5, -1}, {0, 0, 1, 1}},
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
		Nodes: []ir.Node{{
			Transform: ir.Transform{Translation: [3]float32{10, 20, 30}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPMakeLeftHanded, process.Options{}))

	d := &scene.Meshes[0].Primitives[0].Data

	// Positions: Z negated.
	assert.Equal(t, float32(-3), d.Positions[0][2])
	assert.Equal(t, float32(-6), d.Positions[1][2])

	// Normals: Z negated.
	assert.Equal(t, float32(-1), d.Normals[0][2])
	assert.Equal(t, float32(1), d.Normals[1][2])

	// Tangents: Z negated.
	assert.Equal(t, float32(-0.5), d.Tangents[0][2])
	assert.Equal(t, float32(0.5), d.Tangents[1][2])

	// Winding reversed: {0,1,2} → {0,2,1}.
	assert.Equal(t, []uint32{0, 2, 1}, d.Indices)

	// Node translation Z negated.
	assert.Equal(t, float32(-30), scene.Nodes[0].Transform.Translation[2])
}

func TestMakeLeftHandedSkipsEmpty(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 0},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPMakeLeftHanded, process.Options{}))
	assert.Equal(t, 0, scene.Meshes[0].Primitives[0].Data.VertexCount)
}

func TestFixUpAxisSwapsQuaternion(t *testing.T) {
	scene := &ir.Asset{
		UpAxis: ir.ZUp,
		Nodes: []ir.Node{{
			Transform: ir.Transform{
				Translation: [3]float32{1, 2, 3},
				Rotation:    [4]float32{0, 0.5, 0.7, 1},
				Scale:       [3]float32{1, 2, 3},
			},
		}},
		Meshes: []*ir.Mesh{{}},
	}

	require.NoError(t, process.Apply(scene, process.PPFixUpAxis, process.Options{TargetUpAxis: ir.YUp}))

	n := scene.Nodes[0].Transform
	assert.Equal(t, [3]float32{1, 3, 2}, n.Translation)
	assert.Equal(t, [3]float32{1, 3, 2}, n.Scale)
	assert.Equal(t, [4]float32{0, 0.7, 0.5, 1}, n.Rotation)
}

func TestFixUpAxisAlreadyYUp(t *testing.T) {
	scene := &ir.Asset{
		UpAxis: ir.YUp,
		Nodes:  []ir.Node{{Transform: ir.Transform{Translation: [3]float32{1, 2, 3}}}},
	}
	require.NoError(t, process.Apply(scene, process.PPFixUpAxis, process.Options{}))
	assert.Equal(t, [3]float32{1, 2, 3}, scene.Nodes[0].Transform.Translation)
}

func TestMakeLeftHandedNilMesh(t *testing.T) {
	scene := &ir.Asset{
		Nodes:  []ir.Node{{Transform: ir.Transform{Translation: [3]float32{0, 0, 5}}}},
		Meshes: []*ir.Mesh{nil},
	}
	require.NoError(t, process.Apply(scene, process.PPMakeLeftHanded, process.Options{}))
	assert.Equal(t, float32(-5), scene.Nodes[0].Transform.Translation[2])
}

func TestMakeLeftHandedIndices(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Normals:     [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
					Tangents:    [][4]float32{{1, 0, 0, 1}, {1, 0, 0, 1}, {1, 0, 0, 1}},
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
		Nodes: []ir.Node{{}},
	}

	require.NoError(t, process.Apply(scene, process.PPMakeLeftHanded, process.Options{}))
	assert.Equal(t, uint32(2), scene.Meshes[0].Primitives[0].Data.Indices[1])
	assert.Equal(t, uint32(1), scene.Meshes[0].Primitives[0].Data.Indices[2])
}
