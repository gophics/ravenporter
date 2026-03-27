package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalcTangentSpace(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Normals:     [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
					TexCoord0:   [][2]float32{{0, 0}, {1, 0}, {0, 1}},
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPCalcTangentSpace, process.Options{}))

	p := &scene.Meshes[0].Primitives[0]
	require.NotNil(t, p.Data.Tangents)
	assert.Len(t, p.Data.Tangents, 3)

	// Tangent should be approximately (1,0,0) for this UV layout.
	assert.InDelta(t, 1.0, p.Data.Tangents[0][0], 0.01)
	assert.InDelta(t, 0.0, p.Data.Tangents[0][1], 0.01)
	assert.InDelta(t, 0.0, p.Data.Tangents[0][2], 0.01)
	// Handedness: w should be +1 or -1.
	assert.True(t, p.Data.Tangents[0][3] == 1.0 || p.Data.Tangents[0][3] == -1.0)
}

func TestCalcTangentSpaceSkipsExisting(t *testing.T) {
	existing := [][4]float32{{1, 0, 0, 1}}
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Tangents:    existing,
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPCalcTangentSpace, process.Options{}))
	assert.Equal(t, existing, scene.Meshes[0].Primitives[0].Data.Tangents)
}

func TestCalcTangentEdgeCases(t *testing.T) {
	tri := [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	nrm := [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}}
	uv := [][2]float32{{0, 0}, {1, 0}, {0, 1}}

	tests := []struct {
		name    string
		mode    ir.PrimitiveMode
		pos     [][3]float32
		normals [][3]float32
		uvs     [][2]float32
		wantLen int
	}{
		{name: "non_triangles", mode: ir.Lines, pos: tri[:2], normals: nrm[:2], uvs: uv[:2], wantLen: 0},
		{name: "missing_normals", pos: tri, uvs: uv, wantLen: 0},
		{name: "missing_uvs", pos: tri, normals: nrm, wantLen: 0},
		{name: "non_indexed", pos: tri, normals: nrm, uvs: uv, wantLen: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Mode: tt.mode,
						Data: ir.MeshData{VertexCount: len(tt.pos), Positions: tt.pos, Normals: tt.normals, TexCoord0: tt.uvs},
					}},
				}},
			}
			require.NoError(t, process.Apply(scene, process.PPCalcTangentSpace, process.Options{}))
			if tt.wantLen == 0 {
				assert.Empty(t, scene.Meshes[0].Primitives[0].Data.Tangents)
			} else {
				assert.Len(t, scene.Meshes[0].Primitives[0].Data.Tangents, tt.wantLen)
			}
		})
	}
}
