// Package emit defines the output emitter interface.
package emit

import (
	"io"
	"log/slog"

	"github.com/gophics/ravenporter/ir"
)

// Emitter converts an IR asset to an engine-specific representation.
type Emitter interface {
	Emit(asset *ir.Asset, out OutputFS, opts Options) error
}

// OutputFS abstracts multi-file output for emitters.
type OutputFS interface {
	Create(path string) (io.WriteCloser, error)
}

// Options configures emitter behavior.
type Options struct {
	BaseName      string
	Logger        *slog.Logger
	EmbedTextures bool
	PrettyPrint   bool
}
