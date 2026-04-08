package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gophics/ravenporter"
)

const quickstartOBJ = "o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

func run(w io.Writer) (err error) {
	dir, err := os.MkdirTemp("", "ravenporter-quickstart-*")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(dir))
	}()

	scenePath := filepath.Join(dir, "scene.obj")
	if err := os.WriteFile(scenePath, []byte(quickstartOBJ), 0o600); err != nil {
		return err
	}

	result, err := ravenporter.ImportPath(
		context.Background(),
		scenePath,
		ravenporter.WithPreset(ravenporter.BuiltInPresetFast),
		ravenporter.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		w,
		"format=%s preset=%s meshes=%d issues=%d\n",
		result.Report.Source.DetectedFormat,
		result.Report.Source.Preset,
		len(result.Asset.Meshes),
		len(result.Report.Issues),
	)
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
