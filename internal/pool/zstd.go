package pool

import (
	"sync"

	"github.com/klauspost/compress/zstd"
)

var zstdDecoderPool sync.Pool

func newZstdDecoder() (*zstd.Decoder, error) {
	return zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))
}

func ZstdDecodeAll(src []byte) ([]byte, error) {
	dec, _ := zstdDecoderPool.Get().(*zstd.Decoder) //nolint:errcheck // pool only stores *zstd.Decoder
	if dec == nil {
		var err error
		dec, err = newZstdDecoder()
		if err != nil {
			return nil, err
		}
	}
	defer zstdDecoderPool.Put(dec)
	return dec.DecodeAll(src, nil)
}
