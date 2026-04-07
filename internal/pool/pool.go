// Package pool provides typed sync.Pool wrappers for reusable buffers.
package pool

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"sync"
)

const defaultBufSize = 4096

// SlicePool is a sync.Pool-backed reusable typed slice.
type SlicePool[T any] struct {
	pool sync.Pool
}

// NewSlicePool creates a pool that allocates slices of the given default size.
func NewSlicePool[T any](size int) *SlicePool[T] {
	if size <= 0 {
		size = defaultBufSize
	}
	return &SlicePool[T]{
		pool: sync.Pool{
			New: func() any { s := make([]T, size); return &s },
		},
	}
}

// Get retrieves a slice from the pool.
func (p *SlicePool[T]) Get() *[]T {
	buf, _ := p.pool.Get().(*[]T) //nolint:errcheck // pool only stores *[]T
	return buf
}

// Put returns a slice to the pool, resetting its length to the full capacity.
func (p *SlicePool[T]) Put(buf *[]T) {
	*buf = (*buf)[:cap(*buf)]
	p.pool.Put(buf)
}

// ByteBuffer is a SlicePool specialized for byte slices.
type ByteBuffer = SlicePool[byte]

// Float32Buffer is a SlicePool specialized for float32 slices.
type Float32Buffer = SlicePool[float32]

// NewByteBuffer creates a pool that allocates byte slices of the given default size.
func NewByteBuffer(size int) *ByteBuffer { return NewSlicePool[byte](size) }

// NewFloat32Buffer creates a pool that allocates float32 slices of the given default size.
func NewFloat32Buffer(size int) *Float32Buffer { return NewSlicePool[float32](size) }

type zlibResetter interface {
	io.ReadCloser
	Reset(r io.Reader, dict []byte) error
}

func newZlibReader(src []byte) (zlibResetter, error) {
	r, err := zlib.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	zr, ok := r.(zlibResetter)
	if !ok {
		_ = r.Close() //nolint:errcheck // discarding close error; returning type assertion error
		return nil, errors.New("pool: zlib reader does not implement Reset")
	}
	return zr, nil
}

type ZlibReader struct {
	pool sync.Pool
}

func (p *ZlibReader) Decompress(src []byte) ([]byte, error) {
	zr, _ := p.pool.Get().(zlibResetter) //nolint:errcheck // pool type
	if zr == nil {
		var err error
		zr, err = newZlibReader(src)
		if err != nil {
			return nil, err
		}
	} else {
		if err := zr.Reset(bytes.NewReader(src), nil); err != nil {
			p.pool.Put(zr)
			return nil, err
		}
	}

	raw, readErr := io.ReadAll(zr)
	closeErr := zr.Close()
	p.pool.Put(zr)

	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return raw, nil
}

func (p *ZlibReader) DecompressInto(dst, src []byte) error {
	zr, _ := p.pool.Get().(zlibResetter) //nolint:errcheck // pool type
	if zr == nil {
		var err error
		zr, err = newZlibReader(src)
		if err != nil {
			return err
		}
	} else {
		if err := zr.Reset(bytes.NewReader(src), nil); err != nil {
			p.pool.Put(zr)
			return err
		}
	}

	_, readErr := io.ReadFull(zr, dst)
	closeErr := zr.Close()
	p.pool.Put(zr)

	if readErr != nil {
		return readErr
	}
	return closeErr
}

var bytePool sync.Pool

// GetBuffer retrieves a byte slice with at least the requested capacity.
func GetBuffer(size int) []byte {
	v := bytePool.Get()
	if v == nil {
		return make([]byte, size)
	}
	if bPtr, ok := v.(*[]byte); ok {
		buf := *bPtr
		if cap(buf) < size {
			return make([]byte, size)
		}
		return buf[:size]
	}
	return make([]byte, size)
}

// PutBuffer returns a byte slice to the global pool.
func PutBuffer(buf []byte) {
	bytePool.Put(&buf)
}
