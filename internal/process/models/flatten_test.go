package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlattenHierarchy(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{
			{
				Name:      "Root",
				Transform: ir.Transform{Translation: [3]float32{10, 0, 0}, Scale: [3]float32{1, 1, 1}, Rotation: [4]float32{0, 0, 0, 1}},
				Children:  []int{1},
				MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
			},
			{
				Name:      "Child",
				Transform: ir.IdentityTransform(),
				MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
			},
		},
		RootNodes: []int{0},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{0, 0, 0}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFlattenHierarchy, process.Options{}))

	pos := scene.Meshes[0].Primitives[0].Data.Positions[0]
	assert.InDelta(t, 10.0, pos[0], 0.001, "vertex should be translated by parent transform")
}

func TestFlattenSkipsSkinned(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{
			{
				Name:      "SkinnedNode",
				Transform: ir.Transform{Translation: [3]float32{5, 0, 0}, Scale: [3]float32{1, 1, 1}, Rotation: [4]float32{0, 0, 0, 1}},
				MeshIndex: 0, SkinIndex: 0, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
			},
		},
		RootNodes: []int{0},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{1, 2, 3}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFlattenHierarchy, process.Options{}))

	pos := scene.Meshes[0].Primitives[0].Data.Positions[0]
	assert.InDelta(t, 1.0, pos[0], 0.001, "skinned mesh position should be unchanged")
}

func TestFlattenRootOnly(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{{
			Name:      "Root",
			Transform: ir.Transform{Translation: [3]float32{5, 0, 0}, Scale: [3]float32{1, 1, 1}, Rotation: [4]float32{0, 0, 0, 1}},
			MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
		}},
		RootNodes: []int{0},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 1, Positions: [][3]float32{{0, 0, 0}}},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFlattenHierarchy, process.Options{}))
	assert.InDelta(t, 5.0, scene.Meshes[0].Primitives[0].Data.Positions[0][0], 0.001)
}
