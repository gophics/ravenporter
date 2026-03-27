package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeboneTable(t *testing.T) {
	tests := []struct {
		name           string
		scene          *ir.Asset
		threshold      float32
		wantJointCount int
	}{
		{
			name: "no skeleton",
			scene: &ir.Asset{Meshes: []*ir.Mesh{{
				Primitives: []ir.Primitive{{Data: ir.MeshData{
					VertexCount: 1, Positions: [][3]float32{{0, 0, 0}},
				}}},
			}}},
			wantJointCount: -1,
		},
		{
			name: "all bones significant",
			scene: &ir.Asset{
				Nodes: []ir.Node{{Name: "root", Children: []int{1}}, {Name: "arm"}},
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
						Joints0:     [][4]uint16{{0, 1, 0, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}},
						Weights0:    [][4]float32{{0.8, 0.2, 0, 0}, {0.7, 0.3, 0, 0}, {0.6, 0.4, 0, 0}},
					}}},
				}},
				Skeletons: []*ir.Skeleton{{
					Joints:              []int{0, 1},
					InverseBindMatrices: [][16]float32{{}, {}},
				}},
			},
			threshold:      0.01,
			wantJointCount: 2,
		},
		{
			name: "remove negligible bone",
			scene: &ir.Asset{
				Nodes: []ir.Node{{Name: "root", Children: []int{1, 2}}, {Name: "finger1"}, {Name: "finger2"}},
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{Data: ir.MeshData{
						VertexCount: 3,
						Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
						Joints0:     [][4]uint16{{0, 1, 2, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}},
						Weights0:    [][4]float32{{0.98, 0.01, 0.01, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}},
					}}},
				}},
				Skeletons: []*ir.Skeleton{{
					Joints:              []int{0, 1, 2},
					InverseBindMatrices: [][16]float32{{}, {}, {}},
				}},
			},
			threshold:      0.05,
			wantJointCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := process.Options{DeboneThreshold: tt.threshold}
			require.NoError(t, process.Apply(tt.scene, process.PPDebone, opts))
			if tt.wantJointCount >= 0 && len(tt.scene.Skeletons) > 0 {
				assert.Equal(t, tt.wantJointCount, len(tt.scene.Skeletons[0].Joints))
			}
		})
	}
}

func TestDeboneNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPDebone, process.Options{}))
}

func TestDeboneEmptyScene(t *testing.T) {
	scene := &ir.Asset{}
	require.NoError(t, process.Apply(scene, process.PPDebone, process.Options{}))
}

func TestDeboneNilSkeleton(t *testing.T) {
	scene := &ir.Asset{Skeletons: []*ir.Skeleton{nil}}
	require.NoError(t, process.Apply(scene, process.PPDebone, process.Options{}))
}
