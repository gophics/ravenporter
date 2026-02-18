package piz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecompressErrors(t *testing.T) {
	_, err := Decompress([]byte{1, 2, 3}, 1, 1, 1)
	require.ErrorIs(t, err, errShortData)

	_, err = Decompress([]byte{1, 0, 0, 0}, 1, 1, 1)
	require.ErrorIs(t, err, errBadBitmap)
}

func TestDecompressMinimal(t *testing.T) {
	src := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x01,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	out, err := Decompress(src, 1, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0}, out)
}

func TestDecodeBitmapAndLookupTables(t *testing.T) {
	bitmap := make([]byte, maxBitmapSize)
	pos := decodeBitmap([]byte{0, 2, 1, 0, 1}, 0, bitmap, 0, 4)
	assert.Equal(t, 5, pos)
	assert.Equal(t, byte(1), bitmap[0])
	assert.Equal(t, byte(1), bitmap[1])
	assert.Equal(t, byte(1), bitmap[3])

	fwd, rev, size := buildLookupTables(bitmap)
	assert.Equal(t, 3, size)
	assert.Equal(t, uint16(0), fwd[0])
	assert.Equal(t, uint16(1), fwd[1])
	assert.Equal(t, uint16(3), rev[2])
}

func TestDecodeHuffFreqsAndData(t *testing.T) {
	freq := make([]uint64, maxBitmapSize)
	pos := decodeHuffFreqs([]byte{1, 0, 0, 0, 0, 0, 0, 0}, 0, freq, 0, 0)
	assert.Equal(t, 8, pos)
	assert.Equal(t, uint64(1), freq[0])

	out := make([]uint16, 2)
	decodeHuffData([]byte{1, 0, 2, 0}, 40, out, nil)
	assert.Equal(t, []uint16{1, 2}, out)

	mapped := make([]uint16, maxBitmapSize)
	mapped[1] = 7
	out = make([]uint16, 2)
	decodeHuffData([]byte{1, 0, 0}, 16, out, mapped)
	assert.Equal(t, uint16(7), out[0])
}

func TestHaarInverse(t *testing.T) {
	data := []uint16{4, 2}
	haarInverse(data)
	assert.Equal(t, []uint16{6, 2}, data)

	data = []uint16{1}
	haarInverse(data)
	assert.Equal(t, []uint16{1}, data)
}
