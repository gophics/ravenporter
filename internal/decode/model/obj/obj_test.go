package obj_test

import (
	_ "embed"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/obj"
	"github.com/gophics/ravenporter/ir"
)

var (
	//go:embed testdata/triangle.obj
	triangleOBJ string

	//go:embed testdata/quad.obj
	quadOBJ string

	//go:embed testdata/uv_normals.obj
	uvNormalsOBJ string

	//go:embed testdata/multi_object.obj
	multiObjectOBJ string

	//go:embed testdata/normal_only.obj
	normalOnlyOBJ string

	//go:embed testdata/negative_index.obj
	negativeIndexOBJ string
)

const mtlOBJSrc = "mtllib test.mtl\nusemtl red\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

func TestDecodeOBJ(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opts    detect.DecodeOptions
		wantErr bool
		check   func(t *testing.T, asset *ir.Asset)
	}{
		{"Triangle", triangleOBJ, detect.DecodeOptions{}, false, func(t *testing.T, asset *ir.Asset) {
			assert.Equal(t, ir.FormatOBJ, asset.Metadata.SourceFormat)
			require.Len(t, asset.Meshes, 1)
			require.Len(t, asset.Meshes[0].Primitives, 1)
			p := &asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, p.Mode)
			assert.Equal(t, 3, p.Data.VertexCount)
			assert.Len(t, p.Data.Positions, 3)
			assert.Len(t, p.Data.Indices, 3)
			assert.Nil(t, p.Data.Normals)
			assert.Nil(t, p.Data.TexCoord0)
			assert.Equal(t, [3]float32{0, 0, 0}, p.Data.Positions[0])
			assert.Equal(t, [3]float32{1, 0, 0}, p.Data.Positions[1])
			assert.Equal(t, [3]float32{0, 1, 0}, p.Data.Positions[2])
		}},
		{"Line Elements", "v 0 0 0\nv 1 0 0\nv 0 1 0\nl 1 2 3\n", detect.DecodeOptions{}, false, func(t *testing.T, asset *ir.Asset) {
			require.Len(t, asset.Meshes, 1)
			prim := &asset.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Lines, prim.Mode)
			assert.Len(t, prim.Data.Indices, 4) // 2 line segments: 0-1, 1-2
		}},
		{"Quad", quadOBJ, detect.DecodeOptions{}, false, func(t *testing.T, asset *ir.Asset) {
			p := &asset.Meshes[0].Primitives[0]
			assert.Equal(t, 4, p.Data.VertexCount)
			assert.Len(t, p.Data.Indices, 6) // quad -> 2 triangles
		}},
		{"UVs and Normals", uvNormalsOBJ, detect.DecodeOptions{}, false, func(t *testing.T, asset *ir.Asset) {
			p := &asset.Meshes[0].Primitives[0]
			assert.Equal(t, 3, p.Data.VertexCount)
			require.NotNil(t, p.Data.TexCoord0)
			require.NotNil(t, p.Data.Normals)
			assert.Equal(t, [2]float32{0, 0}, p.Data.TexCoord0[0])
			assert.Equal(t, [2]float32{1, 0}, p.Data.TexCoord0[1])
			assert.Equal(t, [3]float32{0, 0, 1}, p.Data.Normals[0])
		}},
		{"Multiple Objects", multiObjectOBJ, detect.DecodeOptions{}, false, func(t *testing.T, asset *ir.Asset) {
			require.Len(t, asset.Meshes, 2)
			assert.Equal(t, "Cube", asset.Meshes[0].Name)
			assert.Equal(t, "Plane", asset.Meshes[1].Name)
		}},
		{"Normal Only", normalOnlyOBJ, detect.DecodeOptions{}, false, func(t *testing.T, asset *ir.Asset) {
			p := &asset.Meshes[0].Primitives[0]
			require.NotNil(t, p.Data.Normals)
			assert.Nil(t, p.Data.TexCoord0)
			assert.Equal(t, [3]float32{0, 0, 1}, p.Data.Normals[0])
		}},
		{"Negative Index", negativeIndexOBJ, detect.DecodeOptions{}, false, func(t *testing.T, asset *ir.Asset) {
			p := &asset.Meshes[0].Primitives[0]
			assert.Equal(t, 3, p.Data.VertexCount)
			assert.Equal(t, [3]float32{0, 0, 0}, p.Data.Positions[0])
		}},
		{"Vertex Limit", triangleOBJ, detect.DecodeOptions{MaxVertices: 2}, true, nil},
		{"Empty", "# empty\n", detect.DecodeOptions{}, true, nil},
	}

	dec := &obj.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := dec.Decode(strings.NewReader(tt.input), tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, scene)
				if tt.check != nil {
					tt.check(t, scene)
				}
			}
		})
	}
}

func TestProbe(t *testing.T) {
	dec := &obj.Decoder{}
	assert.True(t, dec.Probe(strings.NewReader("v 0 0 0\n")))
	assert.True(t, dec.Probe(strings.NewReader("# comment\nv 0 0 0\n")))
	assert.False(t, dec.Probe(strings.NewReader("not obj\n")))
	assert.False(t, dec.Probe(strings.NewReader("")))
}

func TestMTLParsing(t *testing.T) {
	mtl := `newmtl red
Kd 1.0 0.0 0.0
Ns 50
d 0.8
map_Kd texture.png
newmtl blue
Kd 0.0 0.0 1.0
Pm 0.9
Pr 0.1
`

	asset := &ir.Asset{}
	obj.ParseMTLForTest(strings.NewReader(mtl), asset)

	require.Len(t, asset.Materials, 2)
	assert.Equal(t, "red", asset.Materials[0].Name)
	assert.InDelta(t, float32(1.0), asset.Materials[0].BaseColorFactor[0], 0.01)
	assert.InDelta(t, float32(0.8), asset.Materials[0].BaseColorFactor[3], 0.01)
	assert.Equal(t, ir.AlphaBlend, asset.Materials[0].AlphaMode)
	require.NotNil(t, asset.Materials[0].BaseColorTexture)

	assert.Equal(t, "blue", asset.Materials[1].Name)
	assert.InDelta(t, float32(0.9), asset.Materials[1].MetallicFactor, 0.01)
	assert.InDelta(t, float32(0.1), asset.Materials[1].RoughnessFactor, 0.01)

	dec := &obj.Decoder{}
	scene2, err := dec.Decode(strings.NewReader(mtlOBJSrc), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene2.Meshes, 1)
}

type memFS struct {
	files map[string]string
}

func (fs *memFS) Open(name string) (io.ReadCloser, error) {
	data, ok := fs.files[name]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(strings.NewReader(data)), nil
}

func TestMTLFSIntegration(t *testing.T) {
	mtlData := "newmtl red\nKd 1.0 0.0 0.0\nmap_Kd diffuse.png\n"

	fs := &memFS{files: map[string]string{"test.mtl": mtlData}}
	dec := &obj.Decoder{}
	asset, err := dec.Decode(strings.NewReader(mtlOBJSrc), detect.DecodeOptions{FS: fs})
	require.NoError(t, err)

	require.Len(t, asset.Materials, 1)
	assert.Equal(t, "red", asset.Materials[0].Name)
	assert.InDelta(t, float32(1.0), asset.Materials[0].BaseColorFactor[0], 0.01)
	assert.InDelta(t, float32(0.0), asset.Materials[0].BaseColorFactor[1], 0.01)

	require.Len(t, asset.Textures, 1)
	assert.Equal(t, "diffuse.png", asset.Images[asset.Textures[0].ImageIndex].SourcePath)

	require.Len(t, asset.Meshes, 1)
	assert.Equal(t, 0, asset.Meshes[0].Primitives[0].MaterialIndex)
}

func TestMTLMissingFile(t *testing.T) {
	objSrc := "mtllib missing.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

	fs := &memFS{files: map[string]string{}}
	dec := &obj.Decoder{}
	asset, err := dec.Decode(strings.NewReader(objSrc), detect.DecodeOptions{FS: fs})
	require.NoError(t, err) // MTL is optional â€” should not error
	assert.Empty(t, asset.Materials)
	require.Len(t, asset.Meshes, 1)
}

func TestMTLNilFS(t *testing.T) {
	dec := &obj.Decoder{}
	asset, err := dec.Decode(strings.NewReader(mtlOBJSrc), detect.DecodeOptions{})
	require.NoError(t, err) // nil FS â€” should not error
	assert.Empty(t, asset.Materials)
	require.Len(t, asset.Meshes, 1)
}

func TestDecodePoints(t *testing.T) {
	src := "v 0 0 0\nv 1 0 0\nv 0 1 0\nv 1 1 0\np 1 2 3 4\n"
	dec := &obj.Decoder{}
	asset, err := dec.Decode(strings.NewReader(src), detect.DecodeOptions{})
	require.NoError(t, err)

	var ptMesh *ir.Mesh
	for _, m := range asset.Meshes {
		if m.Primitives[0].Mode == ir.Points {
			ptMesh = m
			break
		}
	}
	require.NotNil(t, ptMesh)
	assert.Equal(t, ir.Points, ptMesh.Primitives[0].Mode)
	assert.Len(t, ptMesh.Primitives[0].Data.Indices, 4)
}

func TestDecodeVertexColors(t *testing.T) {
	src := "v 0 0 0 1.0 0.0 0.0\nv 1 0 0 0.0 1.0 0.0\nv 0 1 0 0.0 0.0 1.0\nf 1 2 3\n"
	dec := &obj.Decoder{}
	asset, err := dec.Decode(strings.NewReader(src), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, asset.Meshes, 1)
	p := &asset.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
	require.Len(t, p.Data.Colors0, 3)
	assert.Equal(t, [4]float32{1, 0, 0, 1}, p.Data.Colors0[0])
	assert.Equal(t, [4]float32{0, 1, 0, 1}, p.Data.Colors0[1])
	assert.Equal(t, [4]float32{0, 0, 1, 1}, p.Data.Colors0[2])
}

func TestMTLNewFeatures(t *testing.T) {
	tests := []struct {
		name  string
		mtl   string
		check func(t *testing.T, mat *ir.Material, asset *ir.Asset)
	}{
		{
			name: "Ke emissive color",
			mtl:  "newmtl m\nKe 0.5 0.6 0.7\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				assert.InDelta(t, float32(0.5), mat.EmissiveFactor[0], 0.01)
				assert.InDelta(t, float32(0.6), mat.EmissiveFactor[1], 0.01)
				assert.InDelta(t, float32(0.7), mat.EmissiveFactor[2], 0.01)
			},
		},
		{
			name: "illum 4 sets alpha blend",
			mtl:  "newmtl m\nd 0.8\nillum 4\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				assert.Equal(t, ir.AlphaBlend, mat.AlphaMode)
			},
		},
		{
			name: "illum 1 does not set alpha blend",
			mtl:  "newmtl m\nd 0.8\nillum 1\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				assert.Equal(t, ir.AlphaBlend, mat.AlphaMode) // d=0.8 already sets blend
			},
		},
		{
			name: "Ni optical density IOR",
			mtl:  "newmtl m\nNi 1.5\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.InDelta(t, float32(1.5), mat.Properties["ior"], 0.01)
			},
		},
		{
			name: "map_Ke emissive texture",
			mtl:  "newmtl m\nmap_Ke emissive.png\n",
			check: func(t *testing.T, mat *ir.Material, asset *ir.Asset) {
				require.NotNil(t, mat.EmissiveTexture)
				require.True(t, mat.EmissiveTexture.TextureIndex < len(asset.Textures))
				assert.Equal(t, "emissive.png", asset.Images[asset.Textures[mat.EmissiveTexture.TextureIndex].ImageIndex].SourcePath)
			},
		},
		{
			name: "map_Pr roughness texture",
			mtl:  "newmtl m\nmap_Pr roughness.png\n",
			check: func(t *testing.T, mat *ir.Material, asset *ir.Asset) {
				require.NotNil(t, mat.RoughnessTexture)
				require.True(t, mat.RoughnessTexture.TextureIndex < len(asset.Textures))
				assert.Equal(t, "roughness.png", asset.Images[asset.Textures[mat.RoughnessTexture.TextureIndex].ImageIndex].SourcePath)
			},
		},
		{
			name: "map_Pm metallic texture",
			mtl:  "newmtl m\nmap_Pm metallic.png\n",
			check: func(t *testing.T, mat *ir.Material, asset *ir.Asset) {
				require.NotNil(t, mat.MetallicTexture)
				require.True(t, mat.MetallicTexture.TextureIndex < len(asset.Textures))
				assert.Equal(t, "metallic.png", asset.Images[asset.Textures[mat.MetallicTexture.TextureIndex].ImageIndex].SourcePath)
			},
		},
		{
			name: "Ps sheen factor",
			mtl:  "newmtl m\nPs 0.8\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.InDelta(t, float32(0.8), mat.Properties["sheenFactor"], 0.01)
			},
		},
		{
			name: "Pc clearcoat factor",
			mtl:  "newmtl m\nPc 0.5\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.InDelta(t, float32(0.5), mat.Properties["clearcoatFactor"], 0.01)
			},
		},
		{
			name: "Pcr clearcoat roughness",
			mtl:  "newmtl m\nPcr 0.3\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.InDelta(t, float32(0.3), mat.Properties["clearcoatRoughnessFactor"], 0.01)
			},
		},
		{
			name: "Tf transmittance filter",
			mtl:  "newmtl m\nTf 1.0 0.5 0.2\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				tf := mat.Properties["transmittanceFilter"]
				require.NotNil(t, tf)
				arr := tf.([3]float32)
				assert.InDelta(t, float32(1.0), arr[0], 0.01)
				assert.InDelta(t, float32(0.5), arr[1], 0.01)
				assert.InDelta(t, float32(0.2), arr[2], 0.01)
			},
		},
		{
			name: "aniso anisotropy strength",
			mtl:  "newmtl m\naniso 0.7\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.InDelta(t, float32(0.7), mat.Properties["anisotropyStrength"], 0.01)
			},
		},
		{
			name: "anisor anisotropy rotation",
			mtl:  "newmtl m\nanisor 1.2\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.InDelta(t, float32(1.2), mat.Properties["anisotropyRotation"], 0.01)
			},
		},
		{
			name: "map_Ka ambient texture",
			mtl:  "newmtl m\nmap_Ka ambient.png\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.NotNil(t, mat.Properties["ambientTexture"])
			},
		},
		{
			name: "disp displacement texture",
			mtl:  "newmtl m\ndisp height.png\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.NotNil(t, mat.Properties["displacementTexture"])
			},
		},
		{
			name: "refl reflection texture",
			mtl:  "newmtl m\nrefl env.hdr\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.Properties)
				assert.NotNil(t, mat.Properties["reflectionTexture"])
			},
		},
		{
			name: "texture scale option -s",
			mtl:  "newmtl m\nmap_Kd -s 2.0 3.0 diffuse.png\n",
			check: func(t *testing.T, mat *ir.Material, asset *ir.Asset) {
				require.NotNil(t, mat.BaseColorTexture)
				assert.InDelta(t, float32(2.0), mat.BaseColorTexture.Tiling[0], 0.01)
				assert.InDelta(t, float32(3.0), mat.BaseColorTexture.Tiling[1], 0.01)
				require.True(t, mat.BaseColorTexture.TextureIndex < len(asset.Textures))
				assert.Equal(t, "diffuse.png", asset.Images[asset.Textures[mat.BaseColorTexture.TextureIndex].ImageIndex].SourcePath)
			},
		},
		{
			name: "texture offset option -o",
			mtl:  "newmtl m\nmap_Kd -o 0.5 0.25 diffuse.png\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.BaseColorTexture)
				assert.InDelta(t, float32(0.5), mat.BaseColorTexture.Offset[0], 0.01)
				assert.InDelta(t, float32(0.25), mat.BaseColorTexture.Offset[1], 0.01)
			},
		},
		{
			name: "texture bump multiplier -bm",
			mtl:  "newmtl m\nbump -bm 2.5 normal.png\n",
			check: func(t *testing.T, mat *ir.Material, _ *ir.Asset) {
				require.NotNil(t, mat.NormalTexture)
				require.NotNil(t, mat.Properties)
				assert.InDelta(t, float32(2.5), mat.Properties["bumpMultiplier"], 0.01)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{}
			obj.ParseMTLForTest(strings.NewReader(tt.mtl), asset)
			require.Len(t, asset.Materials, 1)
			tt.check(t, asset.Materials[0], asset)
		})
	}
}

func TestMTLAmbientSpecularTransparency(t *testing.T) {
	tests := []struct {
		name  string
		mtl   string
		check func(t *testing.T, mat *ir.Material)
	}{
		{
			name: "Ka ambient",
			mtl:  "newmtl m\nKa 0.1 0.2 0.3\n",
			check: func(t *testing.T, mat *ir.Material) {
				require.NotNil(t, mat.Properties)
				assert.NotNil(t, mat.Properties["ambient"])
			},
		},
		{
			name: "Ks specular",
			mtl:  "newmtl m\nKs 0.5 0.6 0.7\n",
			check: func(t *testing.T, mat *ir.Material) {
				require.NotNil(t, mat.Properties)
				assert.NotNil(t, mat.Properties["specular"])
			},
		},
		{
			name: "Tr transparency",
			mtl:  "newmtl m\nTr 0.2\n",
			check: func(t *testing.T, mat *ir.Material) {
				assert.InDelta(t, float32(0.8), mat.BaseColorFactor[3], 0.01)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{}
			obj.ParseMTLForTest(strings.NewReader(tt.mtl), asset)
			require.Len(t, asset.Materials, 1)
			tt.check(t, asset.Materials[0])
		})
	}
}

func TestSmoothGroups(t *testing.T) {
	src := "v 0 0 0\nv 1 0 0\nv 0 1 0\ns 1\nf 1 2 3\ns off\n"
	dec := &obj.Decoder{}
	asset, err := dec.Decode(strings.NewReader(src), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, asset.Meshes, 1)
}

func TestDecodeInvalidFace(t *testing.T) {
	src := "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 999 998 997\n"
	dec := &obj.Decoder{}
	_, err := dec.Decode(strings.NewReader(src), detect.DecodeOptions{})
	assert.Error(t, err)
}

func TestParseTexCoord3D(t *testing.T) {
	src := "v 0 0 0\nv 1 0 0\nv 0 1 0\nvt 0.5 0.5 0.0\nvt 1.0 0.0 0.0\nvt 0.0 1.0 0.0\nf 1/1 2/2 3/3\n"
	dec := &obj.Decoder{}
	asset, err := dec.Decode(strings.NewReader(src), detect.DecodeOptions{})
	require.NoError(t, err)
	p := &asset.Meshes[0].Primitives[0]
	require.NotNil(t, p.Data.TexCoord0)
}
