// Package pool provides typed sync.Pool wrappers for reusable buffers.
package pool

import (
	"bytes"
	"compress/zlib"
	"io"
	"sync"
)

const defaultBufSize = 4096

// ByteBuffer is a sync.Pool-backed reusable byte slice.
type ByteBuffer struct {
	pool sync.Pool
}

// NewByteBuffer creates a pool that allocates byte slices of the given default size.
func NewByteBuffer(size int) *ByteBuffer {
	if size <= 0 {
		size = defaultBufSize
	}
	return &ByteBuffer{
		pool: sync.Pool{
			New: func() any { s := make([]byte, size); return &s },
		},
	}
}

// Get retrieves a byte slice from the pool.
func (p *ByteBuffer) Get() *[]byte {
	buf, _ := p.pool.Get().(*[]byte) //nolint:errcheck // pool only stores *[]byte
	return buf
}

// Put returns a byte slice to the pool, resetting its length to the full capacity.
func (p *ByteBuffer) Put(buf *[]byte) {
	*buf = (*buf)[:cap(*buf)]
	p.pool.Put(buf)
}

// Float32Buffer is a sync.Pool-backed reusable float32 slice.
type Float32Buffer struct {
	pool sync.Pool
}

// NewFloat32Buffer creates a pool that allocates float32 slices of the given default size.
func NewFloat32Buffer(size int) *Float32Buffer {
	if size <= 0 {
		size = defaultBufSize
	}
	return &Float32Buffer{
		pool: sync.Pool{
			New: func() any { s := make([]float32, size); return &s },
		},
	}
}

// Get retrieves a float32 slice from the pool.
func (p *Float32Buffer) Get() *[]float32 {
	buf, _ := p.pool.Get().(*[]float32) //nolint:errcheck // pool only stores *[]float32
	return buf
}

// Put returns a float32 slice to the pool, resetting its length to the full capacity.
func (p *Float32Buffer) Put(buf *[]float32) {
	*buf = (*buf)[:cap(*buf)]
	p.pool.Put(buf)
}

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
		return nil, err
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
