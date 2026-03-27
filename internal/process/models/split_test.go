package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitLargeMeshes(t *testing.T) {
	count := 100
	positions := make([][3]float32, count)
	indices := make([]uint32, count)
	for i := range count {
		positions[i] = [3]float32{float32(i), 0, 0}
		indices[i] = uint32(i)
	}

	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					VertexCount: count,
					Positions:   positions,
					Indices:     indices,
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, process.Options{MaxVerticesPerMesh: 30}))

	prims := scene.Meshes[0].Primitives
	assert.Greater(t, len(prims), 1, "mesh should be split into multiple primitives")

	for _, p := range prims {
		assert.LessOrEqual(t, p.Data.VertexCount, 30, "each chunk must respect vertex limit")
	}
}

func TestSplitSmallMeshNoOp(t *testing.T) {
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

	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, process.Options{}))

	assert.Len(t, scene.Meshes[0].Primitives, 1, "small mesh should not be split")
}

func TestSplitLargeMeshesNonIndexed(t *testing.T) {
	n := 70000
	positions := make([][3]float32, n)
	for i := range n {
		positions[i] = [3]float32{float32(i), 0, 0}
	}

	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: n,
					Positions:   positions,
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, process.Options{}))

	totalVerts := 0
	for _, p := range scene.Meshes[0].Primitives {
		totalVerts += p.Data.VertexCount
		assert.LessOrEqual(t, p.Data.VertexCount, 65535)
	}
	assert.Equal(t, n, totalVerts)
}

func TestSplitLargeMeshesIndexed(t *testing.T) {
	n := 200000
	positions := make([][3]float32, n)
	indices := make([]uint32, n)
	for i := range n {
		positions[i] = [3]float32{float32(i), 0, 0}
		indices[i] = uint32(i)
	}

	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: n,
					Positions:   positions,
					Indices:     indices,
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, process.Options{}))
	assert.Greater(t, len(scene.Meshes[0].Primitives), 1)
	for _, p := range scene.Meshes[0].Primitives {
		assert.LessOrEqual(t, p.Data.VertexCount, 65535)
	}
}

func TestSplitNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, process.Options{}))
}

func TestSplitNonTriangles(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Lines,
				Data: ir.MeshData{
					VertexCount: 100000,
					Positions:   make([][3]float32, 100000),
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, process.Options{}))
	assert.Len(t, scene.Meshes[0].Primitives, 1)
}

func TestSplitLargeMeshesCustomLimit(t *testing.T) {
	positions := make([][3]float32, 12)
	for i := range positions {
		positions[i] = [3]float32{float32(i), 0, 0}
	}

	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 12,
					Positions:   positions,
				},
			}},
		}},
	}

	opts := process.Options{MaxVerticesPerMesh: 6}
	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, opts))
	assert.Greater(t, len(scene.Meshes[0].Primitives), 1)
}

func TestSplitLargeMeshesWithAttributes(t *testing.T) {
	n := 200000
	positions := make([][3]float32, n)
	normals := make([][3]float32, n)
	indices := make([]uint32, n)
	for i := range n {
		positions[i] = [3]float32{float32(i), 0, 0}
		normals[i] = [3]float32{0, 0, 1}
		indices[i] = uint32(i)
	}

	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: n,
					Positions:   positions,
					Normals:     normals,
					Indices:     indices,
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPSplitLargeMeshes, process.Options{}))
	for _, p := range scene.Meshes[0].Primitives {
		assert.Equal(t, len(p.Data.Positions), len(p.Data.Normals))
	}
}
