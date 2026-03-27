package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 99, // wrong
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Indices:     []uint32{0, 1, 999}, // out of range
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPValidate, process.Options{}))

	p := &scene.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
	assert.Equal(t, uint32(2), p.Data.Indices[2]) // clamped to max
}

func TestValidateNormalsMismatch(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Normals:     [][3]float32{{0, 0, 1}}, // length mismatch with positions
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPValidate, process.Options{}))
	assert.Empty(t, scene.Meshes[0].Primitives)
}

func TestValidateTexCoordMismatch(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					TexCoord0:   [][2]float32{{0, 0}}, // length mismatch
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPValidate, process.Options{}))
	assert.Empty(t, scene.Meshes[0].Primitives)
}

func TestValidateNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPValidate, process.Options{}))
}

func TestValidateEmptyVertexCount(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 0, Positions: [][3]float32{}},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPValidate, process.Options{}))
}
