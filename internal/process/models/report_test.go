package models_test

import (
	"log/slog"
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/require"
)

func TestReportStatsTable(t *testing.T) {
	tests := []struct {
		name  string
		scene *ir.Asset
	}{
		{
			name:  "empty scene",
			scene: &ir.Asset{},
		},
		{
			name:  "nil meshes",
			scene: &ir.Asset{Meshes: []*ir.Mesh{nil}},
		},
		{
			name: "populated scene",
			scene: &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Data: ir.MeshData{
							VertexCount: 3,
							Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
							Indices:     []uint32{0, 1, 2},
						},
					}},
				}},
				Materials:  []*ir.Material{{}},
				Textures:   []*ir.Texture{{}},
				Nodes:      []ir.Node{{}},
				Animations: []*ir.Animation{{}},
				Skeletons:  []*ir.Skeleton{{}},
			},
		},
		{
			name: "multiple meshes",
			scene: &ir.Asset{
				Meshes: []*ir.Mesh{
					{Primitives: []ir.Primitive{{Data: ir.MeshData{VertexCount: 10}}}},
					{Primitives: []ir.Primitive{{Data: ir.MeshData{VertexCount: 20}}}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := process.Options{Logger: slog.Default()}
			require.NoError(t, process.Apply(tt.scene, process.PPReportStats, opts))
		})
	}
}

func TestReportStatsNilLogger(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{{
		Primitives: []ir.Primitive{{Data: ir.MeshData{VertexCount: 1}}},
	}}}
	require.NoError(t, process.Apply(scene, process.PPReportStats, process.Options{}))
}
