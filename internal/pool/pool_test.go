package pool_test

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/pool"
)

func TestByteBufferGetPut(t *testing.T) {
	p := pool.NewByteBuffer(1024)
	buf := p.Get()
	require.NotNil(t, buf)
	assert.Len(t, *buf, 1024)

	// Shrink and put back â€” should reset to full cap.
	*buf = (*buf)[:10]
	p.Put(buf)

	buf2 := p.Get()
	assert.Equal(t, 1024, len(*buf2))
	p.Put(buf2)
}

func TestByteBufferDefaultSize(t *testing.T) {
	p := pool.NewByteBuffer(0)
	buf := p.Get()
	assert.Equal(t, 4096, len(*buf))
	p.Put(buf)
}

func TestFloat32BufferGetPut(t *testing.T) {
	p := pool.NewFloat32Buffer(256)
	buf := p.Get()
	require.NotNil(t, buf)
	assert.Len(t, *buf, 256)

	*buf = (*buf)[:5]
	p.Put(buf)

	buf2 := p.Get()
	assert.Equal(t, 256, len(*buf2))
	p.Put(buf2)
}

func TestFloat32BufferDefaultSize(t *testing.T) {
	p := pool.NewFloat32Buffer(-1)
	buf := p.Get()
	assert.Equal(t, 4096, len(*buf))
	p.Put(buf)
}

func BenchmarkByteBufferGetPut(b *testing.B) {
	p := pool.NewByteBuffer(4096)
	b.ReportAllocs()
	for b.Loop() {
		buf := p.Get()
		p.Put(buf)
	}
}

func TestLZ4DecodeBlock(t *testing.T) {
	original := []byte("The quick brown fox jumps over the lazy dog.")
	var compressed [256]byte
	n, err := lz4.CompressBlock(original, compressed[:], nil)
	require.NoError(t, err)
	require.Greater(t, n, 0)

	decoded, err := pool.LZ4DecodeBlock(compressed[:n], len(original))
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestLZ4DecodeBlockPoolReuse(t *testing.T) {
	original := []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	var compressed [256]byte
	n, _ := lz4.CompressBlock(original, compressed[:], nil)

	for range 10 {
		decoded, err := pool.LZ4DecodeBlock(compressed[:n], len(original))
		require.NoError(t, err)
		assert.Equal(t, original, decoded)
	}
}

func BenchmarkLZ4DecodeBlock(b *testing.B) {
	original := make([]byte, 4096)
	for i := range original {
		original[i] = byte(i % 256)
	}
	compressed := make([]byte, 8192)
	n, _ := lz4.CompressBlock(original, compressed, nil)
	compressed = compressed[:n]

	b.ReportAllocs()
	b.SetBytes(int64(len(original)))
	for b.Loop() {
		_, _ = pool.LZ4DecodeBlock(compressed, len(original))
	}
}

func TestArena(t *testing.T) {
	arena := pool.NewArena[int](1)
	first := arena.Alloc(1)
	second := arena.Alloc(2)
	assert.Len(t, first, 1)
	assert.Len(t, second, 2)
}

func TestZlibReader(t *testing.T) {
	original := []byte("hello zlib")
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, err := writer.Write(original)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	reader := &pool.ZlibReader{}
	decoded, err := reader.Decompress(compressed.Bytes())
	require.NoError(t, err)
	assert.Equal(t, original, decoded)

	dst := make([]byte, len(original))
	require.NoError(t, reader.DecompressInto(dst, compressed.Bytes()))
	assert.Equal(t, original, dst)
	assert.Error(t, reader.DecompressInto(make([]byte, len(original)+1), compressed.Bytes()))
}

func TestGlobalBytePool(t *testing.T) {
	buf := pool.GetBuffer(8)
	assert.Len(t, buf, 8)
	pool.PutBuffer(buf)
	assert.Len(t, pool.GetBuffer(4), 4)
}

func TestBrotliDecodeSized(t *testing.T) {
	original := []byte("hello brotli")
	var compressed bytes.Buffer
	writer := brotli.NewWriter(&compressed)
	_, err := writer.Write(original)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	decoded, err := pool.BrotliDecodeSized(compressed.Bytes(), len(original))
	require.NoError(t, err)
	assert.Equal(t, original, decoded)

	decoded, err = pool.BrotliDecodeSized(compressed.Bytes(), 0)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestZstdDecodeAll(t *testing.T) {
	original := []byte("hello zstd")
	encoder, err := zstd.NewWriter(io.Discard)
	require.NoError(t, err)
	compressed := encoder.EncodeAll(original, nil)
	require.NoError(t, encoder.Close())

	decoded, err := pool.ZstdDecodeAll(compressed)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}
