package assetio

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/ir"
)

// ReadResult holds the output of a resolved and read asset file.
type ReadResult struct {
	Data     []byte
	FormatID ir.FormatID
}

// ResolveAndRead resolves a potentially relative path against baseDir,
// reads the file contents, and auto-detects its format using the library's
// own detect pipeline. Returns nil if the path cannot be resolved (e.g.
// relative path with empty baseDir).
func ResolveAndRead(path, baseDir string) (*ReadResult, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		if baseDir == "" {
			return nil, nil
		}
		resolved = filepath.Join(baseDir, resolved)
	}

	data, err := os.ReadFile(resolved) //nolint:gosec // path validated via caller-provided baseDir
	if err != nil {
		return nil, err
	}

	formatID, _ := decode.DefaultRegistry().Detect(bytes.NewReader(data), resolved) //nolint:errcheck // best-effort detection
	return &ReadResult{Data: data, FormatID: formatID}, nil
}

func ReadFromFS(path string, fsys detect.SeekableFS) (*ReadResult, error) {
	if fsys == nil || path == "" {
		return nil, nil
	}

	rc, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(rc)
	closeErr := rc.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return nil, closeErr
	}

	formatID, _ := decode.DefaultRegistry().Detect(bytes.NewReader(data), path) //nolint:errcheck // best-effort detection
	return &ReadResult{Data: data, FormatID: formatID}, nil
}
