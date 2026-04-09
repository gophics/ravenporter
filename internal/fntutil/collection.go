package fntutil

import (
	"encoding/binary"
	"errors"
	"strconv"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/ir"
)

const (
	ttcfMagic          = "ttcf"
	ttcfHeaderSize     = 12
	ttcfOffsetEntryLen = 4
	sfntAlignMask      = 3
)

var errInvalidCollection = errors.New("invalid font collection")

func BuildFonts(raw []byte, format ir.FontFormat, defaultName string) ([]*ir.Font, error) {
	if !HasCollectionMagic(raw) {
		return []*ir.Font{buildFont(raw, format, defaultName)}, nil
	}

	members, err := ExtractCollectionMembers(raw)
	if err != nil {
		return nil, err
	}

	return BuildFontsFromMembers(members, format, defaultName), nil
}

func BuildFontsFromMembers(members [][]byte, format ir.FontFormat, defaultName string) []*ir.Font {
	fonts := make([]*ir.Font, 0, len(members))
	for i, raw := range members {
		font := buildFont(raw, format, defaultName)
		font.Name = font.Name + " #" + strconv.Itoa(i)
		fonts = append(fonts, font)
	}
	return fonts
}

func HasCollectionMagic(raw []byte) bool {
	return len(raw) >= len(ttcfMagic) && string(raw[:len(ttcfMagic)]) == ttcfMagic
}

func ExtractCollectionMembers(raw []byte) ([][]byte, error) {
	if len(raw) < ttcfHeaderSize || !HasCollectionMagic(raw) {
		return nil, errInvalidCollection
	}

	numFonts := int(binread.ReadU32BE(raw[8:12]))
	if numFonts <= 0 {
		return nil, errInvalidCollection
	}

	dirEnd := ttcfHeaderSize + numFonts*ttcfOffsetEntryLen
	if dirEnd > len(raw) {
		return nil, errInvalidCollection
	}

	members := make([][]byte, 0, numFonts)
	for i := range numFonts {
		offset := int(binread.ReadU32BE(raw[ttcfHeaderSize+i*ttcfOffsetEntryLen:]))
		member, err := extractStandaloneSFNT(raw, offset)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, nil
}

func extractStandaloneSFNT(raw []byte, base int) ([]byte, error) {
	if base < 0 || base+SFNTHeaderSize > len(raw) {
		return nil, errInvalidCollection
	}

	numTables := int(binread.ReadU16BE(raw[base+4:]))
	dirEnd := base + SFNTHeaderSize + numTables*SFNTTableEntSize
	if dirEnd > len(raw) {
		return nil, errInvalidCollection
	}

	type tableRef struct {
		tag      [4]byte
		checksum uint32
		offset   int
		length   int
	}

	tables := make([]tableRef, 0, numTables)
	size := SFNTHeaderSize + numTables*SFNTTableEntSize
	for i := range numTables {
		entryOff := base + SFNTHeaderSize + i*SFNTTableEntSize
		offset := int(binread.ReadU32BE(raw[entryOff+8:]))
		length := int(binread.ReadU32BE(raw[entryOff+12:]))
		if offset < 0 || length < 0 || offset+length > len(raw) {
			return nil, errInvalidCollection
		}

		var tag [4]byte
		copy(tag[:], raw[entryOff:entryOff+4])
		tables = append(tables, tableRef{
			tag:      tag,
			checksum: binread.ReadU32BE(raw[entryOff+4:]),
			offset:   offset,
			length:   length,
		})
		size += alignSFNT(length)
	}

	out := make([]byte, size)
	copy(out[:4], raw[base:base+4])
	if numTables < 0 || numTables > int(^uint16(0)) {
		return nil, errInvalidCollection
	}
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables))
	searchRange, entrySelector, rangeShift := CalcSFNTSearchParams(numTables)
	binary.BigEndian.PutUint16(out[6:8], searchRange)
	binary.BigEndian.PutUint16(out[8:10], entrySelector)
	binary.BigEndian.PutUint16(out[10:12], rangeShift)

	dataOff := SFNTHeaderSize + numTables*SFNTTableEntSize
	for i, table := range tables {
		entryOff := SFNTHeaderSize + i*SFNTTableEntSize
		if dataOff < 0 {
			return nil, errInvalidCollection
		}
		dataOff64 := uint64(dataOff)
		if dataOff64 > uint64(^uint32(0)) {
			return nil, errInvalidCollection
		}
		if table.length < 0 {
			return nil, errInvalidCollection
		}
		tableLen64 := uint64(table.length)
		if tableLen64 > uint64(^uint32(0)) {
			return nil, errInvalidCollection
		}
		copy(out[entryOff:entryOff+4], table.tag[:])
		binary.BigEndian.PutUint32(out[entryOff+4:entryOff+8], table.checksum)
		binary.BigEndian.PutUint32(out[entryOff+8:entryOff+12], uint32(dataOff64))
		binary.BigEndian.PutUint32(out[entryOff+12:entryOff+16], uint32(tableLen64))
		copy(out[dataOff:dataOff+table.length], raw[table.offset:table.offset+table.length])
		dataOff += alignSFNT(table.length)
	}

	return out, nil
}

func alignSFNT(n int) int {
	return (n + sfntAlignMask) &^ sfntAlignMask
}

func buildFont(raw []byte, format ir.FontFormat, defaultName string) *ir.Font {
	font := &ir.Font{
		Name:   defaultName,
		Format: format,
		Vector: &ir.VectorFontData{RawData: raw},
	}
	ParseSFNTMetrics(raw, font)
	if font.Name == "" {
		font.Name = defaultName
	}
	return font
}
