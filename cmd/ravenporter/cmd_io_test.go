package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestValidateCommandJSONOutput(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "tri.obj")
	require.NoError(t, os.WriteFile(inputPath, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	app := &cli.App{
		Commands: []*cli.Command{validateCmd()},
	}

	stdout, err := captureStdout(t, func() error {
		return app.Run([]string{"app", "validate", "--json", inputPath})
	})
	require.NoError(t, err)

	var output validationOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &output))
	require.Equal(t, "tri.obj", output.File)
	require.True(t, output.OK)
}

func TestInfoCommandJSONMultipleFilesWritesJSONArray(t *testing.T) {
	inputDir := t.TempDir()
	first := filepath.Join(inputDir, "a.obj")
	second := filepath.Join(inputDir, "b.obj")
	obj := []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	require.NoError(t, os.WriteFile(first, obj, 0o644))
	require.NoError(t, os.WriteFile(second, obj, 0o644))

	app := &cli.App{
		Commands: []*cli.Command{infoCmd()},
	}

	stdout, err := captureStdout(t, func() error {
		return app.Run([]string{"app", "info", "--json", first, second})
	})
	require.NoError(t, err)

	var outputs []assetInfo
	require.NoError(t, json.Unmarshal([]byte(stdout), &outputs))
	require.Len(t, outputs, 2)
	require.Equal(t, "a.obj", outputs[0].File)
	require.Equal(t, "b.obj", outputs[1].File)
}

func TestSingleInputCommandsRejectExtraArgs(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "tri.obj")
	require.NoError(t, os.WriteFile(inputPath, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644))

	tests := []struct {
		name string
		app  *cli.App
		args []string
	}{
		{
			name: "batch",
			app:  &cli.App{Commands: []*cli.Command{batchCmd()}},
			args: []string{"app", "batch", dir, dir},
		},
		{
			name: "inspect",
			app:  &cli.App{Commands: []*cli.Command{inspectCmd()}},
			args: []string{"app", "inspect", inputPath, inputPath},
		},
		{
			name: "cook",
			app:  &cli.App{Commands: []*cli.Command{cookCmd()}},
			args: []string{"app", "cook", inputPath, inputPath},
		},
		{
			name: "export",
			app:  &cli.App{Commands: []*cli.Command{exportCmd()}},
			args: []string{"app", "export", "--format", "json", "--out", filepath.Join(dir, "out.json"), inputPath, inputPath},
		},
		{
			name: "convert",
			app:  &cli.App{Commands: []*cli.Command{convertCmd()}},
			args: []string{"app", "convert", "--out", filepath.Join(dir, "out.json"), inputPath, inputPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.app.Run(tt.args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "expected exactly 1 argument")
		})
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	defer func() {
		os.Stdout = oldStdout
	}()

	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()

	runErr := fn()
	require.NoError(t, writer.Close())
	output := <-done
	require.NoError(t, reader.Close())
	return output, runErr
}
