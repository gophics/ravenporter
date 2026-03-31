// Package jsonir implements a JSON IR emitter for debugging and interchange.
package jsonir

import (
	"encoding/json"
	"io"

	"github.com/gophics/ravenporter/emit"
	"github.com/gophics/ravenporter/ir"
)

const (
	defaultBaseName = "scene"
	extJSON         = ".json"
	indentStr       = "  "
)

// Emitter writes the full IR asset as JSON.
type Emitter struct{}

func (e *Emitter) Emit(asset *ir.Asset, out emit.OutputFS, opts emit.Options) error {
	name := opts.BaseName
	if name == "" {
		name = defaultBaseName
	}

	w, err := out.Create(name + extJSON)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	if opts.PrettyPrint {
		enc.SetIndent("", indentStr)
	}
	encErr := enc.Encode(asset)
	closeErr := w.Close()
	if encErr != nil {
		return encErr
	}
	return closeErr
}

// WriteTo is a convenience function that writes JSON to any writer.
func WriteTo(asset *ir.Asset, w io.Writer, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", indentStr)
	}
	return enc.Encode(asset)
}
