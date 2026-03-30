package pipeline

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/process"
)

func TestResolveBuiltInPreset(t *testing.T) {
	flag, err := ResolveBuiltInPreset(BuiltInPresetMaxQuality)
	require.NoError(t, err)
	assert.Equal(t, process.PresetMaxQuality, flag)
}

func TestResolveBuiltInPresetUnknown(t *testing.T) {
	_, err := ResolveBuiltInPreset("godot")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown preset")
}

func TestProfileTOMLRoundTrip(t *testing.T) {
	maxFileSize := int64(4096)
	maxVertices := 512
	scale := 2.5
	axis := "Z"
	smoothAngle := 60.0
	sampleRate := 22050
	deboneThreshold := 0.25
	profile := Profile{
		Version: ProfileVersion,
		Preset:  BuiltInPresetQuality,
		Decode: DecodeProfile{
			MaxFileSize: &maxFileSize,
			MaxVertices: &maxVertices,
		},
		Process: ProcessProfile{
			EnabledSteps:      []string{"embed-textures", "find-instances"},
			DisabledSteps:     []string{"decode-pixels"},
			SmoothNormalAngle: &smoothAngle,
			GlobalScale:       &scale,
			TargetUpAxis:      &axis,
			RemoveFlags:       []string{"normals", "texcoord0"},
			TargetSampleRate:  &sampleRate,
			DegenerateMode:    stringPtr("convert"),
			DeboneThreshold:   &deboneThreshold,
		},
	}

	data, err := profile.MarshalTOML()
	require.NoError(t, err)

	parsed, err := ParseProfileTOML(data)
	require.NoError(t, err)
	assert.Equal(t, profile.Version, parsed.Version)
	assert.Equal(t, profile.Preset, parsed.Preset)
	require.NotNil(t, parsed.Decode.MaxFileSize)
	require.NotNil(t, parsed.Decode.MaxVertices)
	require.NotNil(t, parsed.Process.GlobalScale)
	require.NotNil(t, parsed.Process.TargetUpAxis)
	require.NotNil(t, parsed.Process.SmoothNormalAngle)
	require.NotNil(t, parsed.Process.TargetSampleRate)
	require.NotNil(t, parsed.Process.DegenerateMode)
	require.NotNil(t, parsed.Process.DeboneThreshold)
	assert.Equal(t, maxFileSize, *parsed.Decode.MaxFileSize)
	assert.Equal(t, maxVertices, *parsed.Decode.MaxVertices)
	assert.InDelta(t, scale, *parsed.Process.GlobalScale, 0.0001)
	assert.Equal(t, axis, *parsed.Process.TargetUpAxis)
	assert.InDelta(t, smoothAngle, *parsed.Process.SmoothNormalAngle, 0.0001)
	assert.Equal(t, sampleRate, *parsed.Process.TargetSampleRate)
	assert.Equal(t, "convert", *parsed.Process.DegenerateMode)
	assert.InDelta(t, deboneThreshold, *parsed.Process.DeboneThreshold, 0.0001)
	assert.Equal(t, []string{"embed-textures", "find-instances"}, parsed.Process.EnabledSteps)
	assert.Equal(t, []string{"decode-pixels"}, parsed.Process.DisabledSteps)
	assert.Equal(t, []string{"normals", "texcoord0"}, parsed.Process.RemoveFlags)
}

func TestProfileSaveLoad(t *testing.T) {
	scale := 1.5
	profile := Profile{
		Version: ProfileVersion,
		Preset:  BuiltInPresetFast,
		Process: ProcessProfile{
			GlobalScale: &scale,
		},
	}

	path := filepath.Join(t.TempDir(), "ravenporter.toml")
	require.NoError(t, SaveProfile(path, profile))

	loaded, err := LoadProfile(path)
	require.NoError(t, err)
	require.NotNil(t, loaded.Process.GlobalScale)
	assert.Equal(t, BuiltInPresetFast, loaded.Preset)
	assert.InDelta(t, scale, *loaded.Process.GlobalScale, 0.0001)
}

func TestApplyProfile(t *testing.T) {
	maxVertices := 1024
	scale := 3.0
	axis := "Z"
	maxBoneWeights := 4
	targetChannels := 1
	profile := Profile{
		Version: ProfileVersion,
		Preset:  BuiltInPresetQuality,
		Decode: DecodeProfile{
			MaxVertices: &maxVertices,
		},
		Process: ProcessProfile{
			EnabledSteps:    []string{"embed-textures"},
			DisabledSteps:   []string{"decode-pixels"},
			GlobalScale:     &scale,
			TargetUpAxis:    &axis,
			MaxBoneWeights:  &maxBoneWeights,
			TargetChannels:  &targetChannels,
			RemoveFlags:     []string{"normals"},
			DegenerateMode:  stringPtr("convert"),
			DeboneThreshold: float64Ptr(0.5),
		},
	}

	cfg := config{}
	err := applyProfile(&cfg, profile)
	require.NoError(t, err)
	assert.Equal(t, maxVertices, cfg.DecodeOpts.MaxVertices)
	assert.Equal(t, scale, cfg.ProcessOpts.GlobalScale)
	assert.Equal(t, maxBoneWeights, cfg.ProcessOpts.MaxBoneWeights)
	assert.Equal(t, targetChannels, cfg.ProcessOpts.TargetChannels)
	assert.Equal(t, process.CompNormals, cfg.ProcessOpts.RemoveFlags)
	assert.Equal(t, process.DegenerateModeConvert, cfg.ProcessOpts.DegenerateMode)
	assert.InDelta(t, 0.5, cfg.ProcessOpts.DeboneThreshold, 0.0001)
	assert.Equal(t,
		(process.PresetQuality&^process.PPDecodePixels)|
			process.PPEmbedTextures|
			process.PPGlobalScale|
			process.PPFixUpAxis|
			process.PPLimitBoneWeights|
			process.PPMixdownAudio|
			process.PPRemoveComponent|
			process.PPRemoveDegenerates|
			process.PPDebone,
		cfg.ProcessFlags,
	)
}

func TestMergeProfiles(t *testing.T) {
	maxFileSize := int64(1024)
	scale := 1.0
	maxVertices := 256
	base := Profile{
		Version: ProfileVersion,
		Preset:  BuiltInPresetFast,
		Decode: DecodeProfile{
			MaxFileSize: &maxFileSize,
		},
		Process: ProcessProfile{
			GlobalScale:  &scale,
			EnabledSteps: []string{"embed-textures"},
		},
	}
	override := Profile{
		Version: ProfileVersion,
		Preset:  BuiltInPresetQuality,
		Decode: DecodeProfile{
			MaxVertices: &maxVertices,
		},
		Process: ProcessProfile{
			EnabledSteps:  []string{"find-instances"},
			DisabledSteps: []string{"decode-pixels"},
			RemoveFlags:   []string{"normals"},
		},
	}

	merged := MergeProfiles(base, override)
	assert.Equal(t, BuiltInPresetQuality, merged.Preset)
	require.NotNil(t, merged.Decode.MaxFileSize)
	require.NotNil(t, merged.Decode.MaxVertices)
	assert.Equal(t, maxFileSize, *merged.Decode.MaxFileSize)
	assert.Equal(t, maxVertices, *merged.Decode.MaxVertices)
	assert.Equal(t, []string{"embed-textures", "find-instances"}, merged.Process.EnabledSteps)
	assert.Equal(t, []string{"decode-pixels"}, merged.Process.DisabledSteps)
	assert.Equal(t, []string{"normals"}, merged.Process.RemoveFlags)
}

func stringPtr(value string) *string {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}
