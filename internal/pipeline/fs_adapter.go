package pipeline

import (
	"io"
	"io/fs"

	"github.com/gophics/ravenporter/detect"
)

// fsAdapter wraps an fs.FS to implement detect.SeekableFS.
type fsAdapter struct {
	fsys fs.FS
}

func (a fsAdapter) Open(name string) (io.ReadCloser, error) {
	return a.fsys.Open(name)
}

// ensureFS wraps an fs.FS into a detect.SeekableFS if it isn't already one.
func ensureFS(fsys fs.FS) detect.SeekableFS {
	if sfs, ok := any(fsys).(detect.SeekableFS); ok {
		return sfs
	}
	return fsAdapter{fsys: fsys}
}
