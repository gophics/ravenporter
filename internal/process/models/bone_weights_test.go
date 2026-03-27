package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitBoneWeights(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{0, 0, 0}},
					Joints0:     [][4]uint16{{0, 1, 2, 3}},
					Weights0:    [][4]float32{{0.1, 0.2, 0.3, 0.05}},
					Joints1:     [][4]uint16{{4, 5, 6, 7}},
					Weights1:    [][4]float32{{0.05, 0.15, 0.1, 0.05}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPLimitBoneWeights, process.Options{MaxBoneWeights: 4}))

	d := &scene.Meshes[0].Primitives[0].Data
	assert.Nil(t, d.Joints1, "extended joint arrays should be cleared")
	assert.Nil(t, d.Weights1, "extended weight arrays should be cleared")

	// Verify weights are renormalized to sum to 1.0.
	var sum float32
	for _, w := range d.Weights0[0] {
		sum += w
	}
	assert.InDelta(t, 1.0, sum, 0.001, "weights should sum to 1.0")
}

func TestLimitBoneWeightsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		joints0  [][4]uint16
		weights0 [][4]float32
		joints1  [][4]uint16
		weights1 [][4]float32
		limit    int
		checkSum bool
	}{
		{
			name:     "normalize_overflow",
			joints0:  [][4]uint16{{0, 1, 2, 3}},
			weights0: [][4]float32{{0.5, 0.3, 0.1, 0.05}},
			joints1:  [][4]uint16{{4, 5, 6, 7}},
			weights1: [][4]float32{{0.02, 0.01, 0.01, 0.01}},
			limit:    2,
			checkSum: true,
		},
		{
			name:     "no_op_under_limit",
			joints0:  [][4]uint16{{0, 1, 0, 0}},
			weights0: [][4]float32{{0.7, 0.3, 0, 0}},
			limit:    4,
		},
		{name: "max_at_8_no_op", limit: 8},
		{name: "empty_weights", limit: 2},
		{
			name:     "default_limit",
			joints0:  [][4]uint16{{0, 1, 2, 3}},
			weights0: [][4]float32{{0.5, 0.3, 0.1, 0.1}},
			limit:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Data: ir.MeshData{
							VertexCount: 1,
							Positions:   [][3]float32{{0, 0, 0}},
							Joints0:     tt.joints0,
							Weights0:    tt.weights0,
							Joints1:     tt.joints1,
							Weights1:    tt.weights1,
						},
					}},
				}},
			}
			require.NoError(t, process.Apply(scene, process.PPLimitBoneWeights, process.Options{MaxBoneWeights: tt.limit}))
			if tt.checkSum {
				w := scene.Meshes[0].Primitives[0].Data.Weights0[0]
				sum := w[0] + w[1] + w[2] + w[3]
				assert.InDelta(t, 1.0, sum, 0.01)
				assert.Nil(t, scene.Meshes[0].Primitives[0].Data.Weights1)
			}
		})
	}

	t.Run("nil_mesh", func(t *testing.T) {
		scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
		require.NoError(t, process.Apply(scene, process.PPLimitBoneWeights, process.Options{MaxBoneWeights: 2}))
	})
}
