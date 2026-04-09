package cache

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/ir"
)

func TestWriteReadRoundTrip(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	asset, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.NotNil(t, asset)
	require.NotNil(t, asset.Asset)
	defer func() { require.NoError(t, asset.Close()) }()

	assert.Equal(t, formatVersion, asset.Manifest.FormatVersion)
	assert.Equal(t, ir.FormatGLTF, asset.Manifest.SourceFormat)
	assert.Equal(t, result.Report.Source.Options, asset.Manifest.SourceProfile)
	assert.Equal(t, result.Report.Dependencies, asset.Manifest.Dependencies)
	assert.Equal(t, result.Report.Source.Notes, asset.Manifest.Notes)
	assert.Equal(t, result.Report.Summary, asset.Manifest.Summary)

	cooked := asset.Asset
	assert.Equal(t, result.Asset.Name, cooked.Name)
	assert.Equal(t, result.Asset.Metadata, cooked.Metadata)
	assert.Len(t, cooked.Nodes, 1)
	assert.Equal(t, result.Asset.Nodes[0].Extras, cooked.Nodes[0].Extras)
	assert.Equal(t, result.Asset.Nodes[0].MeshIndex, cooked.Nodes[0].MeshIndex)
	assert.Equal(t, result.Asset.RootNodes, cooked.RootNodes)

	require.Len(t, cooked.Meshes, 1)
	require.NotNil(t, cooked.Meshes[0])
	assert.Equal(t, result.Asset.Meshes[0].Name, cooked.Meshes[0].Name)
	assert.Equal(t, result.Asset.Meshes[0].BoundingBox, cooked.Meshes[0].BoundingBox)
	assert.Equal(t, result.Asset.Meshes[0].Primitives[0].Data.Positions, cooked.Meshes[0].Primitives[0].Data.Positions)
	assert.Equal(t, result.Asset.Meshes[0].Primitives[0].MorphTargets[0].Positions, cooked.Meshes[0].Primitives[0].MorphTargets[0].Positions)

	require.Len(t, cooked.Materials, 1)
	require.NotNil(t, cooked.Materials[0])
	assert.Equal(t, result.Asset.Materials[0].Properties, cooked.Materials[0].Properties)
	assert.Equal(t, result.Asset.Materials[0].Clearcoat.Factor, cooked.Materials[0].Clearcoat.Factor)

	require.Len(t, cooked.Textures, 1)
	require.NotNil(t, cooked.Textures[0])
	assert.Equal(t, result.Asset.Textures[0], cooked.Textures[0])

	require.Len(t, cooked.Animations, 1)
	assert.Equal(t, result.Asset.Animations[0].Channels[0].Translations, cooked.Animations[0].Channels[0].Translations)
	require.Len(t, cooked.Skeletons, 1)
	assert.Equal(t, result.Asset.Skeletons[0].InverseBindMatrices, cooked.Skeletons[0].InverseBindMatrices)
	require.Len(t, cooked.Cameras, 1)
	assert.Equal(t, result.Asset.Cameras[0].Perspective.FOV, cooked.Cameras[0].Perspective.FOV)
	require.Len(t, cooked.Lights, 1)
	assert.Equal(t, result.Asset.Lights[0].Point.Range, cooked.Lights[0].Point.Range)

	require.Len(t, cooked.AudioClips, 1)
	compressedAudio, err := cooked.AudioClips[0].CompressedBytes()
	require.NoError(t, err)
	assert.Equal(t, result.Asset.AudioClips[0].Compressed, compressedAudio)
	assert.Equal(t, result.Asset.AudioClips[0].ChannelMask, cooked.AudioClips[0].ChannelMask)
	assert.Equal(t, result.Asset.AudioClips[0].Metadata.Artwork, cooked.AudioClips[0].Metadata.Artwork)
	assert.NotNil(t, cooked.AudioClips[0].SampleDecode)
	samples, err := cooked.AudioClips[0].DecodeSamples()
	require.NoError(t, err)
	assert.Len(t, samples, 1)

	require.Len(t, cooked.Fonts, 2)
	fontBytes, err := cooked.Fonts[0].Vector.RawBytes()
	require.NoError(t, err)
	assert.Equal(t, result.Asset.Fonts[0].Vector.RawData, fontBytes)
	assert.Equal(t, result.Asset.Fonts[1].Bitmap.Glyphs, cooked.Fonts[1].Bitmap.Glyphs)

	require.Len(t, cooked.Images, 1)
	compressedImage, err := cooked.Images[0].CompressedBytes()
	require.NoError(t, err)
	assert.Equal(t, result.Asset.Images[0].Compressed, compressedImage)
	assert.Equal(t, result.Asset.Images[0].Topology, cooked.Images[0].Topology)
	assert.Equal(t, result.Asset.Images[0].Depth, cooked.Images[0].Depth)
	assert.Equal(t, result.Asset.Images[0].Layers, cooked.Images[0].Layers)
	assert.Equal(t, result.Asset.Images[0].SourcePath, cooked.Images[0].SourcePath)
	require.Nil(t, cooked.Images[0].Pixels())
	require.NotNil(t, cooked.Images[0].PixelDecode)
	pixels, err := cooked.Images[0].DecodePixels()
	require.NoError(t, err)
	require.NotNil(t, pixels)
	assert.Equal(t, result.Asset.Images[0].Pixels().Data, pixels.Data)

	require.Len(t, cooked.LODGroups, 1)
	assert.Equal(t, result.Asset.LODGroups[0].Levels, cooked.LODGroups[0].Levels)
	require.Len(t, cooked.CollisionMeshes, 1)
	assert.Equal(t, result.Asset.CollisionMeshes[0], cooked.CollisionMeshes[0])

	assert.Equal(t, 0, asset.FindMesh("Mesh"))
	assert.Equal(t, 0, asset.FindMaterial("Material"))
	assert.Equal(t, 0, asset.FindAnimation("Anim"))
	assert.Equal(t, 0, asset.FindNode("Root"))
}

func TestWriteNormalizesGraph(t *testing.T) {
	result := &ravenporter.Result{
		Asset: &ir.Asset{
			Scenes: []*ir.Scene{{Name: "Scene", RootNodes: []int{0}}},
			Nodes: []ir.Node{
				{Name: "Root", ParentIndex: 0, Children: []int{1}},
				{Name: "Child"},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	cooked, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.NotNil(t, cooked)
	require.NotNil(t, cooked.Asset)
	defer func() { require.NoError(t, cooked.Close()) }()
	assert.Equal(t, ir.NoIndex, cooked.Nodes[0].ParentIndex)
	assert.Equal(t, 0, cooked.Nodes[1].ParentIndex)
	assert.Equal(t, []int{0}, cooked.RootNodes)
	assert.Equal(t, []int{0}, cooked.Scenes[0].RootNodes)
}

func TestWriteFailsOnExternalTexture(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		result *ravenporter.Result
		want   string
	}{
		{
			name:   "nil result",
			result: nil,
			want:   "nil result",
		},
		{
			name:   "nil scene",
			result: &ravenporter.Result{},
			want:   "nil asset",
		},
		{
			name: "external texture path",
			result: &ravenporter.Result{
				Asset: &ir.Asset{
					Images:   []*ir.ImageAsset{{SourcePath: "albedo.png"}},
					Textures: []*ir.Texture{{ImageIndex: 0}},
				},
			},
			want: "external path",
		},
		{
			name: "unsupported material property",
			result: &ravenporter.Result{
				Asset: &ir.Asset{
					Materials: []*ir.Material{{Properties: map[string]any{"bad": []float32{1, 2, 3}}}},
				},
			},
			want: "unsupported material property",
		},
		{
			name: "malformed data uri",
			result: &ravenporter.Result{
				Asset: &ir.Asset{
					Images:   []*ir.ImageAsset{{SourcePath: "data:bad"}},
					Textures: []*ir.Texture{{ImageIndex: 0}},
				},
			},
			want: "malformed data URI",
		},
		{
			name: "data uri accepted",
			result: &ravenporter.Result{
				Asset: &ir.Asset{
					Images: []*ir.ImageAsset{{
						SourcePath: "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("abc")),
					}},
					Textures: []*ir.Texture{{ImageIndex: 0}},
				},
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Write(&buf, tc.result)
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestReadRejectsCorruptContainer(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))
	data := buf.Bytes()

	cases := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{
			name: "bad magic",
			mutate: func(in []byte) []byte {
				out := append([]byte(nil), in...)
				out[0] = 'X'
				return out
			},
		},
		{
			name: "unsupported version",
			mutate: func(in []byte) []byte {
				out := append([]byte(nil), in...)
				binary.LittleEndian.PutUint16(out[8:10], 99)
				return out
			},
		},
		{
			name: "truncated table",
			mutate: func(in []byte) []byte {
				return append([]byte(nil), in[:headerSize+8]...)
			},
		},
		{
			name: "truncated payload",
			mutate: func(in []byte) []byte {
				return append([]byte(nil), in[:len(in)-5]...)
			},
		},
		{
			name: "out of bounds chunk",
			mutate: func(in []byte) []byte {
				out := append([]byte(nil), in...)
				binary.LittleEndian.PutUint64(out[20:28], uint64(len(out)+128))
				return out
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.mutate(data)
			_, err := Read(bytes.NewReader(data), int64(len(data)))
			require.Error(t, err)
		})
	}
}

func TestImportCookReadMeshoptGLTF(t *testing.T) {
	path := filepath.Join("testdata", "gltf_meshopt_indices.gltf")
	result, err := ravenporter.ImportPath(context.Background(), path)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	asset, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.NotNil(t, asset.Asset)
	defer func() { require.NoError(t, asset.Close()) }()
	assert.Equal(t, len(result.Asset.Meshes), len(asset.Meshes))
	assert.Equal(t, len(result.Asset.Nodes), len(asset.Nodes))
}

func TestImportCookReadOBJ(t *testing.T) {
	dir := t.TempDir()
	objPath := filepath.Join(dir, "tri.obj")
	require.NoError(t, os.WriteFile(objPath, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	result, err := ravenporter.ImportPath(context.Background(), objPath)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	asset, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, asset.Meshes, 1)
	require.NoError(t, asset.Close())
}

func TestImportCookReadEmbeddedTextureGLTF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "embedded.gltf")
	require.NoError(t, os.WriteFile(path, embeddedTextureGLTF(t), 0o644))

	result, err := ravenporter.ImportPath(context.Background(), path)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	asset, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, asset.Textures, 1)
	require.Len(t, asset.Images, 1)
	defer func() { require.NoError(t, asset.Close()) }()
	compressed, err := asset.Images[asset.Textures[0].ImageIndex].CompressedBytes()
	require.NoError(t, err)
	assert.NotEmpty(t, compressed)
}

func TestOpenEntrypoints(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))
	data := buf.Bytes()

	t.Run("Open", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "scene.rpcache")
		require.NoError(t, os.WriteFile(path, data, 0o600))

		asset, err := Open(path)
		require.NoError(t, err)
		defer func() { require.NoError(t, asset.Close()) }()
		require.NotNil(t, asset.Asset)
		assert.Equal(t, result.Asset.Name, asset.Name)
	})

	t.Run("OpenFS", func(t *testing.T) {
		fsys := fstest.MapFS{
			"scene.rpcache": &fstest.MapFile{Data: data},
		}

		asset, err := OpenFS(fsys, "scene.rpcache")
		require.NoError(t, err)
		defer func() { require.NoError(t, asset.Close()) }()
		require.NotNil(t, asset.Asset)
		assert.Equal(t, result.Asset.Name, asset.Name)
	})
}

func TestReadWithEagerMedia(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	asset, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()), WithEagerMedia())
	require.NoError(t, err)
	defer func() { require.NoError(t, asset.Close()) }()

	image := asset.Images[0]
	audio := asset.AudioClips[0]
	font := asset.Fonts[0].Vector

	assert.NotEmpty(t, image.Compressed)
	assert.NotEmpty(t, audio.Compressed)
	assert.NotEmpty(t, font.RawData)
}

func TestOpenCloseInvalidatesLazyMedia(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	path := filepath.Join(t.TempDir(), "scene.rpcache")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))

	asset, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, asset.Close())

	_, err = asset.Images[0].CompressedBytes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestWriteWithImagePixelsIfPresent(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result, WithImagePixels(ImagePixelsIfPresent)))

	asset, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	defer func() { require.NoError(t, asset.Close()) }()

	require.NotNil(t, asset.Images[0].Pixels())
}

func TestWritePreservesDecodeOnlyImage(t *testing.T) {
	result := &ravenporter.Result{
		Asset: &ir.Asset{
			Images: []*ir.ImageAsset{{
				Name:   "decode-only",
				Format: ir.ImagePNG,
				Width:  1,
				Height: 1,
				PixelDecode: func(_ *ir.ImageAsset) (*ir.PixelBuffer, error) {
					return &ir.PixelBuffer{
						Data:     []byte{1, 2, 3, 4},
						DataType: ir.DataTypeUint8,
						BitDepth: ir.BitDepth8,
					}, nil
				},
			}},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, result))

	asset, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	defer func() { require.NoError(t, asset.Close()) }()

	require.NotNil(t, asset.Images[0].Pixels())
	assert.Equal(t, []byte{1, 2, 3, 4}, asset.Images[0].Pixels().Data)
}

func TestWriteRejectsEmbeddedMediaOverLimit(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var buf bytes.Buffer
	err := Write(&buf, result, WithMaxEmbeddedMediaBytes(8))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded media exceeds limit")
}

func fullReport() ravenporter.Report {
	return ravenporter.Report{
		Source: ravenporter.SourceReport{
			DetectedFormat: ir.FormatGLTF,
			Options: ravenporter.Profile{
				Version: 1,
				Preset:  ravenporter.BuiltInPresetQuality,
			},
			Notes: map[string][]string{
				"runtime": {"cook"},
			},
		},
		Dependencies: []ravenporter.Dependency{{
			Kind:       "buffer",
			Path:       "shared.bin",
			Relation:   "buffer",
			ReportedBy: "mock",
		}},
		Summary: ravenporter.AssetSummary{
			Scenes:          1,
			Meshes:          1,
			Materials:       1,
			Textures:        1,
			Nodes:           1,
			Animations:      1,
			Skeletons:       1,
			Cameras:         1,
			Lights:          1,
			AudioClips:      1,
			Fonts:           2,
			Images:          1,
			LODGroups:       1,
			CollisionMeshes: 1,
		},
	}
}

func fullScene() *ir.Asset {
	image := &ir.ImageAsset{
		Name:              "Image",
		Format:            ir.ImagePNG,
		Width:             1,
		Height:            1,
		Channels:          ir.ChannelRGBA,
		ColorSpace:        ir.ColorSRGB,
		MipLevels:         2,
		Topology:          ir.ImageTopologyCube,
		Depth:             1,
		Layers:            6,
		Compressed:        onePixelPNG(),
		SourceFormat:      ir.FormatPNG,
		CompressionFormat: ir.GPUCompressionNone,
		Metadata:          map[string]string{"kind": "albedo"},
	}
	image.SetPixels(&ir.PixelBuffer{
		Data:     []byte{255, 0, 0, 255},
		DataType: ir.DataTypeUint8,
		BitDepth: ir.BitDepth8,
		Mipmaps:  [][]byte{{128, 0, 0, 255}},
	})

	return &ir.Asset{
		Name:   "Scene",
		UpAxis: ir.YUp,
		Unit:   1,
		Scenes: []*ir.Scene{{Name: "Scene", RootNodes: []int{0}}},
		Nodes: []ir.Node{{
			Name:          "Root",
			ParentIndex:   ir.NoIndex,
			Transform:     ir.IdentityTransform(),
			Visible:       true,
			Mobility:      ir.MobilityStatic,
			MeshIndex:     0,
			SkinIndex:     0,
			CameraIndex:   0,
			LightIndex:    0,
			LODGroupIndex: 0,
			MorphWeights:  []float32{0.5},
			Extras:        map[string]any{"drop": true},
		}},
		RootNodes: []int{0},
		Meshes: []*ir.Mesh{{
			Name:         "Mesh",
			MorphWeights: []float32{0.25},
			BoundingBox:  [2][3]float32{{0, 0, 0}, {1, 1, 0}},
			Primitives: []ir.Primitive{{
				Mode:          ir.Triangles,
				MaterialIndex: 0,
				Data: ir.MeshData{
					VertexCount:  3,
					Positions:    [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
					Indices:      []uint32{0, 1, 2},
					Normals:      [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
					TexCoord0:    [][2]float32{{0, 0}, {1, 0}, {0, 1}},
					Colors0:      [][4]float32{{1, 0, 0, 1}, {0, 1, 0, 1}, {0, 0, 1, 1}},
					Joints0:      [][4]uint16{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}},
					Weights0:     [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}},
					SmoothGroups: []int{1},
				},
				MorphTargets: []ir.MorphTarget{{
					Name:      "Smile",
					Indices:   []uint32{0},
					Positions: [][3]float32{{0.1, 0, 0}},
				}},
			}},
		}},
		Materials: []*ir.Material{{
			Name:             "Material",
			BaseColorFactor:  [4]float32{1, 1, 1, 1},
			BaseColorTexture: &ir.TextureRef{TextureIndex: 0, UVSet: 0, Tiling: [2]float32{1, 1}},
			MetallicFactor:   0.2,
			RoughnessFactor:  0.7,
			EmissiveFactor:   [3]float32{0.1, 0.2, 0.3},
			AlphaMode:        ir.AlphaBlend,
			DoubleSided:      true,
			Clearcoat:        &ir.MaterialClearcoat{Factor: 0.5},
			Properties: map[string]any{
				"float":  float32(1.5),
				"int":    2,
				"bool":   true,
				"string": "value",
				"vec3":   [3]float32{1, 2, 3},
				"vec4":   [4]float32{1, 2, 3, 4},
			},
		}},
		Textures: []*ir.Texture{{
			Name:       "Texture",
			ImageIndex: 0,
			MipLevels:  1,
			WrapS:      ir.WrapRepeat,
			WrapT:      ir.WrapClamp,
			MinFilter:  ir.FilterLinear,
			MagFilter:  ir.FilterNearest,
			Metadata:   map[string]string{"role": "base-color"},
		}},
		Animations: []*ir.Animation{{
			Name:     "Anim",
			Duration: 1,
			Channels: []ir.AnimationChannel{{
				NodeIndex:     0,
				Target:        ir.TargetTranslation,
				Interpolation: ir.InterpolationLinear,
				Times:         []float32{0, 1},
				Translations:  [][3]float32{{0, 0, 0}, {1, 0, 0}},
			}},
		}},
		Skeletons: []*ir.Skeleton{{
			Name:                "Rig",
			Joints:              []int{0},
			RootIdx:             0,
			InverseBindMatrices: [][16]float32{{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}},
		}},
		Cameras: []*ir.Camera{{
			Name: "Camera",
			Perspective: &ir.PerspectiveCamera{
				FOV:  1,
				Near: 0.1,
				Far:  100,
			},
		}},
		Lights: []*ir.Light{{
			Name:        "Light",
			Color:       [3]float32{1, 1, 1},
			Temperature: 6500,
			Intensity:   5,
			IESProfile:  &ir.TextureRef{TextureIndex: 0},
			Point:       &ir.PointLight{Range: 10},
		}},
		AudioClips: []*ir.AudioClip{{
			Name:        "Clip",
			Format:      ir.AudioWAV,
			SampleRate:  8000,
			Layout:      ir.LayoutMono,
			ChannelMask: 0x4,
			BitDepth:    ir.BitDepth16,
			Duration:    2,
			LoopStart:   0,
			LoopEnd:     10,
			Metadata: ir.AudioMetadata{
				Title:   "Title",
				Artwork: []byte{6, 7, 8},
				CuePoints: []ir.CuePoint{{
					Name:   "Start",
					Sample: 0,
				}},
			},
			Compressed:  tinyPCM16WAV(),
			SourceCodec: ir.AudioWAV,
		}},
		Fonts: []*ir.Font{
			{
				Name:   "Vector",
				Format: ir.FontTTF,
				Vector: &ir.VectorFontData{
					UnitsPerEm: 1000,
					Ascender:   800,
					Descender:  -200,
					LineGap:    200,
					GlyphCount: 1,
					Codepoints: []rune{'A'},
					Advances:   map[rune]int{'A': 600},
					Kerning:    []ir.KerningPair{{First: 'A', Second: 'V', Amount: -40}},
					RawData:    []byte{1, 2, 3, 4},
				},
			},
			{
				Name:   "Bitmap",
				Format: ir.FontBMFont,
				Bitmap: &ir.BitmapFontData{
					LineHeight: 16,
					Base:       12,
					GlyphCount: 1,
					AtlasPath:  "atlas.png",
					AtlasIndex: 0,
					Glyphs: map[rune]ir.BitmapGlyph{
						'A': {Width: 8, Height: 8, Advance: 9},
					},
				},
			},
		},
		Images:    []*ir.ImageAsset{image},
		LODGroups: []*ir.LODGroup{{Name: "LOD", Levels: []ir.LODLevel{{Threshold: 0.5, NodeIndex: 0}}}},
		CollisionMeshes: []*ir.CollisionMesh{{
			Type:      ir.CollisionTypeMesh,
			MeshIndex: 0,
			NodeIndex: 0,
		}},
		Metadata: ir.AssetMetadata{
			SourceFormat:    ir.FormatGLTF,
			SourceVersion:   "2.0",
			Generator:       "test",
			CreationTime:    "2026-04-03T00:00:00Z",
			ExtraProperties: map[string]string{"mode": "test"},
		},
	}
}

func tinyPCM16WAV() []byte {
	return []byte{
		'R', 'I', 'F', 'F',
		0x26, 0x00, 0x00, 0x00,
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		0x10, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x01, 0x00,
		0x40, 0x1F, 0x00, 0x00,
		0x80, 0x3E, 0x00, 0x00,
		0x02, 0x00,
		0x10, 0x00,
		'd', 'a', 't', 'a',
		0x02, 0x00, 0x00, 0x00,
		0x00, 0x00,
	}
}

func embeddedTextureGLTF(tb testing.TB) []byte {
	tb.Helper()

	buffer := make([]byte, 0, 64)
	appendFloat32 := func(v float32) {
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], math.Float32bits(v))
		buffer = append(buffer, tmp[:]...)
	}
	appendUint16 := func(v uint16) {
		var tmp [2]byte
		binary.LittleEndian.PutUint16(tmp[:], v)
		buffer = append(buffer, tmp[:]...)
	}
	for _, value := range []float32{
		0, 0, 0,
		1, 0, 0,
		0, 1, 0,
		0, 0,
		1, 0,
		0, 1,
	} {
		appendFloat32(value)
	}
	for _, index := range []uint16{0, 1, 2} {
		appendUint16(index)
	}

	doc := map[string]any{
		"asset": map[string]any{"version": "2.0"},
		"buffers": []map[string]any{{
			"uri":        "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(buffer),
			"byteLength": len(buffer),
		}},
		"bufferViews": []map[string]any{
			{"buffer": 0, "byteOffset": 0, "byteLength": 36},
			{"buffer": 0, "byteOffset": 36, "byteLength": 24},
			{"buffer": 0, "byteOffset": 60, "byteLength": 6},
		},
		"accessors": []map[string]any{
			{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"},
			{"bufferView": 1, "componentType": 5126, "count": 3, "type": "VEC2"},
			{"bufferView": 2, "componentType": 5123, "count": 3, "type": "SCALAR"},
		},
		"images": []map[string]any{{
			"uri": "data:image/png;base64," + base64.StdEncoding.EncodeToString(onePixelPNG()),
		}},
		"textures":  []map[string]any{{"source": 0}},
		"materials": []map[string]any{{"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}}}},
		"meshes": []map[string]any{{
			"primitives": []map[string]any{{
				"attributes": map[string]any{"POSITION": 0, "TEXCOORD_0": 1},
				"indices":    2,
				"material":   0,
			}},
		}},
		"nodes":  []map[string]any{{"mesh": 0}},
		"scenes": []map[string]any{{"nodes": []int{0}}},
		"scene":  0,
	}
	data, err := json.Marshal(doc)
	require.NoError(tb, err)
	return data
}

func onePixelPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0xF8, 0xCF, 0xC0, 0xF0,
		0x1F, 0x00, 0x05, 0x00, 0x01, 0xFF, 0x89, 0x99,
		0x3D, 0x1D, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}
