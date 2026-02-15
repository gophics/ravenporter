package binread

import "sync"

const defaultBufSize = 64 * 1024

// BufferPool is a sync.Pool wrapper for reusable byte buffers.
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool creates a pool that vends buffers of the given size.
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				b := make([]byte, size)
				return &b
			},
		},
		size: size,
	}
}

// Get returns a buffer from the pool.
func (p *BufferPool) Get() *[]byte {
	v := p.pool.Get()
	buf, ok := v.(*[]byte)
	if !ok {
		b := make([]byte, p.size)
		return &b
	}
	return buf
}

// Put returns a buffer to the pool.
func (p *BufferPool) Put(b *[]byte) {
	if b == nil || cap(*b) < p.size {
		return
	}
	*b = (*b)[:p.size]
	p.pool.Put(b)
}

// DefaultBufferPool is a pre-configured 64KB buffer pool.
var DefaultBufferPool = NewBufferPool(defaultBufSize)
