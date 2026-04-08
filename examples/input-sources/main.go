package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gophics/ravenporter"
)

const inputSourcesOBJ = "o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

func run(w io.Writer) (err error) {
	ctx := context.Background()
	data := []byte(inputSourcesOBJ)

	dir, err := os.MkdirTemp("", "ravenporter-input-sources-*")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(dir))
	}()

	scenePath := filepath.Join(dir, "scene.obj")
	if err := os.WriteFile(scenePath, data, 0o600); err != nil {
		return err
	}

	fromDirFS, err := ravenporter.ImportFS(ctx, os.DirFS(dir), "scene.obj")
	if err != nil {
		return err
	}

	fromBytes, err := ravenporter.ImportBytes(ctx, data, "scene.obj")
	if err != nil {
		return err
	}

	fromReader, err := ravenporter.ImportReader(ctx, bytes.NewReader(data), "scene.obj")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		w,
		"dirfs=%s/%d\nbytes=%s/%d\nreader=%s/%d\n",
		fromDirFS.Report.Source.DetectedFormat,
		len(fromDirFS.Asset.Meshes),
		fromBytes.Report.Source.DetectedFormat,
		len(fromBytes.Asset.Meshes),
		fromReader.Report.Source.DetectedFormat,
		len(fromReader.Asset.Meshes),
	)
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
