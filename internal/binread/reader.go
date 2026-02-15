package binread

import (
	"encoding/binary"
	"math"
)

func ReadU8(b []byte) uint8      { return b[0] }
func ReadU16LE(b []byte) uint16  { return binary.LittleEndian.Uint16(b) }
func ReadU16BE(b []byte) uint16  { return binary.BigEndian.Uint16(b) }
func ReadU32LE(b []byte) uint32  { return binary.LittleEndian.Uint32(b) }
func ReadU32BE(b []byte) uint32  { return binary.BigEndian.Uint32(b) }
func ReadU64LE(b []byte) uint64  { return binary.LittleEndian.Uint64(b) }
func ReadI16LE(b []byte) int16   { return int16(binary.LittleEndian.Uint16(b)) } //nolint:gosec // intentional
func ReadI16BE(b []byte) int16   { return int16(binary.BigEndian.Uint16(b)) }    //nolint:gosec // intentional
func ReadI32LE(b []byte) int32   { return int32(binary.LittleEndian.Uint32(b)) } //nolint:gosec // intentional
func ReadF32LE(b []byte) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b)) }
func ReadF32BE(b []byte) float32 { return math.Float32frombits(binary.BigEndian.Uint32(b)) }
func ReadF64LE(b []byte) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b)) }
func ReadF64BE(b []byte) float64 { return math.Float64frombits(binary.BigEndian.Uint64(b)) }

func ReadString(b []byte, maxLen int) string {
	if maxLen > len(b) {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		if b[i] == 0 {
			return string(b[:i])
		}
	}
	return string(b[:maxLen])
}

func ReadFixedString(b []byte, n int) string {
	if n > len(b) {
		n = len(b)
	}
	end := n
	for end > 0 && b[end-1] == 0 {
		end--
	}
	return string(b[:end])
}

// CString reads a null-terminated string from b.
func CString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// CStringLen returns the length of a null-terminated string including the null byte.
func CStringLen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i + 1
		}
	}
	return len(b)
}

// ClampChunkSize clamps a chunk size to the available data length.
func ClampChunkSize(dataLen int, size uint32) int {
	n := int(size)
	if n > dataLen {
		return dataLen
	}
	return n
}
