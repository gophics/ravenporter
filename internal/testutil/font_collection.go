package testutil

import "encoding/binary"

const (
	fontCollectionHeaderSize = 12
	fontCollectionOffsetSize = 4
	fontCollectionVersion    = 1 << 16
	sfntHeaderMinSize        = 12
	sfntTableEntrySize       = 16
	uint32Shift              = 8
)

func BuildFontCollection(fonts ...[]byte) []byte {
	headerSize := fontCollectionHeaderSize + len(fonts)*fontCollectionOffsetSize
	totalSize := headerSize
	for _, font := range fonts {
		totalSize += len(font)
	}

	data := make([]byte, totalSize)
	copy(data[:4], "ttcf")
	putUint32BEInt(data[4:8], fontCollectionVersion)
	putUint32BEInt(data[8:12], len(fonts))

	offset := headerSize
	for i, font := range fonts {
		entryOff := fontCollectionHeaderSize + i*fontCollectionOffsetSize
		putUint32BEInt(data[entryOff:], offset)
		copy(data[offset:], font)
		adjustCollectionOffsets(data[offset:offset+len(font)], offset)
		offset += len(font)
	}

	return data
}

func adjustCollectionOffsets(font []byte, base int) {
	if len(font) < sfntHeaderMinSize {
		return
	}
	numTables := int(binary.BigEndian.Uint16(font[4:6]))
	for i := range numTables {
		entryOff := sfntHeaderMinSize + i*sfntTableEntrySize
		if entryOff+sfntTableEntrySize > len(font) {
			break
		}
		tableOff := readUint32BEInt(font[entryOff+8 : entryOff+12])
		putUint32BEInt(font[entryOff+8:entryOff+12], tableOff+base)
	}
}

func putUint32BEInt(dst []byte, v int) {
	if v < 0 || uint64(v) > uint64(^uint32(0)) {
		panic("uint32 overflow")
	}
	binary.BigEndian.PutUint32(dst, uint32(v))
}

func readUint32BEInt(src []byte) int {
	v := 0
	for i := range 4 {
		v = (v << uint32Shift) | int(src[i])
	}
	return v
}
