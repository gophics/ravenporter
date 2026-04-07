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

			require.NotEmpty(t, asset.Meshes)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			assert.Equal(t, 24, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Indices, 36)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount, "positions length must match vertex count")
			assert.True(t, prim.Data.HasUVs(), "BoxTextured must have UVs")

			require.NotEmpty(t, asset.Materials)
			mat := asset.Materials[0]
			assert.NotEqual(t, [4]float32{}, mat.BaseColorFactor, "base color factor must be set")
			assert.NotNil(t, mat.BaseColorTexture, "BoxTextured must have a base color texture")
			assert.True(t, mat.MetallicFactor >= 0, "metallic factor must be set")
			assert.True(t, mat.RoughnessFactor >= 0, "roughness factor must be set")

			require.NotEmpty(t, asset.Textures)
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
			require.NotEmpty(t, asset.Meshes)
			var prim *ir.Primitive
			for meshIdx := range asset.Meshes {
				for primIdx := range asset.Meshes[meshIdx].Primitives {
					candidate := &asset.Meshes[meshIdx].Primitives[primIdx]
					if candidate.Data.HasTangents() {
						prim = candidate
						break
					}
				}
				if prim != nil {
					break
				}
			}
			require.NotNil(t, prim, "Avocado must include a primitive with tangents")
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Normals, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Tangents, prim.Data.VertexCount)
			assert.NotEmpty(t, prim.Data.Indices)
			assert.True(t, prim.Data.HasTangents(), "Avocado must have tangents")

			require.NotEmpty(t, asset.Materials)
			mat := asset.Materials[0]
			assert.True(t, mat.MetallicFactor >= 0, "metallic factor must be set")
			assert.True(t, mat.RoughnessFactor >= 0, "roughness factor must be set")
			assert.NotNil(t, mat.BaseColorTexture, "Avocado should carry a base color texture")
			if mat.NormalTexture != nil {
				assert.True(t, mat.NormalTexture.TextureIndex >= 0, "normal texture index must be valid")
			}
			require.NotEmpty(t, asset.Textures)
			for i, tex := range asset.Textures {
				require.GreaterOrEqual(t, tex.ImageIndex, 0, "texture %d image index must be valid", i)
				require.Less(t, tex.ImageIndex, len(asset.Images), "texture %d image index out of range", i)
			}
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
			assert.Equal(t, "7400", asset.Metadata.SourceVersion)
			require.Equal(t, []int{0}, asset.RootNodes)
			require.Len(t, asset.Nodes, 1)
			assert.Equal(t, "root", asset.Nodes[0].Name)
			require.Len(t, asset.Meshes, 1)
			require.Len(t, asset.Materials, 1)
			assert.Equal(t, "mesh_id43", asset.Meshes[0].Name)
			assert.Equal(t, "Material_50", asset.Materials[0].Name)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, 36, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Indices, 36)
		}},
		{"FBX_PhongCube", corpus.ModelFBXPhongCube, ir.FormatFBX, func(t *testing.T, asset *ir.Asset) {
			require.NotEmpty(t, asset.Materials)
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
			require.NotEmpty(t, asset.Meshes)
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
			require.Equal(t, []int{0}, asset.RootNodes)
			require.Len(t, asset.Nodes, 1)
			assert.Equal(t, "default", asset.Nodes[0].Name)
			require.Len(t, asset.Meshes, 1)
			assert.Equal(t, "default", asset.Meshes[0].Name)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, 34834, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Indices, 208353)

			lo, hi := asset.SceneBoundingBox(0)
			assert.InDelta(t, -0.09469, lo[0], 1e-5)
			assert.InDelta(t, 0.032987, lo[1], 1e-5)
			assert.InDelta(t, -0.061874, lo[2], 1e-5)
			assert.InDelta(t, 0.061009, hi[0], 1e-5)
			assert.InDelta(t, 0.187321, hi[1], 1e-5)
			assert.InDelta(t, 0.0588, hi[2], 1e-5)
		}},

		// Collada (DAE)
		{"DAE_Duck", corpus.ModelDAEDuck, ir.FormatDAE, func(t *testing.T, asset *ir.Asset) {
			require.NotEmpty(t, asset.Meshes)
			require.NotEmpty(t, asset.Nodes)

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
			require.NotEmpty(t, asset.Animations)
			anim := asset.Animations[0]
			assert.True(t, len(anim.Channels) >= 1, "animation must have channels")
			assert.True(t, anim.Duration > 0, "DAE animation duration must be set")
			if len(anim.Channels) > 0 {
				ch := anim.Channels[0]
				assert.True(t, len(ch.Times) > 0, "keyframe timestamps must be populated")
				assert.NotZero(t, ch.Target, "DAE channel target must be set")
			}
		}},

		// STL
		{"STL_ASCII", corpus.ModelSTLASCII, ir.FormatSTL, func(t *testing.T, asset *ir.Asset) {
			require.Len(t, asset.Meshes, 1)
			require.Len(t, asset.Meshes[0].Primitives, 1)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Normals, prim.Data.VertexCount)
			assert.NotEmpty(t, prim.Data.Indices)
			assert.Equal(t, 0, len(prim.Data.Indices)%3)
		}},
		{"STL_Binary", corpus.ModelSTLBinary, ir.FormatSTL, func(t *testing.T, asset *ir.Asset) {
			require.Len(t, asset.Meshes, 1)
			require.Len(t, asset.Meshes[0].Primitives, 1)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Normals, prim.Data.VertexCount)
			assert.NotEmpty(t, prim.Data.Indices)
			assert.Equal(t, 0, len(prim.Data.Indices)%3)
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
			require.NotEmpty(t, asset.Skeletons)
			skel := asset.Skeletons[0]
			assert.True(t, len(skel.Joints) >= 5)
			assert.True(t, skel.RootIdx >= 0, "root joint index must be valid")
			if len(skel.InverseBindMatrices) > 0 {
				assert.Len(t, skel.InverseBindMatrices, len(skel.Joints), "IBM count must match joint count")
			}

			require.NotEmpty(t, asset.Animations)
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
			require.Equal(t, []int{0}, asset.RootNodes)
			require.Len(t, asset.Nodes, 1)
			require.Len(t, asset.Meshes, 1)
			require.Len(t, asset.Materials, 1)
			assert.Equal(t, "Default", asset.Nodes[0].Name)
			assert.Equal(t, "Default", asset.Meshes[0].Name)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, 386, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Indices, 2304)
		}},
		{"3DS_Variant", corpus.Model3DSVariant, ir.Format3DS, func(t *testing.T, asset *ir.Asset) {
			require.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}, asset.RootNodes)
			require.Len(t, asset.Nodes, 9)
			require.Len(t, asset.Meshes, 9)
			require.Len(t, asset.Materials, 3)
			require.Len(t, asset.Animations, 1)
			assert.NotEqual(t, ir.NoIndex, asset.FindNode("Box01"))
			assert.NotEqual(t, ir.NoIndex, asset.FindNode("Box02"))
			assert.NotEqual(t, ir.NoIndex, asset.FindMesh("Box01"))
			prim := asset.Meshes[asset.FindMesh("Box01")].Primitives[0]
			assert.Equal(t, 32, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Indices, 36)
			assert.Len(t, prim.Data.TexCoord0, 32)
			assert.Equal(t, "default", asset.Animations[0].Name)
			assert.Len(t, asset.Animations[0].Channels, 27)
		}},

		// 3MF
		{"3MF_Box", corpus.Model3MFBox, ir.Format3MF, func(t *testing.T, asset *ir.Asset) {
			assert.InDelta(t, 0.001, asset.Unit, 1e-9)
			require.Equal(t, []int{0}, asset.RootNodes)
			require.Len(t, asset.Meshes, 1)
			assert.Equal(t, "3mf", asset.Meshes[0].Name)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, 8, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Indices, 36)
		}},

		// USD
		{"USDA_Comprehensive", corpus.ModelUSDAComprehensive, ir.FormatUSD, func(t *testing.T, asset *ir.Asset) {
			assert.Equal(t, "World", asset.Name)
			assert.Equal(t, ir.ZUp, asset.UpAxis)
			assert.InDelta(t, 0.01, asset.Unit, 1e-9)
			require.Equal(t, []int{19}, asset.RootNodes)
			require.Len(t, asset.Nodes, 20)
			require.Len(t, asset.Meshes, 9)
			require.Len(t, asset.Materials, 1)
			require.Len(t, asset.Cameras, 2)
			require.Len(t, asset.Lights, 4)
			assert.Equal(t, "WoodMat", asset.Materials[0].Name)
			assert.NotEqual(t, ir.NoIndex, asset.FindNode("World"))
			assert.NotEqual(t, ir.NoIndex, asset.FindCamera("MainCam"))
			assert.NotEqual(t, ir.NoIndex, asset.FindCamera("OrthoCam"))
			assert.NotEqual(t, ir.NoIndex, asset.FindLight("Sun"))
			assert.NotEqual(t, ir.NoIndex, asset.FindLight("Lamp"))
			assert.NotEqual(t, ir.NoIndex, asset.FindLight("Spot"))
			assert.NotEqual(t, ir.NoIndex, asset.FindLight("Panel"))
			assertPrimitiveShape(t, asset, "Triangle", ir.Triangles, 3, 3)
			assertPrimitiveShape(t, asset, "Quad", ir.Triangles, 4, 6)
			assertPrimitiveShape(t, asset, "Box", ir.Triangles, 24, 36)
			assertPrimitiveShape(t, asset, "Ball", ir.Triangles, 561, 3072)
			assertPrimitiveShape(t, asset, "Pipe", ir.Triangles, 68, 384)
			assertPrimitiveShape(t, asset, "Spike", ir.Triangles, 35, 192)
			assertPrimitiveShape(t, asset, "Pill", ir.Triangles, 662, 3456)
			assertPrimitiveShape(t, asset, "Wire", ir.Lines, 4, 6)
			assertPrimitiveShape(t, asset, "Dots", ir.Points, 4, 0)
		}},
		{"USDC_Comprehensive", corpus.ModelUSDCComprehensive, ir.FormatUSD, func(t *testing.T, asset *ir.Asset) {
			require.Equal(t, []int{1, 2, 3, 4, 5, 6}, asset.RootNodes)
			require.Len(t, asset.Nodes, 7)
			require.Len(t, asset.Meshes, 2)
			require.Empty(t, asset.Materials)
			require.Len(t, asset.Cameras, 1)
			require.Len(t, asset.Lights, 3)
			assert.NotEqual(t, ir.NoIndex, asset.FindNode("World"))
			assert.NotEqual(t, ir.NoIndex, asset.FindNode("Triangle"))
			assert.NotEqual(t, ir.NoIndex, asset.FindNode("Quad"))
			assert.NotEqual(t, ir.NoIndex, asset.FindCamera("MainCam"))
			assert.NotEqual(t, ir.NoIndex, asset.FindLight("Sun"))
			assert.NotEqual(t, ir.NoIndex, asset.FindLight("Lamp"))
			assert.NotEqual(t, ir.NoIndex, asset.FindLight("Spot"))
			assertPrimitiveShape(t, asset, "Triangle", ir.Triangles, 3, 3)
			assertPrimitiveShape(t, asset, "Quad", ir.Triangles, 4, 6)
		}},

		// ABC (Alembic)
		{"ABC_Cube", corpus.ModelABCCube, ir.FormatAlembic, func(t *testing.T, asset *ir.Asset) {
			assert.Equal(t, "0.1000", asset.Metadata.SourceVersion)
			require.Equal(t, []int{0}, asset.RootNodes)
			require.Len(t, asset.Nodes, 1)
			require.Len(t, asset.Meshes, 1)
			assert.Equal(t, "AlembicMesh", asset.Nodes[0].Name)
			assert.Equal(t, "Cube", asset.Meshes[0].Name)
			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			assert.Equal(t, 3, prim.Data.VertexCount)
			assert.Len(t, prim.Data.Positions, 3)

			lo, hi := asset.SceneBoundingBox(0)
			assert.Equal(t, [3]float32{0, 0, 0}, lo)
			assert.Equal(t, [3]float32{1, 1, 1}, hi)
		}},

		// Exhaustive Isolation
		{"GLTF2_PBRExhaustive", corpus.IsoModelPBRGLTF, ir.FormatGLTF, func(t *testing.T, asset *ir.Asset) {
			require.NotEmpty(t, asset.Materials)
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
	assert.Equal(t, ir.FormatFBX, asset.Metadata.SourceFormat)
	assert.Equal(t, "7500", asset.Metadata.SourceVersion)
	require.Equal(t, []int{0, 2}, asset.RootNodes)
	require.Len(t, asset.Nodes, 4)
	require.Len(t, asset.Meshes, 4)
	require.Len(t, asset.Materials, 2)
	require.Len(t, asset.Animations, 1)
	assert.Equal(t, "Cube2", asset.Nodes[0].Name)
	assert.Equal(t, "Cube3", asset.Nodes[2].Name)
	assert.Equal(t, "Mat_Green", asset.Materials[0].Name)
	assert.Equal(t, "Mat_Red", asset.Materials[1].Name)
	assert.Equal(t, "Take 001", asset.Animations[0].Name)
	assert.Equal(t, ir.Triangles, asset.Meshes[0].Primitives[0].Mode)
	assert.Equal(t, 24, asset.Meshes[0].Primitives[0].Data.VertexCount)
	assert.Len(t, asset.Meshes[0].Primitives[0].Data.Positions, 24)
	assert.Len(t, asset.Meshes[0].Primitives[0].Data.Indices, 36)
	assert.Len(t, asset.Meshes[0].Primitives[0].Data.Normals, 24)
	assert.Equal(t, ir.Triangles, asset.Meshes[3].Primitives[0].Mode)
	assert.Equal(t, 768, asset.Meshes[3].Primitives[0].Data.VertexCount)
	assert.Len(t, asset.Meshes[3].Primitives[0].Data.Positions, 768)
	assert.Len(t, asset.Meshes[3].Primitives[0].Data.Indices, 1152)
	assert.Len(t, asset.Meshes[3].Primitives[0].Data.Normals, 768)
}

func assertPrimitiveShape(t *testing.T, asset *ir.Asset, meshName string, wantMode ir.PrimitiveMode, wantVerts, wantIndices int) {
	t.Helper()

	meshIdx := asset.FindMesh(meshName)
	require.NotEqual(t, ir.NoIndex, meshIdx, "expected mesh %q to exist", meshName)

	mesh := asset.Meshes[meshIdx]
	require.NotNil(t, mesh)
	require.NotEmpty(t, mesh.Primitives)

	prim := mesh.Primitives[0]
	assert.Equal(t, wantMode, prim.Mode)
	assert.Equal(t, wantVerts, prim.Data.VertexCount)
	assert.Len(t, prim.Data.Positions, prim.Data.VertexCount)
	assert.Len(t, prim.Data.Indices, wantIndices)
}
