package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenUVCoords(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPGenUVCoords, process.Options{}))
	assert.Len(t, scene.Meshes[0].Primitives[0].Data.TexCoord0, 3)
}

func TestGenUVCoordsSkipsExisting(t *testing.T) {
	existing := [][2]float32{{0, 0}, {1, 0}, {0, 1}}
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
					TexCoord0:   existing,
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPGenUVCoords, process.Options{}))
	assert.Equal(t, existing, scene.Meshes[0].Primitives[0].Data.TexCoord0)
}

func TestTransformUVCoords(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{0, 0, 0}},
					TexCoord0:   [][2]float32{{2.5, -0.3}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPTransformUVCoords, process.Options{}))

	uv := scene.Meshes[0].Primitives[0].Data.TexCoord0[0]
	assert.InDelta(t, 0.5, float64(uv[0]), 0.01)
	assert.InDelta(t, 0.7, float64(uv[1]), 0.01)
}

func TestGenUVCoordsEmpty(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 0, Positions: [][3]float32{}},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPGenUVCoords, process.Options{}))
}

func TestGenUVCoordsNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPGenUVCoords, process.Options{}))
}

func TestTransformUVCoordsNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPTransformUVCoords, process.Options{}))
}

func TestTransformUVCoordsEmpty(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 1, Positions: [][3]float32{{0, 0, 0}}},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPTransformUVCoords, process.Options{}))
}
