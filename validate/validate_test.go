package validate_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/validate"
)

func TestSceneNil(t *testing.T) {
	r := validate.Asset(nil)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeNilAsset, r.Errors[0].Code)
}

func TestStructuralNilMesh(t *testing.T) {
	s := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	r := validate.Structural(s)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeNilMesh, r.Errors[0].Code)
}

func TestStructuralIndexOutOfBounds(t *testing.T) {
	s := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Indices:     []uint32{0, 1, 99}, // 99 out of bounds
				},
			}},
		}},
	}
	r := validate.Structural(s)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeIndexOutOfBounds, r.Errors[0].Code)
}

func TestStructuralNaNPosition(t *testing.T) {
	nan := float32(math.NaN())
	s := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Positions:   [][3]float32{{nan, 0, 0}},
				},
			}},
		}},
	}
	r := validate.Structural(s)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeNaNInfPosition, r.Errors[0].Code)
}

func TestCycleDetection(t *testing.T) {
	s := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "A", Children: []int{1}, MeshIndex: -1, SkinIndex: -1, CameraIndex: -1, LightIndex: -1, LODGroupIndex: -1},
			{Name: "B", Children: []int{0}, MeshIndex: -1, SkinIndex: -1, CameraIndex: -1, LightIndex: -1, LODGroupIndex: -1}, // cycle: A→B→A
		},
		RootNodes: []int{0},
	}
	r := validate.Structural(s)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeCyclicGraph, r.Errors[0].Code)
}

func TestCycleDetectionNoCycle(t *testing.T) {
	s := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "root", Children: []int{1, 2}, MeshIndex: -1, SkinIndex: -1, CameraIndex: -1, LightIndex: -1, LODGroupIndex: -1},
			{Name: "a", MeshIndex: -1, SkinIndex: -1, CameraIndex: -1, LightIndex: -1, LODGroupIndex: -1},
			{Name: "b", Children: []int{3}, MeshIndex: -1, SkinIndex: -1, CameraIndex: -1, LightIndex: -1, LODGroupIndex: -1},
			{Name: "c", MeshIndex: -1, SkinIndex: -1, CameraIndex: -1, LightIndex: -1, LODGroupIndex: -1},
		},
		RootNodes: []int{0},
	}
	r := validate.Structural(s)
	if !r.OK() {
		t.Log(r.Errors)
	}
	assert.True(t, r.OK())
}

func TestSemanticOrphanMaterial(t *testing.T) {
	s := &ir.Asset{
		Meshes:    []*ir.Mesh{{Primitives: []ir.Primitive{{MaterialIndex: 0}}}},
		Materials: []*ir.Material{{Name: "used"}, {Name: "orphan"}},
	}
	r := validate.Semantic(s)
	require.Len(t, r.Warnings, 1)
	assert.Equal(t, validate.CodeOrphanMaterial, r.Warnings[0].Code)
}

func TestSemanticPBROutOfRange(t *testing.T) {
	s := &ir.Asset{
		Meshes: []*ir.Mesh{{Primitives: []ir.Primitive{{MaterialIndex: 0}}}},
		Materials: []*ir.Material{{
			Name:            "bad",
			MetallicFactor:  1.5,
			RoughnessFactor: -0.1,
		}},
	}
	r := validate.Semantic(s)
	assert.Len(t, r.Warnings, 2) // one for metallic, one for roughness
}

func TestSemanticTextureRefBounds(t *testing.T) {
	s := &ir.Asset{
		Materials: []*ir.Material{{
			Name:             "mat",
			BaseColorTexture: &ir.TextureRef{TextureIndex: 5}, // out of range
		}},
		Textures: []*ir.Texture{{Name: "tex0"}}, // only 1 texture
	}
	r := validate.Semantic(s)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeTextureRefBounds, r.Errors[0].Code)
}

func TestSemanticAnimationNaN(t *testing.T) {
	nan := float32(math.NaN())
	s := &ir.Asset{
		Animations: []*ir.Animation{{
			Name: "walk",
			Channels: []ir.AnimationChannel{{
				Times: []float32{0, nan, 1.0},
			}},
		}},
	}
	r := validate.Semantic(s)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeAnimationNaN, r.Errors[0].Code)
}

func TestSceneBackwardCompatible(t *testing.T) {
	s := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Indices:     []uint32{0, 1, 2},
				},
			}},
		}},
		Materials: []*ir.Material{{Name: "mat", MetallicFactor: 0.5, RoughnessFactor: 0.5}},
	}
	r := validate.Asset(s)
	assert.True(t, r.OK())
}

func TestStructuralNodeRefBounds(t *testing.T) {
	s := &ir.Asset{
		Nodes: []ir.Node{{
			MeshIndex:     1,
			CameraIndex:   1,
			SkinIndex:     1,
			LODGroupIndex: 1,
			LightIndex:    -1,
		}},
		Meshes:          []*ir.Mesh{{Name: "mesh"}},
		Cameras:         []*ir.Camera{{Name: "cam"}},
		Skeletons:       []*ir.Skeleton{{Name: "rig"}},
		LODGroups:       []*ir.LODGroup{{Name: "lod"}},
		CollisionMeshes: []*ir.CollisionMesh{{MeshIndex: 2}},
	}

	r := validate.Structural(s)
	require.False(t, r.OK())
	assert.Equal(t, validate.CodeNodeRefBounds, r.Errors[0].Code)
}

func TestSemanticDegenerateTrianglesAndMaterialExtRanges(t *testing.T) {
	s := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{Indices: []uint32{0, 0, 1}},
			}},
		}},
		Materials: []*ir.Material{{
			Name: "ext",
			Clearcoat: &ir.MaterialClearcoat{
				Factor:          2,
				RoughnessFactor: -1,
			},
			Sheen:        &ir.MaterialSheen{RoughnessFactor: 2},
			Transmission: &ir.MaterialTransmission{Factor: -1},
			Iridescence:  &ir.MaterialIridescence{Factor: 3},
		}},
	}

	r := validate.Semantic(s)
	assert.NotEmpty(t, r.Warnings)
	codes := make([]string, 0, len(r.Warnings))
	for _, warning := range r.Warnings {
		codes = append(codes, warning.Code)
	}
	assert.Contains(t, codes, validate.CodeDegenerateTriangle)
	assert.Contains(t, codes, validate.CodePBROutOfRange)
}
