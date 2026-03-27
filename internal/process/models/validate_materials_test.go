package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMaterials(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{
			{Name: "used", RoughnessFactor: 1.5, MetallicFactor: -0.5},
			{Name: "unused", RoughnessFactor: 0.5},
		},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{MaterialIndex: 0}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPValidateMaterials, process.Options{}))

	// Roughness should be clamped to 1.0.
	assert.Equal(t, float32(1.0), scene.Materials[0].RoughnessFactor)
	// Metallic should be clamped to 0.0.
	assert.Equal(t, float32(0.0), scene.Materials[0].MetallicFactor)
}

func TestValidateMaterialsDefaultAssignment(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{nil},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{MaterialIndex: 0}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPValidateMaterials, process.Options{}))
	assert.Greater(t, len(scene.Materials), 1)
	assert.GreaterOrEqual(t, scene.Meshes[0].Primitives[0].MaterialIndex, 0)
}

func TestValidateMaterialsNilMesh(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{{Name: "mat"}},
		Meshes:    []*ir.Mesh{nil},
	}
	require.NoError(t, process.Apply(scene, process.PPValidateMaterials, process.Options{}))
}

func TestValidateMaterialsNegativeIndex(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{{Name: "mat"}},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{MaterialIndex: -1}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPValidateMaterials, process.Options{}))
	assert.GreaterOrEqual(t, scene.Meshes[0].Primitives[0].MaterialIndex, 0)
}
