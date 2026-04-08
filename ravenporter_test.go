package ravenporter_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

func TestSupportedFormats(t *testing.T) {
	formats := ravenporter.SupportedFormats()
	require.NotEmpty(t, formats)
	assert.Contains(t, formats, ir.FormatGLTF)
	assert.Contains(t, formats, ir.FormatOBJ)
	assert.Contains(t, formats, ir.FormatWebP)
}

func TestSupportedExtensions(t *testing.T) {
	extensions := ravenporter.SupportedExtensions()
	require.NotEmpty(t, extensions)
	assert.Contains(t, extensions, ".gltf")
	assert.Contains(t, extensions, ".obj")
	assert.Contains(t, extensions, ".webp")
	assert.True(t, ravenporter.SupportsExtension(".glb"))
	assert.True(t, ravenporter.SupportsExtension(".GLB"))
	assert.False(t, ravenporter.SupportsExtension(".notreal"))
}

func TestImportBytes(t *testing.T) {
	result, err := ravenporter.ImportBytes(
		context.Background(),
		[]byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"),
		"scene.obj",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ir.FormatOBJ, result.Report.Source.DetectedFormat)
	assert.Len(t, result.Asset.Meshes, 1)
}

func TestImportReader(t *testing.T) {
	result, err := ravenporter.ImportReader(
		context.Background(),
		bytes.NewReader([]byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")),
		"scene.obj",
		ravenporter.WithRegistry(ravenporter.NewRegistry()),
		ravenporter.WithProcessFlags(0),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ir.FormatOBJ, result.Report.Source.DetectedFormat)
	assert.Len(t, result.Asset.Meshes, 1)
}

func TestParseProfileAndResolveOptions(t *testing.T) {
	profileData := []byte("version = 1\npreset = \"quality\"\n\n[decode]\nmax_file_size = 4096\nmax_vertices = 256\nmax_image_pixels = 1024\nmax_audio_samples = 2048\n")

	parsed, err := ravenporter.ParseProfileTOML(profileData)
	require.NoError(t, err)
	require.NotNil(t, parsed.Decode.MaxFileSize)
	require.NotNil(t, parsed.Decode.MaxVertices)
	require.NotNil(t, parsed.Decode.MaxImagePixels)
	require.NotNil(t, parsed.Decode.MaxAudioSamples)
	assert.EqualValues(t, 4096, *parsed.Decode.MaxFileSize)
	assert.Equal(t, 256, *parsed.Decode.MaxVertices)
	assert.Equal(t, 1024, *parsed.Decode.MaxImagePixels)
	assert.Equal(t, 2048, *parsed.Decode.MaxAudioSamples)

	resolved, err := ravenporter.ResolveProfile(
		ravenporter.WithProfile(parsed),
		ravenporter.WithDecodeMaxImagePixels(8192),
		ravenporter.WithDecodeMaxAudioSamples(16384),
	)
	require.NoError(t, err)
	require.NotNil(t, resolved.Decode.MaxImagePixels)
	require.NotNil(t, resolved.Decode.MaxAudioSamples)
	assert.Equal(t, 8192, *resolved.Decode.MaxImagePixels)
	assert.Equal(t, 16384, *resolved.Decode.MaxAudioSamples)
}

func TestBuiltInPresetNames(t *testing.T) {
	names := ravenporter.BuiltInPresetNames()
	require.NotEmpty(t, names)
	assert.Contains(t, names, ravenporter.BuiltInPresetFast)
	assert.Contains(t, names, ravenporter.BuiltInPresetQuality)
	assert.Contains(t, names, ravenporter.BuiltInPresetMaxQuality)
}

func TestLoadMaskPrunesScene(t *testing.T) {
	result, err := ravenporter.ImportBytes(
		context.Background(),
		[]byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"),
		"scene.obj",
		ravenporter.WithLoadMask(ravenporter.LoadMaterials),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Asset.Meshes)
	require.NotEmpty(t, result.Asset.Nodes)
	assert.Equal(t, ir.NoIndex, result.Asset.Nodes[0].MeshIndex)
}

func TestResolveProfileRuntimeOnlyOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []ravenporter.Option
		check   func(t *testing.T, profile ravenporter.Profile)
	}{
		{
			name: "process flags still resolve to profile content",
			options: []ravenporter.Option{
				ravenporter.WithProcessFlags(process.PPEmbedTextures),
			},
			check: func(t *testing.T, profile ravenporter.Profile) {
				t.Helper()
				assert.Equal(t, ravenporter.ProfileVersion, profile.Version)
				assert.Empty(t, profile.Preset)
				assert.Equal(t, []string{"embed-textures"}, profile.Process.EnabledSteps)
			},
		},
		{
			name: "batch concurrency is ignored by profile resolution",
			options: []ravenporter.Option{
				ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
				ravenporter.WithBatchConcurrency(1),
			},
			check: func(t *testing.T, profile ravenporter.Profile) {
				t.Helper()
				assert.Equal(t, ravenporter.ProfileVersion, profile.Version)
				assert.Equal(t, ravenporter.BuiltInPresetQuality, profile.Preset)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := ravenporter.ResolveProfile(tt.options...)
			require.NoError(t, err)
			tt.check(t, profile)
		})
	}
}

func TestImportSingleAssetEntryPoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scene.obj")
	require.NoError(t, os.WriteFile(path, []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	fsys := fstest.MapFS{
		"scene.obj": &fstest.MapFile{Data: []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")},
	}

	tests := []struct {
		name     string
		importFn func() (*ravenporter.Result, error)
	}{
		{
			name: "ImportPath",
			importFn: func() (*ravenporter.Result, error) {
				return ravenporter.ImportPath(context.Background(), path)
			},
		},
		{
			name: "ImportFS",
			importFn: func() (*ravenporter.Result, error) {
				return ravenporter.ImportFS(context.Background(), fsys, "scene.obj")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.importFn()
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, ir.FormatOBJ, result.Report.Source.DetectedFormat)
		})
	}
}

func TestImportBatchEntryPoints(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.obj"), []byte("o A\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.obj"), []byte("o B\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	fsys := fstest.MapFS{
		"assets/a.obj": &fstest.MapFile{Data: []byte("o A\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")},
		"assets/b.obj": &fstest.MapFile{Data: []byte("o B\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")},
	}

	tests := []struct {
		name     string
		importFn func() ([]*ravenporter.Result, error)
	}{
		{
			name: "ImportDir",
			importFn: func() ([]*ravenporter.Result, error) {
				return ravenporter.ImportDir(context.Background(), dir, ravenporter.WithBatchConcurrency(1))
			},
		},
		{
			name: "ImportFSDir",
			importFn: func() ([]*ravenporter.Result, error) {
				return ravenporter.ImportFSDir(context.Background(), fsys, "assets", ravenporter.WithBatchConcurrency(1))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := tt.importFn()
			require.NoError(t, err)
			require.Len(t, results, 2)
			assert.Equal(t, ir.FormatOBJ, results[0].Report.Source.DetectedFormat)
			assert.Equal(t, ir.FormatOBJ, results[1].Report.Source.DetectedFormat)
		})
	}
}

func TestSaveAndLoadProfile(t *testing.T) {
	scale := 2.5
	profile := ravenporter.Profile{
		Version: ravenporter.ProfileVersion,
		Preset:  ravenporter.BuiltInPresetQuality,
		Process: ravenporter.ProcessProfile{
			GlobalScale: &scale,
		},
	}

	path := filepath.Join(t.TempDir(), "ravenporter.toml")
	require.NoError(t, ravenporter.SaveProfile(path, profile))

	loaded, err := ravenporter.LoadProfile(path)
	require.NoError(t, err)
	require.NotNil(t, loaded.Process.GlobalScale)
	assert.Equal(t, ravenporter.BuiltInPresetQuality, loaded.Preset)
	assert.InDelta(t, scale, *loaded.Process.GlobalScale, 0.0001)
}
