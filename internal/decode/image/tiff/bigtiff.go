package tiff

import (
	"encoding/binary"
	"errors"
	"math"
)

const (
	bigTIFFHeaderSize = 16
	bigTIFFEntrySize  = 20

	classicTIFFHeaderSize     = 8
	classicTIFFEntrySize      = 12
	classicTIFFEntryCountSize = 2
	classicTIFFNextIFDSize    = 4
	classicTIFFInlineSize     = 4
	classicTIFFMagic          = 42
	bigTIFFEntryCountSize     = 8
	bigTIFFInlineSize         = 8

	tiffShortSize  = 2
	tiffLongSize   = 4
	tiffLong8Size  = 8
	tiffByteWidth  = uint64(1)
	tiffShortWidth = uint64(2)
	tiffLongWidth  = uint64(4)
	tiffLong8Width = uint64(8)

	tiffTypeByte      = 1
	tiffTypeASCII     = 2
	tiffTypeShort     = 3
	tiffTypeLong      = 4
	tiffTypeRational  = 5
	tiffTypeSByte     = 6
	tiffTypeUndefined = 7
	tiffTypeSShort    = 8
	tiffTypeSLong     = 9
	tiffTypeSRational = 10
	tiffTypeFloat     = 11
	tiffTypeDouble    = 12
	tiffTypeIFD       = 13
	tiffTypeLong8     = 16
	tiffTypeSLong8    = 17
	tiffTypeIFD8      = 18

	tiffTagStripOffsets             = 273
	tiffTagRowsPerStrip             = 278
	tiffTagStripByteCounts          = 279
	tiffTagTileOffsets              = 324
	tiffTagTileByteCounts           = 325
	tiffTagJPEGInterchangeFormat    = 513
	tiffTagJPEGInterchangeFormatLen = 514
)

var (
	errBigTIFFInvalid = errors.New("invalid BigTIFF container")
	errBigTIFFBounds  = errors.New("BigTIFF offsets exceed file bounds")
	errBigTIFFValue   = errors.New("BigTIFF values exceed classic TIFF bounds")
)

type classicIFDEntry struct {
	tag     uint16
	typ     uint16
	count   uint32
	data    []byte
	fileOff uint32
}

type bigTIFFEntry struct {
	tag   uint16
	typ   uint16
	count uint64
	data  []byte
}

type imageDataBlock struct {
	data []byte
}

type imageDataPair struct {
	offsetTag uint16
	countTag  uint16
	offsetIdx int
	countIdx  int
	blocks    []imageDataBlock
}

func isBigTIFF(data []byte) bool {
	if len(data) < bigTIFFHeaderSize {
		return false
	}
	return bytesEqual(data[:len(magicBigTIFFLE)], magicBigTIFFLE) ||
		bytesEqual(data[:len(magicBigTIFFBE)], magicBigTIFFBE)
}

func rewriteBigTIFF(raw []byte) ([]byte, error) {
	order, firstIFD, err := parseBigTIFFHeader(raw)
	if err != nil {
		return nil, err
	}

	bigEntries, err := parseBigTIFFIFD(raw, order, firstIFD)
	if err != nil {
		return nil, err
	}

	entries, pairs, err := buildClassicIFD(raw, order, bigEntries)
	if err != nil {
		return nil, err
	}

	payloadOff, err := assignClassicPayloadOffsets(entries)
	if err != nil {
		return nil, err
	}

	imageOff, err := assignImageBlockOffsets(order, entries, pairs, payloadOff)
	if err != nil {
		return nil, err
	}

	return writeClassicTIFF(raw, order, entries, pairs, payloadOff, imageOff)
}

func buildClassicIFD(raw []byte, order binary.ByteOrder, bigEntries []bigTIFFEntry) ([]classicIFDEntry, []imageDataPair, error) {
	entries := make([]classicIFDEntry, len(bigEntries))
	pairs := []imageDataPair{
		{offsetTag: tiffTagStripOffsets, countTag: tiffTagStripByteCounts, offsetIdx: -1, countIdx: -1},
		{offsetTag: tiffTagTileOffsets, countTag: tiffTagTileByteCounts, offsetIdx: -1, countIdx: -1},
		{offsetTag: tiffTagJPEGInterchangeFormat, countTag: tiffTagJPEGInterchangeFormatLen, offsetIdx: -1, countIdx: -1},
	}

	for i, entry := range bigEntries {
		typ, count, data, err := convertBigTIFFEntry(order, entry)
		if err != nil {
			return nil, nil, err
		}
		entries[i] = classicIFDEntry{tag: entry.tag, typ: typ, count: count, data: data}
		assignImageDataPairIndex(pairs, entry.tag, i)
	}

	for i := range pairs {
		pair := &pairs[i]
		if pair.offsetIdx < 0 || pair.countIdx < 0 {
			continue
		}
		offsetEntry, countEntry, blocks, err := buildImageDataEntries(
			raw,
			order,
			bigEntries[pair.offsetIdx],
			bigEntries[pair.countIdx],
			pair.offsetTag,
			pair.countTag,
		)
		if err != nil {
			return nil, nil, err
		}
		pair.blocks = blocks
		entries[pair.offsetIdx] = offsetEntry
		entries[pair.countIdx] = countEntry
	}

	return entries, pairs, nil
}

func assignImageDataPairIndex(pairs []imageDataPair, tag uint16, idx int) {
	for i := range pairs {
		switch tag {
		case pairs[i].offsetTag:
			pairs[i].offsetIdx = idx
		case pairs[i].countTag:
			pairs[i].countIdx = idx
		}
	}
}

func assignClassicPayloadOffsets(entries []classicIFDEntry) (int, error) {
	payloadOff := classicTIFFHeaderSize + classicTIFFEntryCountSize + len(entries)*classicTIFFEntrySize + classicTIFFNextIFDSize
	for i := range entries {
		if len(entries[i].data) <= classicTIFFInlineSize {
			continue
		}
		payloadOff = alignTIFFOffset(payloadOff)
		fileOff, err := checkedUint32(payloadOff)
		if err != nil {
			return 0, err
		}
		entries[i].fileOff = fileOff
		payloadOff += len(entries[i].data)
	}
	return payloadOff, nil
}

func assignImageBlockOffsets(order binary.ByteOrder, entries []classicIFDEntry, pairs []imageDataPair, start int) (int, error) {
	imageOff := start
	for i := range pairs {
		pair := &pairs[i]
		if pair.offsetIdx < 0 {
			continue
		}
		offsetData := entries[pair.offsetIdx].data
		for j := range pair.blocks {
			imageOff = alignTIFFOffset(imageOff)
			offset, err := checkedUint32(imageOff)
			if err != nil {
				return 0, err
			}
			order.PutUint32(offsetData[j*classicTIFFInlineSize:], offset)
			imageOff += len(pair.blocks[j].data)
		}
	}
	return imageOff, nil
}

func writeClassicTIFF(
	raw []byte,
	order binary.ByteOrder,
	entries []classicIFDEntry,
	pairs []imageDataPair,
	payloadOff, outSize int,
) ([]byte, error) {
	entryCount, err := checkedUint16(len(entries))
	if err != nil {
		return nil, err
	}

	out := make([]byte, outSize)
	copy(out[:2], raw[:2])
	order.PutUint16(out[2:], classicTIFFMagic)
	order.PutUint32(out[4:], classicTIFFHeaderSize)

	order.PutUint16(out[classicTIFFHeaderSize:], entryCount)
	entryOff := classicTIFFHeaderSize + classicTIFFEntryCountSize
	for i := range entries {
		entry := entries[i]
		order.PutUint16(out[entryOff:], entry.tag)
		order.PutUint16(out[entryOff+2:], entry.typ)
		order.PutUint32(out[entryOff+4:], entry.count)
		if len(entry.data) <= classicTIFFInlineSize {
			copy(out[entryOff+8:entryOff+8+len(entry.data)], entry.data)
		} else {
			order.PutUint32(out[entryOff+8:], entry.fileOff)
			copy(out[entry.fileOff:], entry.data)
		}
		entryOff += classicTIFFEntrySize
	}

	blockOff := payloadOff
	for i := range pairs {
		for j := range pairs[i].blocks {
			blockOff = alignTIFFOffset(blockOff)
			copy(out[blockOff:], pairs[i].blocks[j].data)
			blockOff += len(pairs[i].blocks[j].data)
		}
	}

	return out, nil
}

func parseBigTIFFHeader(raw []byte) (binary.ByteOrder, int, error) {
	if len(raw) < bigTIFFHeaderSize {
		return nil, 0, errBigTIFFInvalid
	}

	var order binary.ByteOrder
	switch {
	case bytesEqual(raw[:2], []byte{0x49, 0x49}):
		order = binary.LittleEndian
	case bytesEqual(raw[:2], []byte{0x4D, 0x4D}):
		order = binary.BigEndian
	default:
		return nil, 0, errBigTIFFInvalid
	}

	if order.Uint16(raw[2:4]) != 43 || order.Uint16(raw[4:6]) != 8 || order.Uint16(raw[6:8]) != 0 {
		return nil, 0, errBigTIFFInvalid
	}

	firstIFD := order.Uint64(raw[8:16])
	if firstIFD < bigTIFFHeaderSize || firstIFD > math.MaxInt {
		return nil, 0, errBigTIFFBounds
	}
	return order, int(firstIFD), nil
}

func parseBigTIFFIFD(raw []byte, order binary.ByteOrder, off int) ([]bigTIFFEntry, error) {
	if off+8 > len(raw) {
		return nil, errBigTIFFBounds
	}
	count := order.Uint64(raw[off : off+8])
	if count > math.MaxInt {
		return nil, errBigTIFFBounds
	}
	entryOff := off + bigTIFFEntryCountSize
	if entryOff+int(count)*bigTIFFEntrySize+8 > len(raw) {
		return nil, errBigTIFFBounds
	}

	entries := make([]bigTIFFEntry, 0, int(count))
	for i := 0; i < int(count); i++ {
		base := entryOff + i*bigTIFFEntrySize
		tag := order.Uint16(raw[base:])
		typ := order.Uint16(raw[base+2:])
		valCount := order.Uint64(raw[base+4:])
		_, ok := bigTIFFTypeSize(typ)
		if !ok {
			return nil, errBigTIFFInvalid
		}
		totalSize := valCount * bigTIFFTypeWidth(typ)
		valueField := raw[base+12 : base+20]
		var data []byte
		switch {
		case totalSize == 0:
		case totalSize <= bigTIFFInlineSize:
			data = valueField[:totalSize]
		default:
			offset := order.Uint64(valueField)
			if offset > math.MaxInt || totalSize > math.MaxInt {
				return nil, errBigTIFFBounds
			}
			start := int(offset)
			end := start + int(totalSize)
			if start < 0 || end < start || end > len(raw) {
				return nil, errBigTIFFBounds
			}
			data = raw[start:end]
		}
		entries = append(entries, bigTIFFEntry{tag: tag, typ: typ, count: valCount, data: data})
	}

	return entries, nil
}

func convertBigTIFFEntry(order binary.ByteOrder, entry bigTIFFEntry) (typ uint16, count uint32, data []byte, err error) {
	if entry.count > math.MaxUint32 {
		return 0, 0, nil, errBigTIFFValue
	}

	switch entry.typ {
	case tiffTypeLong8, tiffTypeIFD8:
		out := make([]byte, int(entry.count)*classicTIFFInlineSize)
		for i := 0; i < int(entry.count); i++ {
			v, readErr := bigTIFFUintAt(order, entry, i)
			if readErr != nil {
				return 0, 0, nil, readErr
			}
			if v > math.MaxUint32 {
				return 0, 0, nil, errBigTIFFValue
			}
			order.PutUint32(out[i*classicTIFFInlineSize:], uint32(v))
		}
		typ := uint16(tiffTypeLong)
		if entry.typ == tiffTypeIFD8 {
			typ = tiffTypeIFD
		}
		return typ, uint32(entry.count), out, nil
	case tiffTypeSLong8:
		out := make([]byte, int(entry.count)*classicTIFFInlineSize)
		for i := 0; i < int(entry.count); i++ {
			v, readErr := bigTIFFIntAt(order, entry, i)
			if readErr != nil {
				return 0, 0, nil, readErr
			}
			if v < math.MinInt32 || v > math.MaxInt32 {
				return 0, 0, nil, errBigTIFFValue
			}
			order.PutUint32(out[i*classicTIFFInlineSize:], uint32(int32(v))) //nolint:gosec
		}
		return tiffTypeSLong, uint32(entry.count), out, nil
	default:
		return entry.typ, uint32(entry.count), entry.data, nil
	}
}

func buildImageDataEntries(
	raw []byte,
	order binary.ByteOrder,
	offsetEntry, countEntry bigTIFFEntry,
	offsetTag, countTag uint16,
) (offset, count classicIFDEntry, blocks []imageDataBlock, err error) {
	if offsetEntry.count == 0 || offsetEntry.count != countEntry.count || offsetEntry.count > math.MaxUint32 {
		return classicIFDEntry{}, classicIFDEntry{}, nil, errBigTIFFInvalid
	}
	entryCount := int(offsetEntry.count)
	offsetData := make([]byte, entryCount*classicTIFFInlineSize)
	countData := make([]byte, entryCount*classicTIFFInlineSize)
	blocks = make([]imageDataBlock, entryCount)
	for i := 0; i < entryCount; i++ {
		entryOffset, readErr := bigTIFFUintAt(order, offsetEntry, i)
		if readErr != nil {
			return classicIFDEntry{}, classicIFDEntry{}, nil, readErr
		}
		size, readErr := bigTIFFUintAt(order, countEntry, i)
		if readErr != nil {
			return classicIFDEntry{}, classicIFDEntry{}, nil, readErr
		}
		if entryOffset > math.MaxInt || size > math.MaxUint32 || size > math.MaxInt {
			return classicIFDEntry{}, classicIFDEntry{}, nil, errBigTIFFBounds
		}
		start := int(entryOffset)
		length := int(size)
		end := start + length
		if start < 0 || end < start || end > len(raw) {
			return classicIFDEntry{}, classicIFDEntry{}, nil, errBigTIFFBounds
		}
		order.PutUint32(countData[i*classicTIFFInlineSize:], uint32(size))
		blocks[i] = imageDataBlock{data: raw[start:end]}
	}
	entryCount32, err := checkedUint32(entryCount)
	if err != nil {
		return classicIFDEntry{}, classicIFDEntry{}, nil, err
	}
	offset = classicIFDEntry{tag: offsetTag, typ: tiffTypeLong, count: entryCount32, data: offsetData}
	count = classicIFDEntry{tag: countTag, typ: tiffTypeLong, count: entryCount32, data: countData}
	return offset, count, blocks, nil
}

func bigTIFFUintAt(order binary.ByteOrder, entry bigTIFFEntry, idx int) (uint64, error) {
	size, ok := bigTIFFTypeSize(entry.typ)
	if !ok || idx < 0 || uint64(idx) >= entry.count {
		return 0, errBigTIFFInvalid
	}
	base := idx * size
	if base+size > len(entry.data) {
		return 0, errBigTIFFBounds
	}
	switch entry.typ {
	case tiffTypeByte, tiffTypeSByte, tiffTypeUndefined, tiffTypeASCII:
		return uint64(entry.data[base]), nil
	case tiffTypeShort, tiffTypeSShort:
		return uint64(order.Uint16(entry.data[base:])), nil
	case tiffTypeLong, tiffTypeIFD, tiffTypeSLong:
		return uint64(order.Uint32(entry.data[base:])), nil
	case tiffTypeLong8, tiffTypeIFD8, tiffTypeSLong8:
		return order.Uint64(entry.data[base:]), nil
	default:
		return 0, errBigTIFFInvalid
	}
}

func bigTIFFIntAt(order binary.ByteOrder, entry bigTIFFEntry, idx int) (int64, error) {
	if entry.typ != tiffTypeSLong8 {
		return 0, errBigTIFFInvalid
	}
	if idx < 0 || uint64(idx) >= entry.count {
		return 0, errBigTIFFInvalid
	}
	base := idx * tiffLong8Size
	if base+tiffLong8Size > len(entry.data) {
		return 0, errBigTIFFBounds
	}
	value := order.Uint64(entry.data[base:])
	if value > math.MaxInt64 {
		return 0, errBigTIFFValue
	}
	return int64(value), nil
}

func bigTIFFTypeSize(typ uint16) (int, bool) {
	switch typ {
	case tiffTypeByte, tiffTypeASCII, tiffTypeSByte, tiffTypeUndefined:
		return 1, true
	case tiffTypeShort, tiffTypeSShort:
		return tiffShortSize, true
	case tiffTypeLong, tiffTypeSLong, tiffTypeFloat, tiffTypeIFD:
		return tiffLongSize, true
	case tiffTypeRational, tiffTypeSRational, tiffTypeDouble, tiffTypeLong8, tiffTypeSLong8, tiffTypeIFD8:
		return tiffLong8Size, true
	default:
		return 0, false
	}
}

func bigTIFFTypeWidth(typ uint16) uint64 {
	switch typ {
	case tiffTypeByte, tiffTypeASCII, tiffTypeSByte, tiffTypeUndefined:
		return tiffByteWidth
	case tiffTypeShort, tiffTypeSShort:
		return tiffShortWidth
	case tiffTypeLong, tiffTypeSLong, tiffTypeFloat, tiffTypeIFD:
		return tiffLongWidth
	case tiffTypeRational, tiffTypeSRational, tiffTypeDouble, tiffTypeLong8, tiffTypeSLong8, tiffTypeIFD8:
		return tiffLong8Width
	default:
		return 0
	}
}

func alignTIFFOffset(off int) int {
	if off&1 != 0 {
		return off + 1
	}
	return off
}

func checkedUint16(v int) (uint16, error) {
	if v < 0 || v > math.MaxUint16 {
		return 0, errBigTIFFValue
	}
	return uint16(v), nil
}

func checkedUint32(v int) (uint32, error) {
	if v < 0 || v > math.MaxUint32 {
		return 0, errBigTIFFValue
	}
	return uint32(v), nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
