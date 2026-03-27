package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalScale(t *testing.T) {
	scene := &ir.Asset{
		Unit: 1.0,
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{1, 2, 3}},
				},
			}},
		}},
	}

	opts := process.Options{GlobalScale: 0.01}
	require.NoError(t, process.Apply(scene, process.PPGlobalScale, opts))

	pos := scene.Meshes[0].Primitives[0].Data.Positions[0]
	assert.InDelta(t, 0.01, pos[0], 0.001)
	assert.InDelta(t, 0.02, pos[1], 0.001)
	assert.InDelta(t, 0.03, pos[2], 0.001)
	assert.InDelta(t, 0.01, scene.Unit, 0.001)
}

func TestPreTransform(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{{
			Transform: ir.Transform{
				Translation: [3]float32{10, 0, 0},
				Scale:       [3]float32{1, 1, 1},
				Rotation:    [4]float32{0, 0, 0, 1},
			},
			MeshIndex:   0,
			SkinIndex:   ir.NoIndex,
			CameraIndex: ir.NoIndex,
			LightIndex:  ir.NoIndex,
		}},
		RootNodes: []int{0},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{0, 0, 0}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPPreTransform, process.Options{}))

	pos := scene.Meshes[0].Primitives[0].Data.Positions[0]
	assert.InDelta(t, 10.0, pos[0], 0.001, "position should be baked with node translation")
}

func TestGlobalScaleEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		scale float64
	}{
		{name: "zero_no_op", scale: 0},
		{name: "identity_no_op", scale: 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Data: ir.MeshData{VertexCount: 1, Positions: [][3]float32{{1, 2, 3}}},
					}},
				}},
			}
			require.NoError(t, process.Apply(scene, process.PPGlobalScale, process.Options{GlobalScale: tt.scale}))
			assert.Equal(t, float32(1.0), scene.Meshes[0].Primitives[0].Data.Positions[0][0])
		})
	}

	t.Run("nil_mesh", func(t *testing.T) {
		scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
		require.NoError(t, process.Apply(scene, process.PPGlobalScale, process.Options{GlobalScale: 2.0}))
	})
}
