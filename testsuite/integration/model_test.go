//go:build integration

package integration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/testsuite/corpus"
)

func TestIntegration_Model(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectedFmt ir.FormatID
		verifyFn    func(t *testing.T, asset *ir.Asset)
	}{
		// glTF2
		{"GLTF2_BoxTextured", corpus.ModelGLTF2BoxTextured, ir.FormatGLTF, func(t *testing.T, asset *ir.Asset) {
			scene := asset.PrimaryScene()
			require.NotNil(t, scene)
			assert.Equal(t, "2.0", asset.Metadata.SourceVersion)
			assert.NotEmpty(t, asset.Metadata.Generator, "glTF generator must be parsed")
			assert.True(t, len(scene.RootNodes) > 0, "root nodes must be populated")
			assert.Equal(t, ir.YUp, asset.UpAxis)

			require.True(t, len(asset.Meshes) >= 1)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			assert.Equal(t, 24, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Indices, 36)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount, "positions length must match vertex count")
			assert.True(t, prim.Data.HasUVs(), "BoxTextured must have UVs")

			require.True(t, len(asset.Materials) >= 1)
			mat := asset.Materials[0]
			assert.NotEqual(t, [4]float32{}, mat.BaseColorFactor, "base color factor must be set")
			assert.NotNil(t, mat.BaseColorTexture, "BoxTextured must have a base color texture")
			assert.True(t, mat.MetallicFactor >= 0, "metallic factor must be set")
			assert.True(t, mat.RoughnessFactor >= 0, "roughness factor must be set")

			require.True(t, len(asset.Textures) >= 1)
			tex := asset.Textures[0]
			require.GreaterOrEqual(t, tex.ImageIndex, 0)
			require.Less(t, tex.ImageIndex, len(asset.Images))
			assert.Equal(t, ir.ImagePNG, asset.Images[tex.ImageIndex].Format)
			assert.NotEmpty(t, asset.Images[tex.ImageIndex].Compressed)
			t.Logf("BoxTextured texture: wrapS=%v wrapT=%v", tex.WrapS, tex.WrapT)
		}},
		{"GLTF2_2CylinderEngine", corpus.ModelGLTF2CylinderEngine, ir.FormatGLTF, func(t *testing.T, asset *ir.Asset) {
			assert.True(t, len(asset.Meshes) >= 5)
			assert.True(t, len(asset.Materials) >= 2)
			assert.True(t, len(asset.Nodes) >= 5)

			hasChildren := false
			hasTransform := false
			for _, n := range asset.Nodes {
				if len(n.Children) > 0 {
					hasChildren = true
				}
				t := n.Transform
				if t.Translation != [3]float32{} || t.Rotation != [4]float32{} || t.Scale != [3]float32{} || t.Matrix != [16]float32{} {
					hasTransform = true
				}
			}
			assert.True(t, hasChildren, "scene must have hierarchical nodes")
			assert.True(t, hasTransform, "at least one node must have a non-identity transform")

			for i, n := range asset.Nodes {
				if n.MeshIndex != ir.NoIndex {
					assert.True(t, n.MeshIndex < len(asset.Meshes), "node %d mesh index out of bounds", i)
				}
				if n.SkinIndex != ir.NoIndex {
					assert.True(t, n.SkinIndex < len(asset.Skeletons), "node %d skin index out of bounds", i)
					skel := asset.Skeletons[n.SkinIndex]
					assert.True(t, len(skel.Joints) > 0, "skeleton %d must have joints", n.SkinIndex)
					if len(skel.InverseBindMatrices) > 0 {
						assert.Len(t, skel.InverseBindMatrices, len(skel.Joints), "IBM must match joints length")
					}
				}
			}
		}},
		{"GLTF2_Avocado", corpus.ModelGLTF2Avocado, ir.FormatGLTF, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			prim := asset.Meshes[0].Primitives[0]
			assert.True(t, prim.Data.VertexCount > 0)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
			assert.True(t, prim.Data.HasTangents(), "Avocado must have tangents")

			require.True(t, len(asset.Materials) >= 1)
			mat := asset.Materials[0]
			assert.True(t, mat.MetallicFactor >= 0, "metallic factor must be set")
			assert.True(t, mat.RoughnessFactor >= 0, "roughness factor must be set")
			if mat.NormalTexture != nil {
				assert.True(t, mat.NormalTexture.TextureIndex >= 0, "normal texture index must be valid")
			}
			assert.True(t, len(asset.Textures) >= 1)
		}},
		{"GLTF2_MeshoptIndices", corpus.IsoModelMeshoptGLTF, ir.FormatGLTF, func(t *testing.T, asset *ir.Asset) {
			require.Len(t, asset.Meshes, 1)
			require.Len(t, asset.Meshes[0].Primitives, 1)

			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			assert.Equal(t, 10, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Positions, 10)
			assert.Len(t, prim.Data.Indices, 12)
			for _, index := range prim.Data.Indices {
				assert.Less(t, int(index), prim.Data.VertexCount)
			}

			lo, hi := asset.SceneBoundingBox(0)
			assert.Equal(t, [3]float32{0, 0, 0}, lo)
			assert.Equal(t, [3]float32{9, 2, 0}, hi)
		}},

		// FBX
		{"FBX_Box", corpus.ModelFBXBox, ir.FormatFBX, func(t *testing.T, asset *ir.Asset) {
			assert.NotEmpty(t, asset.Metadata.SourceVersion, "FBX version must be parsed")
			require.True(t, len(asset.Meshes) >= 1)
			prim := asset.Meshes[0].Primitives[0]
			assert.True(t, prim.Data.VertexCount > 0)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
			assert.True(t, len(prim.Data.Indices) > 0)
		}},
		{"FBX_PhongCube", corpus.ModelFBXPhongCube, ir.FormatFBX, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Materials) >= 1)
			mat := asset.Materials[0]
			assert.NotEmpty(t, mat.Name)
			assert.Equal(t, ir.AlphaOpaque, mat.AlphaMode)
			assert.True(t, mat.RoughnessFactor >= 0, "roughness factor must be set")

			if len(asset.Cameras) > 0 {
				cam := asset.Cameras[0]
				assert.NotEmpty(t, cam.Name, "camera name must be set")
				assert.True(t, cam.Perspective != nil || cam.Orthographic != nil, "camera must have projection")
			}
			if len(asset.Lights) > 0 {
				light := asset.Lights[0]
				assert.NotEmpty(t, light.Name, "light name must be set")
			}
		}},

		// OBJ
		{"OBJ_Cube", corpus.ModelOBJCube, ir.FormatOBJ, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			totalVerts := 0
			for _, m := range asset.Meshes {
				for _, p := range m.Primitives {
					totalVerts += p.Data.VertexCount
				}
			}
			assert.True(t, totalVerts >= 8)
			if len(asset.Materials) > 0 {
				assert.NotEmpty(t, asset.Materials[0].Name, "OBJ material name must be parsed")
			}
		}},
		{"OBJ_Bunny", corpus.ModelOBJBunny, ir.FormatOBJ, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 1000)
		}},

		// Collada (DAE)
		{"DAE_Duck", corpus.ModelDAEDuck, ir.FormatDAE, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			require.True(t, len(asset.Nodes) >= 1)

			if len(asset.Cameras) > 0 {
				cam := asset.Cameras[0]
				assert.NotEmpty(t, cam.Name, "camera name must be set")
				assert.True(t, cam.Perspective != nil || cam.Orthographic != nil, "camera must have projection")
			}
			if len(asset.Lights) > 0 {
				light := asset.Lights[0]
				assert.NotEmpty(t, light.Name, "light name must be set")
			}
		}},
		{"DAE_Animated", corpus.ModelDAEAnimated, ir.FormatDAE, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1 || len(asset.Animations) >= 1)
			if len(asset.Animations) > 0 {
				anim := asset.Animations[0]
				assert.True(t, len(anim.Channels) >= 1, "animation must have channels")
				assert.True(t, anim.Duration > 0, "DAE animation duration must be set")
				if len(anim.Channels) > 0 {
					ch := anim.Channels[0]
					assert.True(t, len(ch.Times) > 0, "keyframe timestamps must be populated")
					assert.NotZero(t, ch.Target, "DAE channel target must be set")
				}
			}
		}},

		// STL
		{"STL_ASCII", corpus.ModelSTLASCII, ir.FormatSTL, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 0)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.HasNormals())
		}},
		{"STL_Binary", corpus.ModelSTLBinary, ir.FormatSTL, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount >= 100)
			assert.True(t, len(asset.Meshes[0].Primitives[0].Data.Normals) > 0)
		}},

		// PLY
		{"PLY_CubeASCII", corpus.ModelPLYCubeASCII, ir.FormatPLY, func(t *testing.T, asset *ir.Asset) {
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount >= 8)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.HasIndices())
		}},
		{"PLY_CubeBinary", corpus.ModelPLYCubeBinary, ir.FormatPLY, func(t *testing.T, asset *ir.Asset) {
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount >= 8)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.HasIndices())
		}},

		// BVH
		{"BVH_MoCap", corpus.ModelBVHMoCap, ir.FormatBVH, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Skeletons) >= 1)
			skel := asset.Skeletons[0]
			assert.True(t, len(skel.Joints) >= 5)
			assert.True(t, skel.RootIdx >= 0, "root joint index must be valid")
			if len(skel.InverseBindMatrices) > 0 {
				assert.Len(t, skel.InverseBindMatrices, len(skel.Joints), "IBM count must match joint count")
			}

			require.True(t, len(asset.Animations) >= 1)
			anim := asset.Animations[0]
			assert.True(t, len(anim.Channels) >= 2)
			assert.True(t, anim.Duration > 0, "BVH animation duration must be set")

			ch := anim.Channels[0]
			assert.True(t, len(ch.Times) > 0, "keyframe timestamps must be populated")
			hasData := len(ch.Translations) > 0 || len(ch.Rotations) > 0 || len(ch.Scales) > 0
			assert.True(t, hasData, "channel must have transform data")
			t.Logf("BVH channel: target=%v interp=%v", ch.Target, ch.Interpolation)
		}},

		// 3DS
		{"3DS_Cube", corpus.Model3DSCube, ir.Format3DS, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 0)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.HasIndices())
		}},
		{"3DS_Variant", corpus.Model3DSVariant, ir.Format3DS, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 0)
		}},

		// 3MF
		{"3MF_Box", corpus.Model3MFBox, ir.Format3MF, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 0)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.HasIndices())
		}},

		// USD
		{"USDA_Comprehensive", corpus.ModelUSDAComprehensive, ir.FormatUSD, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 0)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.HasIndices())
		}},
		{"USDC_Comprehensive", corpus.ModelUSDCComprehensive, ir.FormatUSD, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 0)
		}},

		// ABC (Alembic)
		{"ABC_Cube", corpus.ModelABCCube, ir.FormatAlembic, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Meshes) >= 1)
			assert.True(t, asset.Meshes[0].Primitives[0].Data.VertexCount > 0)
		}},

		// Exhaustive Isolation
		{"GLTF2_PBRExhaustive", corpus.IsoModelPBRGLTF, ir.FormatGLTF, func(t *testing.T, asset *ir.Asset) {
			require.True(t, len(asset.Materials) >= 1)
			mat := asset.Materials[0]

			require.NotNil(t, mat.Clearcoat)
			assert.Equal(t, float32(0.5), mat.Clearcoat.Factor)
			assert.Equal(t, float32(0.2), mat.Clearcoat.RoughnessFactor)

			require.NotNil(t, mat.Sheen)
			assert.Equal(t, float32(1.0), mat.Sheen.ColorFactor[0])
			assert.Equal(t, float32(0.5), mat.Sheen.ColorFactor[1])
			assert.Equal(t, float32(0.0), mat.Sheen.ColorFactor[2])
			assert.Equal(t, float32(0.8), mat.Sheen.RoughnessFactor)

			require.NotNil(t, mat.Transmission)
			assert.Equal(t, float32(0.9), mat.Transmission.Factor)

			require.NotNil(t, mat.Volume)
			assert.Equal(t, float32(2.0), mat.Volume.ThicknessFactor)

			require.NotNil(t, mat.IOR)
			assert.Equal(t, float32(1.45), mat.IOR.IOR)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := runPipeline(t, tc.path)
			if asset == nil {
				return // runPipeline internally fails the test if decoding fails
			}
			assert.Equal(t, tc.expectedFmt, asset.Metadata.SourceFormat)

			if tc.verifyFn != nil {
				tc.verifyFn(t, asset)
			}
			t.Logf("%s: successfully decoded and structured", tc.name)
		})
	}
}

func TestIntegration_Model_OBJ_NaNDegenerate(t *testing.T) {
	path := filepath.Join(corpusDir(t, corpus.IsoModelBadGeoOBJ), filepath.FromSlash(corpus.IsoModelBadGeoOBJ))
	_, err := pipeline.ImportPath(context.Background(), path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "structural validation failed")
}

func TestIntegration_Model_MemoryClamps(t *testing.T) {
	paths := []string{corpus.ModelOBJBunny, corpus.ModelPLYCubeASCII}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			path := filepath.Join(corpusDir(t, p), filepath.FromSlash(p))
			result, err := pipeline.ImportPath(context.Background(), path, pipeline.WithDecodeMaxVertices(5))
			if err == nil {
				t.Logf("Pipeline illegally returned success. Meshes=%d NodeCount=%d", len(result.Asset.Meshes), len(result.Asset.Nodes))
			}
			require.Error(t, err, "Pipeline should error due to MaxVertices limit")
			assert.Contains(t, err.Error(), "vertex limit exceeded", "error should denote limit violation")
		})
	}
}

func TestIntegration_Model_FBX_ASCII(t *testing.T) {
	asset := runPipeline(t, corpus.ModelFBXASCII)
	require.True(t, len(asset.Meshes) >= 1, "ASCII FBX must produce meshes")
	prim := asset.Meshes[0].Primitives[0]
	assert.True(t, prim.Data.VertexCount > 0, "mesh must have vertices")
	assert.True(t, len(prim.Data.Indices) > 0, "mesh must have indices")
	if prim.Data.HasNormals() {
		assert.Len(t, prim.Data.Normals, prim.Data.VertexCount, "normals must match vertex count")
	}
	t.Logf("FBX_ASCII: %d meshes, %d nodes, verts=%d",
		len(asset.Meshes), len(asset.Nodes), prim.Data.VertexCount)
}
