package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveComponent(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Normals:     [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
					Tangents:    [][4]float32{{1, 0, 0, 1}, {1, 0, 0, 1}, {1, 0, 0, 1}},
					TexCoord0:   [][2]float32{{0, 0}, {1, 0}, {0, 1}},
				},
			}},
		}},
	}

	opts := process.Options{RemoveFlags: process.CompNormals | process.CompTangents}
	require.NoError(t, process.Apply(scene, process.PPRemoveComponent, opts))

	d := &scene.Meshes[0].Primitives[0].Data
	assert.Nil(t, d.Normals, "normals should be stripped")
	assert.Nil(t, d.Tangents, "tangents should be stripped")
	assert.NotNil(t, d.TexCoord0, "UVs should be preserved")
	assert.NotNil(t, d.Positions, "positions should be preserved")
}

func TestValidateAnimationsNoError(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{{Name: "node0"}},
		Animations: []*ir.Animation{{
			Channels: []ir.AnimationChannel{{NodeIndex: 999}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPValidateAnimations, process.Options{}))
}

func TestFindInvalidNoError(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{MaterialIndex: 999}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFindInvalid, process.Options{}))
}

func TestReportStatsNoError(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{Data: ir.MeshData{VertexCount: 3, Indices: []uint32{0, 1, 2}}}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPReportStats, process.Options{}))
}

func TestFindInvalidSanitizesNaN(t *testing.T) {
	nan := float32(0)
	nan /= nan // NaN

	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Name: "test",
			Primitives: []ir.Primitive{{
				MaterialIndex: ir.NoIndex,
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{nan, 1, 2}},
					Normals:     [][3]float32{{0, 0, 0}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPFindInvalid, process.Options{}))

	// NaN position should be zeroed.
	assert.Equal(t, float32(0), scene.Meshes[0].Primitives[0].Data.Positions[0][0])
	// Zero-length normal should be replaced with fallback.
	assert.Equal(t, [3]float32{0, 0, 1}, scene.Meshes[0].Primitives[0].Data.Normals[0])
}

func TestRemoveComponentMultipleFlags(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{1, 2, 3}},
					Normals:     [][3]float32{{0, 0, 1}},
					Tangents:    [][4]float32{{1, 0, 0, 1}},
					TexCoord0:   [][2]float32{{0, 0}},
					TexCoord1:   [][2]float32{{0.5, 0.5}},
					Colors0:     [][4]float32{{1, 1, 1, 1}},
					Joints0:     [][4]uint16{{0, 0, 0, 0}},
					Weights0:    [][4]float32{{1, 0, 0, 0}},
				},
			}},
		}},
	}

	opts := process.Options{
		RemoveFlags: process.CompNormals | process.CompTangents | process.CompTexCoord0 | process.CompTexCoord1 | process.CompColors0 | process.CompJoints | process.CompWeights,
	}
	require.NoError(t, process.Apply(scene, process.PPRemoveComponent, opts))

	p := scene.Meshes[0].Primitives[0].Data
	assert.Nil(t, p.Normals)
	assert.Nil(t, p.Tangents)
	assert.Nil(t, p.TexCoord0)
	assert.Nil(t, p.TexCoord1)
	assert.Nil(t, p.Colors0)
	assert.Nil(t, p.Joints0)
	assert.Nil(t, p.Weights0)
}

func TestFindInvalidNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPFindInvalid, process.Options{}))
}

func TestFindInvalidZeroTangent(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{1, 2, 3}},
					Tangents:    [][4]float32{{0, 0, 0, 0}},
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPFindInvalid, process.Options{}))
	assert.Equal(t, [4]float32{1, 0, 0, 1}, scene.Meshes[0].Primitives[0].Data.Tangents[0])
}
