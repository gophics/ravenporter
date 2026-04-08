package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/cache"
)

const cacheOBJ = "o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

func run(w io.Writer) (err error) {
	result, err := ravenporter.ImportBytes(context.Background(), []byte(cacheOBJ), "scene.obj")
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := cache.Write(&buf, result); err != nil {
		return err
	}

	pkg, err := cache.Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkg.Close())
	}()

	_, err = fmt.Fprintf(
		w,
		"source=%s cooked_meshes=%d summary_meshes=%d\n",
		pkg.Manifest.SourceFormat,
		len(pkg.Meshes),
		pkg.Manifest.Summary.Meshes,
	)
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
