package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitByBoneCountTable(t *testing.T) {
	tests := []struct {
		name      string
		joints    [][4]uint16
		weights   [][4]float32
		indices   []uint32
		maxBones  int
		wantSplit bool
	}{
		{
			name:     "under limit no split",
			joints:   [][4]uint16{{0, 0, 0, 0}, {1, 0, 0, 0}, {0, 0, 0, 0}},
			weights:  [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}},
			indices:  []uint32{0, 1, 2},
			maxBones: 4,
		},
		{
			name: "over limit splits",
			joints: [][4]uint16{
				{0, 0, 0, 0}, {1, 0, 0, 0}, {2, 0, 0, 0},
				{3, 0, 0, 0}, {4, 0, 0, 0}, {5, 0, 0, 0},
			},
			weights: [][4]float32{
				{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0},
				{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0},
			},
			indices:   []uint32{0, 1, 2, 3, 4, 5},
			maxBones:  2,
			wantSplit: true,
		},
		{
			name:     "default max bones",
			joints:   [][4]uint16{{0, 0, 0, 0}, {1, 0, 0, 0}, {2, 0, 0, 0}},
			weights:  [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}},
			indices:  []uint32{0, 1, 2},
			maxBones: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			positions := make([][3]float32, len(tt.joints))
			for i := range positions {
				positions[i] = [3]float32{float32(i), 0, 0}
			}
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Mode: ir.Triangles,
						Data: ir.MeshData{
							VertexCount: len(positions),
							Positions:   positions,
							Indices:     tt.indices,
							Joints0:     tt.joints,
							Weights0:    tt.weights,
						},
					}},
				}},
			}
			opts := process.Options{MaxBonesPerMesh: tt.maxBones}
			require.NoError(t, process.Apply(scene, process.PPSplitByBoneCount, opts))
			if tt.wantSplit {
				assert.Greater(t, len(scene.Meshes[0].Primitives), 1)
			} else {
				assert.Len(t, scene.Meshes[0].Primitives, 1)
			}
		})
	}
}

func TestSplitByBoneCountNoBones(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPSplitByBoneCount, process.Options{MaxBonesPerMesh: 2}))
	assert.Len(t, scene.Meshes[0].Primitives, 1)
}

func TestSplitByBoneCountNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPSplitByBoneCount, process.Options{}))
}

func TestSplitByBoneCountNonTriangles(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Lines,
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Joints0:     [][4]uint16{{0, 1, 2, 3}, {4, 5, 6, 7}, {8, 9, 10, 11}},
					Weights0:    [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}},
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPSplitByBoneCount, process.Options{MaxBonesPerMesh: 2}))
	assert.Len(t, scene.Meshes[0].Primitives, 1)
}

func TestSplitByBoneCountPreservesAttributes(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					VertexCount: 6,
					Positions: [][3]float32{
						{0, 0, 0}, {1, 0, 0}, {0, 1, 0},
						{2, 0, 0}, {3, 0, 0}, {2, 1, 0},
					},
					Normals: [][3]float32{
						{0, 0, 1}, {0, 0, 1}, {0, 0, 1},
						{0, 0, 1}, {0, 0, 1}, {0, 0, 1},
					},
					Joints0: [][4]uint16{
						{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0},
						{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0},
					},
					Weights0: [][4]float32{
						{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0},
						{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0},
					},
					Indices: []uint32{0, 1, 2, 3, 4, 5},
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPSplitByBoneCount, process.Options{MaxBonesPerMesh: 1}))
	for _, p := range scene.Meshes[0].Primitives {
		assert.Equal(t, len(p.Data.Positions), len(p.Data.Normals))
		assert.Equal(t, len(p.Data.Positions), len(p.Data.Joints0))
		assert.Equal(t, len(p.Data.Positions), len(p.Data.Weights0))
	}
}
