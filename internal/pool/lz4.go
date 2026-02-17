package pool

import (
	"sync"

	"github.com/pierrec/lz4/v4"
)

var lz4BufPool = sync.Pool{}

func LZ4DecodeBlock(src []byte, dstSize int) ([]byte, error) {
	var dst []byte
	if v := lz4BufPool.Get(); v != nil {
		buf := v.(*[]byte) //nolint:errcheck,forcetypeassert
		if cap(*buf) >= dstSize {
			dst = (*buf)[:dstSize]
		} else {
			dst = make([]byte, dstSize)
		}
	} else {
		dst = make([]byte, dstSize)
	}

	n, err := lz4.UncompressBlock(src, dst)
	if err != nil {
		return nil, err
	}

	out := make([]byte, n)
	copy(out, dst[:n])
	lz4BufPool.Put(&dst)
	return out, nil
}
