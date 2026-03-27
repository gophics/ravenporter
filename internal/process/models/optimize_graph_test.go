package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimizeGraphRemovesEmptyLeaf(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{
			{
				Name:      "Root",
				Children:  []int{1},
				MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
			},
			{
				Name:      "EmptyLeaf",
				MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
			},
		},
		RootNodes: []int{0},
		Meshes:    []*ir.Mesh{{}},
	}

	require.NoError(t, process.Apply(scene, process.PPOptimizeGraph, process.Options{}))

	assert.Empty(t, scene.Nodes[0].Children, "empty leaf should be pruned")
}

func TestOptimizeGraphPreservesAnimTarget(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{
			{
				Name:      "Root",
				Children:  []int{1},
				MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
			},
			{
				Name:      "AnimTarget",
				MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
			},
		},
		RootNodes: []int{0},
		Meshes:    []*ir.Mesh{{}},
		Animations: []*ir.Animation{{
			Channels: []ir.AnimationChannel{{NodeIndex: 1}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPOptimizeGraph, process.Options{}))

	assert.Len(t, scene.Nodes[0].Children, 1, "animation target node must be preserved")
}

func TestOptimizeGraphPreservesJoints(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "Root", Children: []int{1, 2}, MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
			{Name: "Joint", MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex, IsJoint: true},
			{Name: "Empty", MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
		RootNodes: []int{0},
		Skeletons: []*ir.Skeleton{{Joints: []int{1}}},
	}

	require.NoError(t, process.Apply(scene, process.PPOptimizeGraph, process.Options{}))
	found := false
	for _, n := range scene.Nodes {
		if n.Name == "Joint" {
			found = true
		}
	}
	assert.True(t, found, "joint node must be preserved")
}

func TestOptimizeGraphNilAnim(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "Root", Children: []int{1}, MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
			{Name: "Leaf", MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
		RootNodes:  []int{0},
		Animations: []*ir.Animation{},
		Skeletons:  []*ir.Skeleton{},
	}

	require.NoError(t, process.Apply(scene, process.PPOptimizeGraph, process.Options{}))
	assert.Empty(t, scene.Nodes)
}

func TestOptimizeGraphEmptyNodes(t *testing.T) {
	scene := &ir.Asset{Nodes: []ir.Node{}}
	require.NoError(t, process.Apply(scene, process.PPOptimizeGraph, process.Options{}))
}
