package exr

import (
	"encoding/binary"

	"github.com/gophics/ravenporter/internal/binread"
)

const (
	b44BlockW    = 4
	b44BlockH    = 4
	b44BlockPx   = b44BlockW * b44BlockH
	b44Packed    = 14
	b44Flat      = 3
	b44Half      = 2
	b44MaxShift  = 32
	b44DiffBits  = 6
	b44DiffMask  = (1 << b44DiffBits) - 1
	b44DiffHalf  = 1 << (b44DiffBits - 1)
	b44DiffRange = 1 << b44DiffBits
	b44FlatShift = 63
	b44DiffCount = 15
	b44BitBytes  = 10
	b44BitShift  = 8
	b44ShiftDrop = 2
)

func b44Decompress(dst, src []byte, nChan, w, h int) {
	bx := (w + b44BlockW - 1) / b44BlockW
	by := (h + b44BlockH - 1) / b44BlockH
	pos := 0

	for ch := range nChan {
		chBase := ch * w * h * b44Half
		for ty := range by {
			for tx := range bx {
				block, advance := readB44Block(src, pos)
				if advance == 0 {
					return
				}
				pos += advance
				writeBlock(dst, block, chBase, tx*b44BlockW, ty*b44BlockH, w, h)
			}
		}
	}
}

func readB44Block(src []byte, pos int) (block [b44BlockPx]uint16, advance int) {
	remaining := len(src) - pos
	if remaining < b44Flat {
		return block, 0
	}

	if remaining >= b44Packed && int(src[pos+2])>>b44ShiftDrop != b44FlatShift {
		return unpackB44Block(src[pos:]), b44Packed
	}

	v := binread.ReadU16LE(src[pos:])
	for i := range b44BlockPx {
		block[i] = v //nolint:gosec
	}
	return block, b44Flat
}

func writeBlock(dst []byte, block [b44BlockPx]uint16, chBase, ox, oy, w, h int) {
	curW := min(b44BlockW, w-ox)
	curH := min(b44BlockH, h-oy)

	for py := range curH {
		for px := range curW {
			off := chBase + ((oy+py)*w+ox+px)*b44Half
			if off+1 >= len(dst) {
				return
			}
			binary.LittleEndian.PutUint16(dst[off:], block[py*b44BlockW+px])
		}
	}
}

func unpackB44Block(src []byte) [b44BlockPx]uint16 {
	var t [b44BlockPx]uint16

	t[0] = binread.ReadU16LE(src)
	shift := min(int(src[2])>>b44ShiftDrop, b44MaxShift)

	bits := uint64(0)
	for i := range b44BitBytes {
		if 3+i < len(src) {
			bits |= uint64(src[3+i]) << (i * b44BitShift)
		}
	}

	var r [b44DiffCount]int
	for i := range b44DiffCount {
		raw := int(bits & b44DiffMask) //nolint:gosec // mask guarantees small value
		if raw >= b44DiffHalf {
			raw -= b44DiffRange
		}
		r[i] = raw << shift
		bits >>= b44DiffBits
	}

	// Differential topology (OpenEXR spec):
	//  0 -->  1 -->  2 -->  3
	//  |      3      7     11
	//  | 0
	//  v
	//  4 -->  5 -->  6 -->  7
	//  |      4      8     12
	//  | 1
	//  v
	//  8 -->  9 --> 10 --> 11
	//  |      5      9     13
	//  | 2
	//  v
	// 12 --> 13 --> 14 --> 15
	//         6     10     14

	t[4] = halfAdd(t[0], r[0])  //nolint:gosec
	t[8] = halfAdd(t[4], r[1])  //nolint:gosec
	t[12] = halfAdd(t[8], r[2]) //nolint:gosec

	t[1] = halfAdd(t[0], r[3])   //nolint:gosec
	t[5] = halfAdd(t[4], r[4])   //nolint:gosec
	t[9] = halfAdd(t[8], r[5])   //nolint:gosec
	t[13] = halfAdd(t[12], r[6]) //nolint:gosec

	t[2] = halfAdd(t[1], r[7])    //nolint:gosec
	t[6] = halfAdd(t[5], r[8])    //nolint:gosec
	t[10] = halfAdd(t[9], r[9])   //nolint:gosec
	t[14] = halfAdd(t[13], r[10]) //nolint:gosec

	t[3] = halfAdd(t[2], r[11])   //nolint:gosec
	t[7] = halfAdd(t[6], r[12])   //nolint:gosec
	t[11] = halfAdd(t[10], r[13]) //nolint:gosec
	t[15] = halfAdd(t[14], r[14]) //nolint:gosec

	return t
}

func halfAdd(base uint16, diff int) uint16 {
	return uint16(max(0, min(0xFFFF, int(base)+diff))) //nolint:gosec,mnd // clamped
}
