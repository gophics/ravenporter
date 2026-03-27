package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlipUVs(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 2,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}},
					TexCoord0:   [][2]float32{{0.0, 0.25}, {1.0, 0.75}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFlipUVs, process.Options{}))

	uvs := scene.Meshes[0].Primitives[0].Data.TexCoord0
	assert.InDelta(t, 0.75, uvs[0][1], 0.001)
	assert.InDelta(t, 0.25, uvs[1][1], 0.001)
}

func TestFlipUVsTexCoord1(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{0, 0, 0}},
					TexCoord0:   [][2]float32{{0, 0.3}},
					TexCoord1:   [][2]float32{{0.5, 0.8}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFlipUVs, process.Options{}))

	assert.InDelta(t, 0.7, scene.Meshes[0].Primitives[0].Data.TexCoord0[0][1], 0.01)
	assert.InDelta(t, 0.2, scene.Meshes[0].Primitives[0].Data.TexCoord1[0][1], 0.01)
}

func TestFlipUVsNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPFlipUVs, process.Options{}))
}
