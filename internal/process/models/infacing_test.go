package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixInfacingNormalsTable(t *testing.T) {
	tests := []struct {
		name      string
		positions [][3]float32
		normals   [][3]float32
	}{
		{
			name:      "inward facing flipped",
			positions: [][3]float32{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}},
			normals:   [][3]float32{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}},
		},
		{
			name:      "outward facing unchanged",
			positions: [][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
			normals:   [][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
		},
		{
			name:      "single vertex",
			positions: [][3]float32{{0, 0, 0}},
			normals:   [][3]float32{{0, 0, 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Data: ir.MeshData{
							VertexCount: len(tt.positions),
							Positions:   tt.positions,
							Normals:     tt.normals,
						},
					}},
				}},
			}
			require.NoError(t, process.Apply(scene, process.PPFixInfacingNormals, process.Options{}))
			assert.NotNil(t, scene.Meshes[0].Primitives[0].Data.Normals)
		})
	}
}

func TestFixInfacingEmptyPositions(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 0},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPFixInfacingNormals, process.Options{}))
}

func TestFixInfacingMismatchedLengths(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Normals:     [][3]float32{{0, 0, 1}},
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPFixInfacingNormals, process.Options{}))
}

func TestFixInfacingNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPFixInfacingNormals, process.Options{}))
}
