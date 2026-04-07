package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/gophics/ravenporter/cache"
)

func TestCookCommandWritesCache(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "tri.obj")
	outputPath := filepath.Join(t.TempDir(), "tri.rpcache")
	require.NoError(t, os.WriteFile(inputPath, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	app := &cli.App{
		Commands: []*cli.Command{cookCmd()},
	}

	err := app.Run([]string{"app", "cook", "--out", outputPath, inputPath})
	require.NoError(t, err)

	asset, err := cache.Open(outputPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, asset.Close()) }()
	require.NotNil(t, asset.Asset)
	require.Len(t, asset.Meshes, 1)
}

func TestCookCommandUsesDefaultOutputPath(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "tri.obj")
	require.NoError(t, os.WriteFile(inputPath, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	app := &cli.App{
		Commands: []*cli.Command{cookCmd()},
	}

	err := app.Run([]string{"app", "cook", inputPath})
	require.NoError(t, err)

	_, statErr := os.Stat(defaultCookPath(inputPath))
	require.NoError(t, statErr)
}

func TestInfoCommandReadsCache(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "tri.obj")
	outputPath := filepath.Join(inputDir, "tri.rpcache")
	require.NoError(t, os.WriteFile(inputPath, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	app := &cli.App{
		Commands: []*cli.Command{cookCmd(), infoCmd()},
	}

	require.NoError(t, app.Run([]string{"app", "cook", "--out", outputPath, inputPath}))
	require.NoError(t, app.Run([]string{"app", "info", "--json", outputPath}))
}
