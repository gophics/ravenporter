// Package piz implements PIZ decompression for OpenEXR images.
package piz

import (
	"errors"

	"github.com/gophics/ravenporter/internal/binread"
)

const (
	haarMinSamples = 2
	bitsPerByte    = 8
	halfSize       = 2
	maxBitmapSize  = 1 << 16
	minHeaderSize  = 4
	u64Size        = 8
	u16Bits        = 16
	u32Shift       = 32
	bitsRound      = 7
)

var (
	errShortData = errors.New("piz: data too short")
	errBadBitmap = errors.New("piz: bitmap exceeds max size")
)

func Decompress(src []byte, channels, w, h int) ([]byte, error) {
	if len(src) < minHeaderSize {
		return nil, errShortData
	}

	pos := 0
	if pos+minHeaderSize > len(src) {
		return nil, errShortData
	}
	minNZ := int(binread.ReadU16LE(src[pos:]))
	maxNZ := int(binread.ReadU16LE(src[pos+halfSize:]))
	pos += minHeaderSize

	if minNZ > maxNZ || maxNZ >= maxBitmapSize {
		return nil, errBadBitmap
	}

	bitmap := make([]byte, maxBitmapSize)
	pos = decodeBitmap(src, pos, bitmap, minNZ, maxNZ)

	fwd, rev, _ := buildLookupTables(bitmap)

	if pos+minHeaderSize > len(src) {
		return nil, errShortData
	}
	nBits := int(binread.ReadU32LE(src[pos:]))
	pos += minHeaderSize

	huffFreq := make([]uint64, maxBitmapSize)
	pos = decodeHuffFreqs(src, pos, huffFreq, minNZ, maxNZ)

	totalSamples := channels * w * h
	decoded := make([]uint16, totalSamples)
	if nBits > 0 {
		decodeHuffData(src[pos:], nBits, decoded, fwd)
	}

	stride := w * h
	for ch := range channels {
		haarInverse(decoded[ch*stride : (ch+1)*stride])
	}

	out := make([]byte, totalSamples*halfSize)
	for i, v := range decoded {
		val := rev[v]
		out[i*halfSize] = byte(val)
		out[i*halfSize+1] = byte(val >> bitsPerByte)
	}

	return out, nil
}

func decodeBitmap(src []byte, pos int, bitmap []byte, minNZ, maxNZ int) int {
	for i := minNZ; i <= maxNZ && pos < len(src); {
		count := int(src[pos])
		pos++
		if count > 0 {
			i += count
			continue
		}
		if pos >= len(src) {
			break
		}
		count = int(src[pos])
		pos++
		for j := range count {
			if i+j < len(bitmap) {
				bitmap[i+j] = 1
			}
		}
		i += count
	}
	return pos
}

func buildLookupTables(bitmap []byte) (fwd, rev []uint16, size int) {
	fwd = make([]uint16, maxBitmapSize)
	rev = make([]uint16, maxBitmapSize)
	k := 0
	for i := range maxBitmapSize {
		if bitmap[i] != 0 {
			fwd[i] = uint16(k)
			rev[k] = uint16(i)
			k++
		}
	}
	return fwd, rev, k
}

func decodeHuffFreqs(src []byte, pos int, freq []uint64, minNZ, maxNZ int) int {
	for i := minNZ; i <= maxNZ && pos+u64Size <= len(src); i++ {
		lo := uint64(binread.ReadU32LE(src[pos:]))
		hi := uint64(binread.ReadU32LE(src[pos+4:]))
		freq[i] = lo | hi<<u32Shift
		pos += u64Size
	}
	return pos
}

func decodeHuffData(src []byte, nBits int, out, fwd []uint16) {
	nBytes := (nBits + bitsRound) / bitsPerByte
	if nBytes >= len(out)*halfSize {
		for i := range min(len(out), len(src)/halfSize) {
			out[i] = binread.ReadU16LE(src[i*halfSize:])
		}
		return
	}

	bitPos := 0
	for i := range out {
		if bitPos+u16Bits > nBits {
			break
		}
		byteIdx := bitPos / bitsPerByte
		bitOff := uint(bitPos % bitsPerByte)
		if byteIdx+halfSize <= len(src) {
			raw := uint32(src[byteIdx]) | uint32(src[byteIdx+1])<<bitsPerByte
			if byteIdx+halfSize < len(src) {
				raw |= uint32(src[byteIdx+2]) << u16Bits
			}
			out[i] = fwd[uint16(raw>>bitOff)] //nolint:gosec // truncation intended
		}
		bitPos += u16Bits
	}
}

func haarInverse(data []uint16) {
	n := len(data)
	if n < haarMinSamples {
		return
	}

	p := 1
	for p*haarMinSamples <= n {
		p *= haarMinSamples
	}

	for s := haarMinSamples; s <= p; s *= haarMinSamples {
		half := s / haarMinSamples
		for i := 0; i+half < n; i += s {
			a := data[i]
			b := data[i+half]
			data[i] = a + b
			data[i+half] = a - b
		}
	}
}
