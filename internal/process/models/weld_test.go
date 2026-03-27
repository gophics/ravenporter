package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeldSharedVertices(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 4,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 0}},
					Indices:     []uint32{0, 1, 2, 3, 1, 2},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPJoinIdenticalVertices, process.Options{}))

	p := &scene.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
	assert.Equal(t, []uint32{0, 1, 2, 0, 1, 2}, p.Data.Indices)
}

func TestWeldNoOp(t *testing.T) {
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

	require.NoError(t, process.Apply(scene, process.PPJoinIdenticalVertices, process.Options{}))
	assert.Equal(t, 3, scene.Meshes[0].Primitives[0].Data.VertexCount)
}

func TestWeldPreservesSkinnedData(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 4,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 0}},
					TexCoord1:   [][2]float32{{0, 0}, {1, 0}, {0, 1}, {0, 0}},
					Joints0:     [][4]uint16{{1, 0, 0, 0}, {2, 0, 0, 0}, {3, 0, 0, 0}, {1, 0, 0, 0}},
					Weights0:    [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}},
					Indices:     []uint32{0, 1, 2, 3, 1, 2},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPJoinIdenticalVertices, process.Options{}))

	p := &scene.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
	assert.Len(t, p.Data.TexCoord1, 3)
	assert.Len(t, p.Data.Joints0, 3)
	assert.Len(t, p.Data.Weights0, 3)
	assert.Equal(t, [4]uint16{1, 0, 0, 0}, p.Data.Joints0[0])
}

func TestWeldNonIndexed(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 6,
					Positions: [][3]float32{
						{0, 0, 0}, {1, 0, 0}, {0, 1, 0},
						{0, 0, 0}, {1, 0, 0}, {0, 1, 0},
					},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPJoinIdenticalVertices, process.Options{}))
	assert.Equal(t, 3, scene.Meshes[0].Primitives[0].Data.VertexCount)
	assert.Len(t, scene.Meshes[0].Primitives[0].Data.Indices, 6)
}

func TestWeldWithAllAttributes(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 4,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 0}},
					Normals:     [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
					Tangents:    [][4]float32{{1, 0, 0, 1}, {1, 0, 0, 1}, {1, 0, 0, 1}, {1, 0, 0, 1}},
					TexCoord0:   [][2]float32{{0, 0}, {1, 0}, {0, 1}, {0, 0}},
					TexCoord1:   [][2]float32{{0, 0}, {1, 0}, {0, 1}, {0, 0}},
					Colors0:     [][4]float32{{1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}},
					Joints0:     [][4]uint16{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}},
					Joints1:     [][4]uint16{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}},
					Weights0:    [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}},
					Weights1:    [][4]float32{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}},
					Indices:     []uint32{0, 1, 2, 3, 1, 2},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPJoinIdenticalVertices, process.Options{}))
	assert.Equal(t, 3, scene.Meshes[0].Primitives[0].Data.VertexCount)
}
