package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriangulate(t *testing.T) {
	tests := []struct {
		name       string
		mode       ir.PrimitiveMode
		verts      int
		positions  [][3]float32
		indices    []uint32
		wantMode   ir.PrimitiveMode
		wantIdxLen int
		wantIdx    []uint32
	}{
		{
			name:       "fan_to_triangles",
			mode:       ir.TriangleFan,
			verts:      4,
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0}},
			indices:    []uint32{0, 1, 2, 3},
			wantMode:   ir.Triangles,
			wantIdxLen: 6,
		},
		{
			name:       "already_triangles",
			mode:       ir.Triangles,
			verts:      3,
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			indices:    []uint32{0, 1, 2},
			wantMode:   ir.Triangles,
			wantIdxLen: 3,
		},
		{
			name:       "strip_to_triangles",
			mode:       ir.TriangleStrip,
			verts:      5,
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}, {0, 2, 0}},
			indices:    []uint32{0, 1, 2, 3, 4},
			wantMode:   ir.Triangles,
			wantIdxLen: 9,
			wantIdx:    []uint32{0, 1, 2, 1, 3, 2, 2, 3, 4},
		},
		{
			name:       "strip_too_few",
			mode:       ir.TriangleStrip,
			verts:      2,
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}},
			indices:    []uint32{0, 1},
			wantMode:   ir.TriangleStrip,
			wantIdxLen: 2,
		},
		{
			name:       "line_loop_passthrough",
			mode:       ir.LineLoop,
			verts:      4,
			positions:  [][3]float32{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0}},
			indices:    []uint32{0, 1, 2, 3},
			wantMode:   ir.LineLoop,
			wantIdxLen: 4,
			wantIdx:    []uint32{0, 1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Mode: tt.mode,
						Data: ir.MeshData{VertexCount: tt.verts, Positions: tt.positions, Indices: tt.indices},
					}},
				}},
			}
			require.NoError(t, process.Apply(scene, process.PPTriangulate, process.Options{}))
			p := &scene.Meshes[0].Primitives[0]
			assert.Equal(t, tt.wantMode, p.Mode)
			assert.Len(t, p.Data.Indices, tt.wantIdxLen)
			if tt.wantIdx != nil {
				assert.Equal(t, tt.wantIdx, p.Data.Indices)
			}
		})
	}
}
