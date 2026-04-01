package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestBatchCommand(t *testing.T) {
	outDir := t.TempDir()
	inputDir := t.TempDir()
	obj := []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "triangle.obj"), obj, 0o644))

	app := &cli.App{
		Commands: []*cli.Command{batchCmd()},
	}

	err := app.Run([]string{"app", "batch", "--out", outDir, inputDir})
	require.NoError(t, err, "batch command should not return an error")

	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "batch should produce at least one output file")
}

func TestBatchCommandNoArgs(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{batchCmd()},
	}
	err := app.Run([]string{"app", "batch"})
	require.Error(t, err, "batch without directory should fail")
}

func TestConvertCommandRegistered(t *testing.T) {
	cmd := convertCmd()
	require.Equal(t, "convert", cmd.Name)
	require.NotNil(t, cmd.Action)
}

func TestBatchCommandFailsFast(t *testing.T) {
	inputDir := t.TempDir()
	outDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "01-broken.gltf"), []byte("{"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "02-good.obj"), []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	app := &cli.App{
		Commands: []*cli.Command{batchCmd()},
	}

	err := app.Run([]string{"app", "batch", "--out", outDir, inputDir})
	require.Error(t, err)

	entries, readErr := os.ReadDir(outDir)
	require.NoError(t, readErr)
	require.Empty(t, entries)
}
