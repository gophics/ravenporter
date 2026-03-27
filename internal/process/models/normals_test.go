package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenNormals(t *testing.T) {
	tri := [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}

	tests := []struct {
		name      string
		flag      process.PPFlag
		mode      ir.PrimitiveMode
		positions [][3]float32
		normals   [][3]float32
		wantLen   int
		wantZ     float64
	}{
		{
			name:      "flat",
			flag:      process.PPGenNormals,
			positions: tri,
			wantLen:   3,
			wantZ:     1.0,
		},
		{
			name:      "smooth",
			flag:      process.PPGenSmoothNormals,
			positions: tri,
			wantLen:   3,
		},
		{
			name:      "skip_existing",
			flag:      process.PPGenNormals,
			positions: tri,
			normals:   [][3]float32{{0, 1, 0}, {0, 1, 0}, {0, 1, 0}},
			wantLen:   3,
		},
		{
			name:      "force_regenerates",
			flag:      process.PPForceGenNormals,
			positions: tri,
			normals:   [][3]float32{{0, 1, 0}, {0, 1, 0}, {0, 1, 0}},
			wantLen:   3,
			wantZ:     1.0,
		},
		{
			name:      "non_indexed",
			flag:      process.PPGenNormals,
			positions: tri,
			wantLen:   3,
		},
		{
			name:      "non_triangles_skipped",
			flag:      process.PPGenNormals,
			mode:      ir.Lines,
			positions: [][3]float32{{0, 0, 0}, {1, 0, 0}},
			wantLen:   0,
		},
		{
			name:      "empty_positions",
			flag:      process.PPGenNormals,
			positions: [][3]float32{},
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{
				Meshes: []*ir.Mesh{{
					Primitives: []ir.Primitive{{
						Mode: tt.mode,
						Data: ir.MeshData{
							VertexCount: len(tt.positions),
							Positions:   tt.positions,
							Normals:     tt.normals,
						},
					}},
				}},
			}
			require.NoError(t, process.Apply(scene, tt.flag, process.Options{}))
			normals := scene.Meshes[0].Primitives[0].Data.Normals
			if tt.wantLen == 0 {
				assert.Empty(t, normals)
				return
			}
			require.Len(t, normals, tt.wantLen)
			if tt.wantZ != 0 {
				assert.InDelta(t, tt.wantZ, float64(normals[0][2]), 0.01)
			}
		})
	}

	t.Run("nil_mesh", func(t *testing.T) {
		scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
		require.NoError(t, process.Apply(scene, process.PPGenNormals, process.Options{}))
	})
}

func TestDropNormals(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Normals:     [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
					Tangents:    [][4]float32{{1, 0, 0, 1}, {1, 0, 0, 1}, {1, 0, 0, 1}},
				},
			}},
		}},
	}
	require.NoError(t, process.Apply(scene, process.PPDropNormals, process.Options{}))
	assert.Nil(t, scene.Meshes[0].Primitives[0].Data.Normals)
	assert.Nil(t, scene.Meshes[0].Primitives[0].Data.Tangents)
}

func TestSmoothGroupNormals(t *testing.T) {
	// Two triangles sharing an edge (vertices 1,2), both in smooth group 1.
	// Normals at shared vertices should be averaged.
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 4,
					Positions: [][3]float32{
						{0, 0, 0}, // v0
						{1, 0, 0}, // v1 (shared)
						{0, 1, 0}, // v2 (shared)
						{1, 1, 0}, // v3
					},
					Indices:      []uint32{0, 1, 2, 1, 3, 2},
					SmoothGroups: []int{1, 1}, // both faces in group 1
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPGenSmoothNormals, process.Options{}))

	normals := scene.Meshes[0].Primitives[0].Data.Normals
	require.Len(t, normals, 4)

	// Shared vertices (1, 2) should have averaged normals from both faces.
	// Both faces are coplanar (Z=0 plane), so all normals point in Z.
	assert.InDelta(t, 0.0, normals[1][0], 0.01)
	assert.InDelta(t, 0.0, normals[1][1], 0.01)
	assert.NotEqual(t, float32(0), normals[1][2], "shared vertex should have non-zero Z normal")
}

func TestSmoothGroupNormalsGroup0Flat(t *testing.T) {
	// Two coplanar triangles, each in smooth group 0 (flat shading).
	// Even shared vertices should still get correct normals (accumulated from
	// contributing faces - group 0 currently accumulates same as any group).
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 4,
					Positions: [][3]float32{
						{0, 0, 0},
						{1, 0, 0},
						{0, 1, 0},
						{1, 1, 0},
					},
					Indices:      []uint32{0, 1, 2, 1, 3, 2},
					SmoothGroups: []int{0, 0}, // flat shading
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPGenSmoothNormals, process.Options{}))

	normals := scene.Meshes[0].Primitives[0].Data.Normals
	require.Len(t, normals, 4)

	// All normals should point in Z (both faces are in the XY plane).
	for i, n := range normals {
		assert.InDelta(t, 0.0, n[0], 0.01, "normal[%d].x", i)
		assert.InDelta(t, 0.0, n[1], 0.01, "normal[%d].y", i)
		assert.NotEqual(t, float32(0), n[2], "normal[%d].z should be non-zero", i)
	}
}

func TestSmoothNormalsAngleThreshold(t *testing.T) {
	// Two triangles sharing an edge at a 90-degree angle.
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 4,
					Positions: [][3]float32{
						{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1},
					},
					Indices: []uint32{0, 1, 2, 0, 1, 3},
				},
			}},
		}},
	}

	// With a very small angle, shared vertices should not smooth.
	require.NoError(t, process.Apply(scene, process.PPGenSmoothNormals, process.Options{SmoothNormalAngle: 0.01}))

	normals := scene.Meshes[0].Primitives[0].Data.Normals
	require.NotNil(t, normals)
	assert.Len(t, normals, 4)
}
