package main

import (
	"flag"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/gophics/ravenporter"
)

func TestBuildImportOptionsRejectsUnknownPreset(t *testing.T) {
	ctx := newCLIContext(t, importCmd().Flags, "--preset", "godot")

	_, err := buildImportOptions(ctx, quietLogger(), true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown preset")
}

func TestBuildImportOptionsWiresGenericFlags(t *testing.T) {
	ctx := newCLIContext(t, importCmd().Flags,
		"--scale", "2.5",
		"--up-axis", "Z",
		"--embed-textures",
		"--decode-max-file-size", "4096",
		"--decode-max-vertices", "123",
	)

	opts, err := buildImportOptions(ctx, quietLogger(), true)
	require.NoError(t, err)
	profile, err := ravenporter.ResolveProfile(opts...)
	require.NoError(t, err)
	require.NotNil(t, profile.Decode.MaxFileSize)
	require.NotNil(t, profile.Decode.MaxVertices)
	require.NotNil(t, profile.Process.GlobalScale)
	require.NotNil(t, profile.Process.TargetUpAxis)
	assert.Equal(t, int64(4096), *profile.Decode.MaxFileSize)
	assert.Equal(t, 123, *profile.Decode.MaxVertices)
	assert.Equal(t, 2.5, *profile.Process.GlobalScale)
	assert.Equal(t, []string{"embed-textures"}, profile.Process.EnabledSteps)
	assert.Equal(t, "Z", *profile.Process.TargetUpAxis)
}

func TestBuildImportOptionsProfilePrecedence(t *testing.T) {
	scale := 2.0
	profile := ravenporter.Profile{
		Version: ravenporter.ProfileVersion,
		Preset:  ravenporter.BuiltInPresetFast,
		Process: ravenporter.ProcessProfile{
			GlobalScale: &scale,
		},
	}
	path := filepath.Join(t.TempDir(), "ravenporter.toml")
	require.NoError(t, ravenporter.SaveProfile(path, profile))

	ctx := newCLIContext(t, importCmd().Flags,
		"--profile-file", path,
		"--preset", ravenporter.BuiltInPresetMaxQuality,
		"--scale", "4.0",
	)

	opts, err := buildImportOptions(ctx, quietLogger(), true)
	require.NoError(t, err)
	resolvedProfile, err := ravenporter.ResolveProfile(opts...)
	require.NoError(t, err)
	assert.Equal(t, ravenporter.BuiltInPresetMaxQuality, resolvedProfile.Preset)
	require.NotNil(t, resolvedProfile.Process.GlobalScale)
	assert.InDelta(t, 4.0, *resolvedProfile.Process.GlobalScale, 0.0001)
}

func TestBuildImportOptionsProcessStepOverrides(t *testing.T) {
	ctx := newCLIContext(t, importCmd().Flags,
		"--enable-step", "find-instances",
		"--enable-step", "report-stats",
		"--disable-step", "decode-pixels",
	)

	opts, err := buildImportOptions(ctx, quietLogger(), true)
	require.NoError(t, err)

	profile, err := ravenporter.ResolveProfile(opts...)
	require.NoError(t, err)
	assert.Equal(t, []string{"find-instances", "report-stats"}, profile.Process.EnabledSteps)
	assert.Equal(t, []string{"decode-pixels"}, profile.Process.DisabledSteps)
}

func TestBuildImportOptionsRejectsUnknownProcessStep(t *testing.T) {
	ctx := newCLIContext(t, importCmd().Flags, "--enable-step", "not-a-step")

	_, err := buildImportOptions(ctx, quietLogger(), true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown process step")
}

func TestProfileExportCommandWritesTOML(t *testing.T) {
	out := filepath.Join(t.TempDir(), "ravenporter.toml")
	app := &cli.App{Commands: []*cli.Command{profileCmd()}}

	err := app.Run([]string{
		"app",
		"profile",
		"export",
		"--out", out,
		"--preset", ravenporter.BuiltInPresetFast,
		"--embed-textures",
		"--scale", "3",
	})
	require.NoError(t, err)

	profile, err := ravenporter.LoadProfile(out)
	require.NoError(t, err)
	assert.Equal(t, ravenporter.BuiltInPresetFast, profile.Preset)
	assert.Equal(t, []string{"embed-textures"}, profile.Process.EnabledSteps)
	require.NotNil(t, profile.Process.GlobalScale)
	assert.InDelta(t, 3.0, *profile.Process.GlobalScale, 0.0001)
}

func TestProfileExportCommandWritesProcessSteps(t *testing.T) {
	out := filepath.Join(t.TempDir(), "ravenporter.toml")
	app := &cli.App{Commands: []*cli.Command{profileCmd()}}

	err := app.Run([]string{
		"app",
		"profile",
		"export",
		"--out", out,
		"--enable-step", "find-instances",
		"--disable-step", "decode-pixels",
	})
	require.NoError(t, err)

	profile, err := ravenporter.LoadProfile(out)
	require.NoError(t, err)
	assert.Equal(t, []string{"find-instances"}, profile.Process.EnabledSteps)
	assert.Equal(t, []string{"decode-pixels"}, profile.Process.DisabledSteps)
}

func newCLIContext(t *testing.T, flags []cli.Flag, args ...string) *cli.Context {
	t.Helper()

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.SetOutput(ioDiscard{})
	for _, fl := range flags {
		require.NoError(t, fl.Apply(set))
	}
	require.NoError(t, set.Parse(args))
	return cli.NewContext(nil, set, nil)
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
