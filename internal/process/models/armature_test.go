package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulateArmatureData(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "root"},
			{Name: "bone1"},
			{Name: "bone2"},
			{Name: "mesh", SkinIndex: 0},
		},
		Skeletons: []*ir.Skeleton{{
			Name:    "skeleton",
			Joints:  []int{1, 2},
			RootIdx: 0,
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPPopulateArmatureData, process.Options{}))

	assert.True(t, scene.Nodes[1].IsJoint)
	assert.True(t, scene.Nodes[2].IsJoint)
	assert.False(t, scene.Nodes[0].IsJoint) // root is not a joint
	assert.Len(t, scene.Skeletons[0].InverseBindMatrices, 2)
}

func TestPopulateArmatureNilSkeleton(t *testing.T) {
	scene := &ir.Asset{
		Nodes:     []ir.Node{{Name: "root"}},
		Skeletons: []*ir.Skeleton{nil},
	}
	require.NoError(t, process.Apply(scene, process.PPPopulateArmatureData, process.Options{}))
}
