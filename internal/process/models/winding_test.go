package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlipWindingOrder(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFlipWindingOrder, process.Options{}))
	assert.Equal(t, []uint32{0, 2, 1}, scene.Meshes[0].Primitives[0].Data.Indices)
}

func TestFixWindingCCW(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Normals:     [][3]float32{{0, 0, -1}, {0, 0, -1}, {0, 0, -1}}, // opposing
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFixWinding, process.Options{}))
	// Should have flipped i1 and i2
	assert.Equal(t, []uint32{0, 2, 1}, scene.Meshes[0].Primitives[0].Data.Indices)
}

func TestFixWindingCCWNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPFixWinding, process.Options{}))
}

func TestFixWindingCCWNonTriangles(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Lines,
				Data: ir.MeshData{
					VertexCount: 2,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}},
					Indices:     []uint32{0, 1},
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPFixWinding, process.Options{}))
	assert.Equal(t, []uint32{0, 1}, scene.Meshes[0].Primitives[0].Data.Indices)
}

func TestFlipWindingNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPFlipWindingOrder, process.Options{}))
}
