package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimizeMeshes(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{
				{
					MaterialIndex: 0,
					Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
						Indices:     []uint32{0, 1, 2},
					},
				},
				{
					MaterialIndex: 0,
					Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{2, 0, 0}, {3, 0, 0}, {2, 1, 0}},
						Indices:     []uint32{0, 1, 2},
					},
				},
			},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPOptimizeMeshes, process.Options{}))

	prims := scene.Meshes[0].Primitives
	require.Len(t, prims, 1, "same-material primitives should merge")
	assert.Equal(t, 6, prims[0].Data.VertexCount)
	assert.Len(t, prims[0].Data.Indices, 6)
	// Second primitive's indices should be rebased by 3.
	assert.Equal(t, uint32(3), prims[0].Data.Indices[3])
}

func TestFindInstances(t *testing.T) {
	shared := [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{
			{Primitives: []ir.Primitive{{Data: ir.MeshData{VertexCount: 3, Positions: shared}}}},
			{Primitives: []ir.Primitive{{Data: ir.MeshData{VertexCount: 3, Positions: shared}}}},
		},
		Nodes: []ir.Node{
			{MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
			{MeshIndex: 1, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
	}

	require.NoError(t, process.Apply(scene, process.PPFindInstances, process.Options{}))

	assert.Equal(t, scene.Nodes[0].MeshIndex, scene.Nodes[1].MeshIndex, "duplicate mesh nodes should share index")
}

func TestFindInstancesUsesFullData(t *testing.T) {
	// Two meshes with same positions but different normals.
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{
			{
				Primitives: []ir.Primitive{{
					Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
						Normals:     [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
					},
				}},
			},
			{
				Primitives: []ir.Primitive{{
					Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
						Normals:     [][3]float32{{0, 1, 0}, {0, 1, 0}, {0, 1, 0}},
					},
				}},
			},
		},
		Nodes: []ir.Node{{MeshIndex: 0}, {MeshIndex: 1}},
	}

	require.NoError(t, process.Apply(scene, process.PPFindInstances, process.Options{}))

	// Should NOT be merged since normals differ.
	assert.Equal(t, 0, scene.Nodes[0].MeshIndex)
	assert.Equal(t, 1, scene.Nodes[1].MeshIndex)
}

func TestOptimizeMeshesNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPOptimizeMeshes, process.Options{}))
}

func TestFindInstancesDuplicates(t *testing.T) {
	mesh := ir.MeshData{
		VertexCount: 3,
		Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
	}
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{
			{Primitives: []ir.Primitive{{Data: mesh}}},
			{Primitives: []ir.Primitive{{Data: mesh}}},
		},
		Nodes: []ir.Node{{MeshIndex: 0}, {MeshIndex: 1}},
	}

	require.NoError(t, process.Apply(scene, process.PPFindInstances, process.Options{}))
	assert.Equal(t, scene.Nodes[0].MeshIndex, scene.Nodes[1].MeshIndex)
	assert.Len(t, scene.Meshes, 1)
}

func TestFindInstancesNilMesh(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{nil, {Primitives: []ir.Primitive{}}},
		Nodes:  []ir.Node{{MeshIndex: 0}, {MeshIndex: 1}},
	}
	require.NoError(t, process.Apply(scene, process.PPFindInstances, process.Options{}))
}

func TestOptimizeMeshesIndexRebasing(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{
				{
					MaterialIndex: 0,
					Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
						Indices:     []uint32{0, 1, 2},
					},
				},
				{
					MaterialIndex: 0,
					Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{2, 0, 0}, {3, 0, 0}, {2, 1, 0}},
						Indices:     []uint32{0, 1, 2},
					},
				},
			},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPOptimizeMeshes, process.Options{}))
	assert.Len(t, scene.Meshes[0].Primitives, 1)
	assert.Equal(t, 6, scene.Meshes[0].Primitives[0].Data.VertexCount)
}
