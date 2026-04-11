package binread_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gophics/ravenporter/internal/binread"
)

func TestReadIntegers(t *testing.T) {
	buf := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	assert.Equal(t, uint8(0x01), binread.ReadU8(buf))
	assert.Equal(t, uint16(0x0201), binread.ReadU16LE(buf))
	assert.Equal(t, uint16(0x0102), binread.ReadU16BE(buf))
	assert.Equal(t, uint32(0x04030201), binread.ReadU32LE(buf))
	assert.Equal(t, uint32(0x01020304), binread.ReadU32BE(buf))
	assert.Equal(t, uint64(0x0807060504030201), binread.ReadU64LE(buf))
	assert.Equal(t, int16(0x0201), binread.ReadI16LE(buf))
	assert.Equal(t, int16(0x0102), binread.ReadI16BE(buf))
	assert.Equal(t, int32(0x04030201), binread.ReadI32LE(buf))
}

func TestReadFloat32(t *testing.T) {
	val := float32(3.14)
	bits := math.Float32bits(val)
	buf := []byte{byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24)}
	assert.Equal(t, val, binread.ReadF32LE(buf))
	assert.Equal(t, val, binread.ReadF32BE([]byte{byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits)}))
}

func TestReadFloat64(t *testing.T) {
	val := 6.28
	bits := math.Float64bits(val)
	le := []byte{byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24), byte(bits >> 32), byte(bits >> 40), byte(bits >> 48), byte(bits >> 56)}
	be := []byte{byte(bits >> 56), byte(bits >> 48), byte(bits >> 40), byte(bits >> 32), byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits)}
	assert.Equal(t, val, binread.ReadF64LE(le))
	assert.Equal(t, val, binread.ReadF64BE(be))
}

func TestReadString(t *testing.T) {
	buf := []byte{'H', 'e', 'l', 'l', 'o', 0, 'X'}
	assert.Equal(t, "Hello", binread.ReadString(buf, 10))
}

func TestReadFixedString(t *testing.T) {
	buf := []byte{'A', 'B', 0, 0, 0}
	assert.Equal(t, "AB", binread.ReadFixedString(buf, 5))
}

func TestCStringHelpers(t *testing.T) {
	buf := []byte{'O', 'K', 0, 'X'}
	assert.Equal(t, "OK", binread.CString(buf))
	assert.Equal(t, 3, binread.CStringLen(buf))
	assert.Equal(t, 2, binread.ClampChunkSize(2, 10))
	assert.Equal(t, 1, binread.ClampChunkSize(4, 1))
}

func TestBufferPool(t *testing.T) {
	pool := binread.NewBufferPool(1024)
	b := pool.Get()
	assert.Len(t, *b, 1024)
	pool.Put(b)

	b2 := pool.Get()
	assert.Len(t, *b2, 1024)

	bad := []byte{1}
	pool.Put(&bad)
	assert.NotNil(t, binread.DefaultBufferPool.Get())
}

func TestWriteAndAlignmentHelpers(t *testing.T) {
	buf := make([]byte, 12)
	binread.PutU32LE(buf[:4], 0x04030201)
	binread.PutU64LE(buf[4:], 0x0C0B0A0908070605)

	assert.Equal(t, uint32(0x04030201), binread.ReadU32LE(buf[:4]))
	assert.Equal(t, uint64(0x0C0B0A0908070605), binread.ReadU64LE(buf[4:]))

	aligned := binread.AppendAligned([]byte{1, 2, 3}, 8)
	assert.Len(t, aligned, 8)
	assert.Equal(t, []byte{1, 2, 3, 0, 0, 0, 0, 0}, aligned)
}
