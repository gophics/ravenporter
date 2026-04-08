package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/gophics/ravenporter/validate"
)

const mockFormat = ir.FormatID("mock")

type mockDecoder struct {
	scene *ir.Asset
	err   error
}

func (m *mockDecoder) Probe(_ io.ReadSeeker) bool { return true }
func (m *mockDecoder) Extensions() []string       { return []string{".glb"} }
func (m *mockDecoder) FormatName() string         { return "Mock GLB" }

func (m *mockDecoder) Decode(_ detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if opts.Reporter != nil {
		opts.Reporter.AddDependency("buffer", "shared.bin", "buffer", "mock")
		opts.Reporter.AddProvenanceNote("mock", "decode")
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.scene, nil
}

type factoryDecoder struct {
	build func() *ir.Asset
}

func (d *factoryDecoder) Probe(_ io.ReadSeeker) bool { return true }
func (d *factoryDecoder) Extensions() []string       { return []string{".glb"} }
func (d *factoryDecoder) FormatName() string         { return "Factory Mock GLB" }

func (d *factoryDecoder) Decode(_ detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if opts.Reporter != nil {
		opts.Reporter.AddDependency("buffer", "shared.bin", "buffer", "mock")
		opts.Reporter.AddProvenanceNote("mock", "decode")
	}
	if d.build == nil {
		return &ir.Asset{}, nil
	}
	return d.build(), nil
}

func setupMockRegistry(d detect.Decoder) *detect.Registry {
	reg := detect.NewRegistry()
	if d != nil {
		reg.Register(ir.FormatGLB, d)
	}
	return reg
}

func gltfMagic() *bytes.Reader {
	return bytes.NewReader([]byte("glTF_mock_body_data"))
}

type sidecarDecoder struct{}

func (d *sidecarDecoder) Probe(_ io.ReadSeeker) bool { return true }
func (d *sidecarDecoder) Extensions() []string       { return []string{".mock"} }
func (d *sidecarDecoder) FormatName() string         { return "Mock" }

func (d *sidecarDecoder) Decode(_ detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if opts.FS == nil {
		return nil, errors.New("missing decode fs")
	}
	rc, err := opts.FS.Open("sidecar.txt")
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return &ir.Asset{
		Name:     strings.TrimSpace(string(data)),
		Metadata: ir.AssetMetadata{SourceFormat: mockFormat},
	}, nil
}

type textureDecoder struct{}

func (d *textureDecoder) Probe(_ io.ReadSeeker) bool { return true }
func (d *textureDecoder) Extensions() []string       { return []string{".mock"} }
func (d *textureDecoder) FormatName() string         { return "MockTexture" }

func (d *textureDecoder) Decode(_ detect.ReadSeekerAt, _ detect.DecodeOptions) (*ir.Asset, error) {
	return &ir.Asset{
		Images:   []*ir.ImageAsset{{Name: "AlbedoImage", SourcePath: "albedo.png"}},
		Textures: []*ir.Texture{{Name: "Albedo", ImageIndex: 0}},
		Metadata: ir.AssetMetadata{SourceFormat: mockFormat},
	}, nil
}

func setupRegistry(decoder detect.Decoder) *detect.Registry {
	reg := detect.NewRegistry()
	reg.Register(mockFormat, decoder)
	return reg
}

func TestImportHappyPath(t *testing.T) {
	scene := &ir.Asset{
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatGLB},
		Nodes: []ir.Node{{
			Name:          "Root",
			MeshIndex:     ir.NoIndex,
			SkinIndex:     ir.NoIndex,
			CameraIndex:   ir.NoIndex,
			LightIndex:    ir.NoIndex,
			LODGroupIndex: ir.NoIndex,
		}},
		Images:   []*ir.ImageAsset{{Name: "AlbedoImage", SourcePath: "albedo.png"}},
		Textures: []*ir.Texture{{Name: "Albedo", ImageIndex: 0}},
	}
	reg := setupMockRegistry(&mockDecoder{scene: scene})

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{
		Registry: reg,
		Preset:   BuiltInPresetQuality,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, scene, out.Asset)
	assert.Equal(t, ir.FormatGLB, out.Report.Source.DetectedFormat)
	assert.Equal(t, "test.glb", out.Report.Source.InputName)
	assert.Equal(t, scene.Metadata, out.Report.Source.Metadata)
	assert.Len(t, out.Report.Dependencies, 2)
	assert.Contains(t, out.Report.Dependencies, Dependency{
		Kind:       "image",
		Path:       "albedo.png",
		Relation:   "image",
		ReportedBy: "asset",
	})
	assert.Contains(t, out.Report.Dependencies, Dependency{
		Kind:       "buffer",
		Path:       "shared.bin",
		Relation:   "buffer",
		ReportedBy: "mock",
	})
	require.NotNil(t, out.Report.Source.Notes)
	assert.Equal(t, []string{"decode"}, out.Report.Source.Notes["mock"])
	assert.Equal(t, 1, out.Report.Summary.Nodes)
	assert.Equal(t, 1, out.Report.Summary.Textures)
}

func TestImportSummaryIncludesInstances(t *testing.T) {
	scene := &ir.Asset{
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatGLB},
		Meshes:   []*ir.Mesh{{Name: "Shared"}},
		Nodes: []ir.Node{
			{Name: "A", MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex, LODGroupIndex: ir.NoIndex},
			{Name: "B", MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex, LODGroupIndex: ir.NoIndex},
		},
	}

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{
		Registry: setupMockRegistry(&mockDecoder{scene: scene}),
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, 1, out.Report.Summary.InstancedMeshes)
	assert.Equal(t, 2, out.Report.Summary.InstanceNodes)
}

func TestImportDetectError(t *testing.T) {
	reg := setupMockRegistry(nil)
	out, err := importReader(context.Background(), strings.NewReader("INVALID_DATA_XXX"), "fail.mock", config{Registry: reg})
	require.Error(t, err)
	require.NotNil(t, out)
	assert.Nil(t, out.Asset)
	assert.Contains(t, err.Error(), "pipeline: no decoder registered for format")
	require.Len(t, out.Report.Issues, 1)
	assert.Equal(t, "NO_DECODER", out.Report.Issues[0].Code)
}

func TestImportDecodeError(t *testing.T) {
	decodeErr := errors.New("simulated decode error")
	reg := setupMockRegistry(&mockDecoder{err: decodeErr})

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{Registry: reg})
	require.ErrorIs(t, err, decodeErr)
	require.NotNil(t, out)
	assert.Nil(t, out.Asset)
	require.Len(t, out.Report.Issues, 1)
	assert.Equal(t, "DECODE_FAILED", out.Report.Issues[0].Code)
}

func TestImportStructuralValidationFail(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 1,
					Indices:     []uint32{99},
				},
			}},
		}},
	}
	reg := setupMockRegistry(&mockDecoder{scene: scene})

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{Registry: reg})
	require.Error(t, err)
	require.NotNil(t, out)
	assert.Nil(t, out.Asset)
	assert.Contains(t, err.Error(), "structural validation failed")
	assert.Contains(t, issueCodes(out.Report.Issues), validate.CodeIndexOutOfBounds)
	assert.Equal(t, 1, out.Report.Summary.Meshes)
}

func TestImportContextCancellation(t *testing.T) {
	reg := setupMockRegistry(&mockDecoder{scene: &ir.Asset{}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out, err := importReader(ctx, gltfMagic(), "test.glb", config{Registry: reg})
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, out)
	assert.Nil(t, out.Asset)
	require.Len(t, out.Report.Issues, 1)
	assert.Equal(t, "CONTEXT_CANCELED", out.Report.Issues[0].Code)
}

func TestImportBatchConcurrent(t *testing.T) {
	scene := func() *ir.Asset {
		return &ir.Asset{
			Nodes: []ir.Node{{
				Name:          "Root",
				MeshIndex:     ir.NoIndex,
				SkinIndex:     ir.NoIndex,
				CameraIndex:   ir.NoIndex,
				LightIndex:    ir.NoIndex,
				LODGroupIndex: ir.NoIndex,
			}},
		}
	}
	reg := setupMockRegistry(&factoryDecoder{build: scene})

	results, err := importBatch(context.Background(), []batchInput{
		{Reader: gltfMagic(), Filename: "test1.glb", Path: "dir/test1.glb"},
		{Reader: gltfMagic(), Filename: "test2.glb", Path: "dir/test2.glb"},
		{Reader: gltfMagic(), Filename: "test3.glb", Path: "dir/test3.glb"},
	}, config{Registry: reg, workerLimit: 2})
	require.NoError(t, err)
	require.Len(t, results, 3)
	for i, res := range results {
		require.NotNil(t, res)
		expected := scene()
		expected.NormalizeGraph()
		assert.Equal(t, expected, res.Asset)
		assert.Equal(t, fmt.Sprintf("dir/test%d.glb", i+1), res.Report.Source.InputPath)
	}
}

func TestWithBatchConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		wantWorkers int
		wantErr     string
	}{
		{
			name:        "explicit limit",
			limit:       3,
			wantWorkers: 3,
		},
		{
			name:        "zero uses default",
			limit:       0,
			wantWorkers: runtime.GOMAXPROCS(0),
		},
		{
			name:    "negative limit rejected",
			limit:   -1,
			wantErr: "batch concurrency must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := resolveOptions(WithBatchConcurrency(tt.limit))
			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantWorkers, cfg.workers())
		})
	}
}

func TestImportBatchPartialFailure(t *testing.T) {
	targetErr := errors.New("boom")
	failReg := detect.NewRegistry()
	failReg.Register(ir.FormatGLB, &dynamicDecoder{scene: &ir.Asset{}, targetErr: targetErr})

	results, err := importBatch(context.Background(), []batchInput{
		{Reader: bytes.NewReader([]byte("glTF_pass")), Filename: "pass1.glb"},
		{Reader: bytes.NewReader([]byte("glTF_fail")), Filename: "fail.glb"},
		{Reader: bytes.NewReader([]byte("glTF_pass")), Filename: "pass2.glb"},
	}, config{Registry: failReg, workerLimit: 2})
	assert.ErrorIs(t, err, targetErr)
	assert.Nil(t, results)
}

func TestImportPathEmbedsTextures(t *testing.T) {
	dir := t.TempDir()
	scenePath := filepath.Join(dir, "scene.mock")
	texturePath := filepath.Join(dir, "albedo.png")
	require.NoError(t, os.WriteFile(scenePath, []byte("scene"), 0o644))
	require.NoError(t, os.WriteFile(texturePath, []byte{1, 2, 3}, 0o644))

	result, err := ImportPath(
		context.Background(),
		scenePath,
		WithRegistry(setupRegistry(&textureDecoder{})),
		WithProcessFlags(process.PPEmbedTextures),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Asset.Textures, 1)
	require.Len(t, result.Asset.Images, 1)
	assert.Equal(t, []byte{1, 2, 3}, result.Asset.Images[result.Asset.Textures[0].ImageIndex].Compressed)
	assert.Empty(t, result.Asset.Images[result.Asset.Textures[0].ImageIndex].SourcePath)
}

func TestImportFSEmbedsTextures(t *testing.T) {
	fsys := fstest.MapFS{
		"assets/scene.mock": &fstest.MapFile{Data: []byte("scene")},
		"assets/albedo.png": &fstest.MapFile{Data: []byte{4, 5, 6}},
	}

	result, err := ImportFS(
		context.Background(),
		fsys,
		"assets/scene.mock",
		WithRegistry(setupRegistry(&textureDecoder{})),
		WithProcessFlags(process.PPEmbedTextures),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Asset.Textures, 1)
	require.Len(t, result.Asset.Images, 1)
	assert.Equal(t, []byte{4, 5, 6}, result.Asset.Images[result.Asset.Textures[0].ImageIndex].Compressed)
	assert.Empty(t, result.Asset.Images[result.Asset.Textures[0].ImageIndex].SourcePath)
}

func TestImportReader(t *testing.T) {
	scene := &ir.Asset{Metadata: ir.AssetMetadata{SourceFormat: ir.FormatGLB}}
	reg := setupMockRegistry(&mockDecoder{scene: scene})

	result, err := ImportReader(context.Background(), gltfMagic(), "memory.glb", WithRegistry(reg))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, scene, result.Asset)
	assert.Equal(t, "memory.glb", result.Report.Source.InputName)
}

func TestImportBytes(t *testing.T) {
	scene := &ir.Asset{Metadata: ir.AssetMetadata{SourceFormat: ir.FormatGLB}}
	reg := setupMockRegistry(&mockDecoder{scene: scene})

	result, err := ImportBytes(context.Background(), []byte("glTF_mock_body_data"), "memory.glb", WithRegistry(reg))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, scene, result.Asset)
	assert.Equal(t, ir.FormatGLB, result.Report.Source.DetectedFormat)
}

func TestImportDirUsesPerFileFS(t *testing.T) {
	root := t.TempDir()
	alphaDir := filepath.Join(root, "alpha")
	betaDir := filepath.Join(root, "beta")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.MkdirAll(betaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "first.mock"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "sidecar.txt"), []byte("alpha"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "second.mock"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "sidecar.txt"), []byte("beta"), 0o644))

	results, err := importDir(context.Background(), root, config{
		Registry: setupRegistry(&sidecarDecoder{}),
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	namesByPath := make(map[string]string, len(results))
	for _, result := range results {
		require.NotNil(t, result)
		namesByPath[filepath.Base(result.Report.Source.InputPath)] = result.Asset.Name
	}
	assert.Equal(t, "alpha", namesByPath["first.mock"])
	assert.Equal(t, "beta", namesByPath["second.mock"])
}

func TestImportFSDirUsesPerFileFS(t *testing.T) {
	fsys := fstest.MapFS{
		"alpha/first.mock":  &fstest.MapFile{Data: []byte("a")},
		"alpha/sidecar.txt": &fstest.MapFile{Data: []byte("alpha")},
		"beta/second.mock":  &fstest.MapFile{Data: []byte("b")},
		"beta/sidecar.txt":  &fstest.MapFile{Data: []byte("beta")},
	}

	results, err := importFSDir(context.Background(), fsys, ".", config{
		Registry: setupRegistry(&sidecarDecoder{}),
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	namesByPath := make(map[string]string, len(results))
	for _, result := range results {
		require.NotNil(t, result)
		namesByPath[result.Report.Source.InputPath] = result.Asset.Name
	}
	assert.Equal(t, "alpha", namesByPath["alpha/first.mock"])
	assert.Equal(t, "beta", namesByPath["beta/second.mock"])
}

func TestImportProcessingAndSemanticValidation(t *testing.T) {
	scene := &ir.Asset{
		Nodes: []ir.Node{{
			Name:          "Root",
			MeshIndex:     ir.NoIndex,
			SkinIndex:     ir.NoIndex,
			CameraIndex:   ir.NoIndex,
			LightIndex:    ir.NoIndex,
			LODGroupIndex: ir.NoIndex,
		}},
		Materials: []*ir.Material{{
			Name:           "PBR_Bad",
			MetallicFactor: 2.5,
		}},
	}
	reg := setupMockRegistry(&mockDecoder{scene: scene})

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{
		Registry:     reg,
		ProcessFlags: process.PPFindInstances,
		Logger:       logger,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, scene, out.Asset)
	assert.Contains(t, logBuf.String(), validate.CodePBROutOfRange)
	assert.Contains(t, issueCodes(out.Report.Issues), validate.CodePBROutOfRange)
}

func TestImportLoadMaskPrunesMeshesAndRefs(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{Primitives: []ir.Primitive{{MaterialIndex: 0}}}},
		Materials: []*ir.Material{{
			BaseColorTexture: &ir.TextureRef{TextureIndex: 0},
		}},
		Textures: []*ir.Texture{{Name: "tex"}},
		Nodes: []ir.Node{{
			Name:          "Root",
			MeshIndex:     0,
			SkinIndex:     0,
			CameraIndex:   0,
			LightIndex:    0,
			LODGroupIndex: ir.NoIndex,
		}},
		Skeletons: []*ir.Skeleton{{Name: "Rig"}},
		Cameras:   []*ir.Camera{{Name: "Cam"}},
		Lights: []*ir.Light{{
			Name:       "Light",
			IESProfile: &ir.TextureRef{TextureIndex: 0},
		}},
		CollisionMeshes: []*ir.CollisionMesh{{MeshIndex: 0, NodeIndex: 0}},
		Fonts: []*ir.Font{{
			Name: "Bitmap",
			Bitmap: &ir.BitmapFontData{
				AtlasIndex: 0,
				AtlasPath:  "atlas.png",
			},
		}},
		Images:   []*ir.ImageAsset{{Name: "Image"}},
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatGLB},
	}

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{
		Registry:    setupMockRegistry(&mockDecoder{scene: scene}),
		loadMask:    LoadMaterials | LoadTextures | LoadFonts,
		loadMaskSet: true,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Asset)

	assert.Empty(t, out.Asset.Meshes)
	assert.Empty(t, out.Asset.Skeletons)
	assert.Empty(t, out.Asset.Cameras)
	assert.Empty(t, out.Asset.Lights)
	assert.Empty(t, out.Asset.Images)
	assert.Equal(t, ir.NoIndex, out.Asset.Nodes[0].MeshIndex)
	assert.Equal(t, ir.NoIndex, out.Asset.Nodes[0].SkinIndex)
	assert.Equal(t, ir.NoIndex, out.Asset.Nodes[0].CameraIndex)
	assert.Equal(t, ir.NoIndex, out.Asset.Nodes[0].LightIndex)
	assert.Equal(t, ir.NoIndex, out.Asset.CollisionMeshes[0].MeshIndex)
	assert.Equal(t, ir.NoIndex, out.Asset.Fonts[0].Bitmap.AtlasIndex)
	assert.Empty(t, out.Asset.Fonts[0].Bitmap.AtlasPath)
	assert.NotNil(t, out.Asset.Materials[0].BaseColorTexture)
}

func TestImportLoadMaskPrunesTextures(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{{
			BaseColorTexture: &ir.TextureRef{TextureIndex: 0},
			Clearcoat:        &ir.MaterialClearcoat{Texture: &ir.TextureRef{TextureIndex: 0}},
		}},
		Textures: []*ir.Texture{{Name: "tex"}},
		Lights: []*ir.Light{{
			Name:       "Light",
			IESProfile: &ir.TextureRef{TextureIndex: 0},
		}},
		Nodes:    []ir.Node{{MeshIndex: ir.NoIndex, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: 0, LODGroupIndex: ir.NoIndex}},
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatGLB},
	}

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{
		Registry:    setupMockRegistry(&mockDecoder{scene: scene}),
		loadMask:    LoadMaterials | LoadLights,
		loadMaskSet: true,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Asset)

	assert.Empty(t, out.Asset.Textures)
	assert.Nil(t, out.Asset.Materials[0].BaseColorTexture)
	assert.Nil(t, out.Asset.Materials[0].Clearcoat.Texture)
	assert.Nil(t, out.Asset.Lights[0].IESProfile)
	assert.Empty(t, issueCodes(out.Report.Issues))
}

func TestImportNormalizesGraph(t *testing.T) {
	scene := &ir.Asset{
		Scenes: []*ir.Scene{{Name: "Scene", RootNodes: []int{0}}},
		Nodes: []ir.Node{
			{
				Name:          "Root",
				ParentIndex:   0,
				Children:      []int{1},
				MeshIndex:     0,
				SkinIndex:     ir.NoIndex,
				CameraIndex:   ir.NoIndex,
				LightIndex:    ir.NoIndex,
				LODGroupIndex: ir.NoIndex,
			},
			{
				Name:          "Child",
				MeshIndex:     ir.NoIndex,
				SkinIndex:     ir.NoIndex,
				CameraIndex:   ir.NoIndex,
				LightIndex:    ir.NoIndex,
				LODGroupIndex: ir.NoIndex,
			},
		},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
				},
			}},
		}},
	}

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{
		Registry: setupMockRegistry(&mockDecoder{scene: scene}),
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Asset)
	assert.Equal(t, ir.NoIndex, out.Asset.Nodes[0].ParentIndex)
	assert.Equal(t, 0, out.Asset.Nodes[1].ParentIndex)
	assert.Equal(t, []int{0}, out.Asset.RootNodes)
	assert.Equal(t, []int{0}, out.Asset.Scenes[0].RootNodes)

	lo, hi := out.Asset.SceneBoundingBox(0)
	assert.Equal(t, [3]float32{0, 0, 0}, lo)
	assert.Equal(t, [3]float32{1, 1, 0}, hi)
}

func TestImportNormalizesGraphAfterProcessing(t *testing.T) {
	scene := &ir.Asset{
		Scenes:    []*ir.Scene{{Name: "Scene", RootNodes: []int{0}}},
		RootNodes: []int{0},
		Nodes: []ir.Node{
			{
				Name:          "Root",
				Children:      []int{1},
				MeshIndex:     ir.NoIndex,
				SkinIndex:     ir.NoIndex,
				CameraIndex:   ir.NoIndex,
				LightIndex:    ir.NoIndex,
				LODGroupIndex: ir.NoIndex,
			},
			{
				Name:          "Leaf",
				MeshIndex:     0,
				SkinIndex:     ir.NoIndex,
				CameraIndex:   ir.NoIndex,
				LightIndex:    ir.NoIndex,
				LODGroupIndex: ir.NoIndex,
			},
		},
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
				},
			}},
		}},
	}

	out, err := importReader(context.Background(), gltfMagic(), "test.glb", config{
		Registry:     setupMockRegistry(&mockDecoder{scene: scene}),
		ProcessFlags: process.PPFlattenHierarchy,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Asset)
	assert.Equal(t, []int{0, 1}, out.Asset.RootNodes)
	assert.Equal(t, []int{0, 1}, out.Asset.Scenes[0].RootNodes)
	assert.Equal(t, ir.NoIndex, out.Asset.Nodes[0].ParentIndex)
	assert.Equal(t, ir.NoIndex, out.Asset.Nodes[1].ParentIndex)
}

type dynamicDecoder struct {
	scene     *ir.Asset
	targetErr error
}

func (d *dynamicDecoder) Probe(_ io.ReadSeeker) bool { return true }
func (d *dynamicDecoder) Extensions() []string       { return []string{".glb"} }
func (d *dynamicDecoder) FormatName() string         { return "Dynamic Mock" }

func (d *dynamicDecoder) Decode(r detect.ReadSeekerAt, _ detect.DecodeOptions) (*ir.Asset, error) {
	buf := make([]byte, 20)
	n, _ := r.ReadAt(buf, 0)
	if bytes.Contains(buf[:n], []byte("_fail")) {
		return nil, d.targetErr
	}
	return d.scene, nil
}

func issueCodes(issues []Issue) []string {
	codes := make([]string, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code)
	}
	return codes
}
