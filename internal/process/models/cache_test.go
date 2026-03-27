package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimizeCacheTable(t *testing.T) {
	tests := []struct {
		name       string
		vertCount  int
		indices    []uint32
		wantLen    int
		wantAllIdx bool
	}{
		{
			name:       "two triangles",
			vertCount:  6,
			indices:    []uint32{0, 1, 2, 3, 4, 5},
			wantLen:    6,
			wantAllIdx: true,
		},
		{
			name:       "single triangle",
			vertCount:  3,
			indices:    []uint32{0, 1, 2},
			wantLen:    3,
			wantAllIdx: true,
		},
		{
			name:       "shared vertices",
			vertCount:  4,
			indices:    []uint32{0, 1, 2, 1, 2, 3},
			wantLen:    6,
			wantAllIdx: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Data: ir.MeshData{VertexCount: tt.vertCount, Indices: tt.indices},
					}},
				}},
			}
			require.NoError(t, process.Apply(scene, process.PPOptimizeCache, process.Options{}))
			got := scene.Meshes[0].Primitives[0].Data.Indices
			assert.Len(t, got, tt.wantLen)
			if tt.wantAllIdx {
				seen := make(map[uint32]bool)
				for _, idx := range got {
					seen[idx] = true
				}
				for _, idx := range tt.indices {
					assert.True(t, seen[idx], "missing index %d", idx)
				}
			}
		})
	}
}

func TestOptimizeCacheSkipsShort(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 2, Indices: []uint32{0, 1}},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPOptimizeCache, process.Options{}))
	assert.Equal(t, []uint32{0, 1}, scene.Meshes[0].Primitives[0].Data.Indices)
}

func TestOptimizeCacheNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPOptimizeCache, process.Options{}))
}

func TestOptimizeCacheEmptyIndices(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 0},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPOptimizeCache, process.Options{}))
}
