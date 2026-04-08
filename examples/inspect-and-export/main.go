package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/gophics/ravenporter"
	jsonir "github.com/gophics/ravenporter/emit/json"
	"github.com/gophics/ravenporter/validate"
)

const inspectOBJ = "o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

type exportedAsset struct {
	Meshes    []json.RawMessage `json:"Meshes"`
	RootNodes []int             `json:"RootNodes"`
}

func run(w io.Writer) error {
	result, err := ravenporter.ImportBytes(context.Background(), []byte(inspectOBJ), "scene.obj")
	if err != nil {
		return err
	}

	validation := validate.Asset(result.Asset)

	var out bytes.Buffer
	if err := jsonir.WriteTo(result.Asset, &out, true); err != nil {
		return err
	}

	var exported exportedAsset
	if err := json.Unmarshal(out.Bytes(), &exported); err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		w,
		"format=%s report_issues=%d validation_errors=%d validation_warnings=%d json_meshes=%d json_root_nodes=%d\n",
		result.Report.Source.DetectedFormat,
		len(result.Report.Issues),
		len(validation.Errors),
		len(validation.Warnings),
		len(exported.Meshes),
		len(exported.RootNodes),
	)
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
