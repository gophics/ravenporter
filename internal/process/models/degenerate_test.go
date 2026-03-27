package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveDegenerates(t *testing.T) {
	tests := []struct {
		name       string
		mode       ir.PrimitiveMode
		positions  [][3]float32
		indices    []uint32
		degMode    process.DegenerateMode
		wantIdxLen int
		wantMode   ir.PrimitiveMode
	}{
		{
			name:       "removes_degenerate",
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 0}},
			indices:    []uint32{0, 1, 2, 0, 0, 3},
			wantIdxLen: 3,
		},
		{
			name:       "keeps_valid",
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			indices:    []uint32{0, 1, 2},
			wantIdxLen: 3,
		},
		{
			name:       "convert_all_equal_discarded",
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 0}},
			indices:    []uint32{0, 1, 2, 0, 0, 0},
			degMode:    process.DegenerateModeConvert,
			wantIdxLen: 3,
		},
		{
			name:      "convert_to_lines_i0_eq_i1",
			positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			indices:   []uint32{0, 0, 1},
			degMode:   process.DegenerateModeConvert,
			wantMode:  ir.Lines,
		},
		{
			name:      "convert_to_points_all_equal",
			positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			indices:   []uint32{0, 0, 0},
			degMode:   process.DegenerateModeConvert,
			wantMode:  ir.Points,
		},
		{
			name:       "remove_mode",
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			indices:    []uint32{0, 0, 1},
			degMode:    process.DegenerateModeRemove,
			wantIdxLen: 0,
		},
		{
			name:       "non_triangles_skipped",
			mode:       ir.Lines,
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}},
			indices:    []uint32{0, 1},
			wantIdxLen: 2,
			wantMode:   ir.Lines,
		},
		{
			name:      "i2_eq_i0_converts_to_lines",
			positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			indices:   []uint32{0, 1, 0},
			degMode:   process.DegenerateModeConvert,
			wantMode:  ir.Lines,
		},
		{
			name:      "i1_eq_i2_converts_to_lines",
			positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			indices:   []uint32{0, 1, 1},
			degMode:   process.DegenerateModeConvert,
			wantMode:  ir.Lines,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Mode: tt.mode,
						Data: ir.MeshData{VertexCount: len(tt.positions), Positions: tt.positions, Indices: tt.indices},
					}},
				}},
			}
			opts := process.Options{DegenerateMode: tt.degMode}
			require.NoError(t, process.Apply(scene, process.PPRemoveDegenerates, opts))
			p := &scene.Meshes[0].Primitives[0]
			if tt.wantMode != 0 {
				assert.Equal(t, tt.wantMode, p.Mode)
			}
			if tt.wantIdxLen > 0 || tt.degMode == process.DegenerateModeRemove {
				assert.Len(t, p.Data.Indices, tt.wantIdxLen)
			}
		})
	}

	t.Run("nil_mesh", func(t *testing.T) {
		scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
		require.NoError(t, process.Apply(scene, process.PPRemoveDegenerates, process.Options{}))
	})
}
