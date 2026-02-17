package pool

import (
	"io"
	"sync"

	"github.com/andybalholm/brotli"
)

var brotliReaderPool = sync.Pool{
	New: func() any {
		return brotli.NewReader(nil)
	},
}

func BrotliDecodeSized(src []byte, sizeHint int) ([]byte, error) {
	br := brotliReaderPool.Get().(*brotli.Reader) //nolint:errcheck,forcetypeassert
	_ = br.Reset(newBytesReader(src))             //nolint:errcheck // reset never fails

	var out []byte
	var err error
	if sizeHint > 0 {
		out = make([]byte, sizeHint)
		n, readErr := io.ReadFull(br, out)
		out = out[:n]
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			err = readErr
		}
	} else {
		out, err = io.ReadAll(br)
	}

	brotliReaderPool.Put(br)
	return out, err
}

type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader { return &bytesReader{data: data} }

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
