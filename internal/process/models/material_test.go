package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveRedundantMaterials(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{
			{Name: "used"},
			{Name: "unused"},
			{Name: "also_used"},
		},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{
				{MaterialIndex: 0},
				{MaterialIndex: 2},
			},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPRemoveRedundantMaterials, process.Options{}))

	require.Len(t, scene.Materials, 2)
	assert.Equal(t, "used", scene.Materials[0].Name)
	assert.Equal(t, "also_used", scene.Materials[1].Name)
	assert.Equal(t, 0, scene.Meshes[0].Primitives[0].MaterialIndex)
	assert.Equal(t, 1, scene.Meshes[0].Primitives[1].MaterialIndex)
}

func TestRemoveRedundantMaterialsNoOp(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{{Name: "mat1"}},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{MaterialIndex: 0}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPRemoveRedundantMaterials, process.Options{}))
	require.Len(t, scene.Materials, 1)
}

func TestRemoveMaterialsNilMesh(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{{Name: "mat"}},
		Meshes:    []*ir.Mesh{nil},
	}
	require.NoError(t, process.Apply(scene, process.PPRemoveRedundantMaterials, process.Options{}))
}

func TestRemoveMaterialsEmpty(t *testing.T) {
	scene := &ir.Asset{Materials: []*ir.Material{}}
	require.NoError(t, process.Apply(scene, process.PPRemoveRedundantMaterials, process.Options{}))
}
