package fntutil

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
)

func TestCleanString(t *testing.T) {
	assert.Equal(t, "Hello", CleanString([]byte("Hello")))
	assert.Equal(t, "Hello", CleanString([]byte{'H', 0, 'e', 0, 'l', 'l', 'o'}))
}

func TestReadBase128(t *testing.T) {
	value, consumed := ReadBase128([]byte{0x81, 0x01})
	assert.Equal(t, uint32(129), value)
	assert.Equal(t, 2, consumed)

	value, consumed = ReadBase128([]byte{0x80})
	assert.Zero(t, value)
	assert.Zero(t, consumed)
}

func TestCalcSFNTSearchParams(t *testing.T) {
	searchRange, entrySelector, rangeShift := CalcSFNTSearchParams(5)
	assert.Equal(t, uint16(64), searchRange)
	assert.Equal(t, uint16(2), entrySelector)
	assert.Equal(t, uint16(16), rangeShift)
}

func TestParseOS2HeadMaxpHheaAndHmtx(t *testing.T) {
	font := &ir.Font{Vector: &ir.VectorFontData{Codepoints: []rune{'A', 'B'}}}

	os2 := make([]byte, os2MinSize)
	putI16BE(os2[68:70], 800)
	putI16BE(os2[70:72], -200)
	putI16BE(os2[72:74], 120)
	parseOS2(os2, font)
	assert.Equal(t, 800, font.Vector.Ascender)
	assert.Equal(t, -200, font.Vector.Descender)
	assert.Equal(t, 120, font.Vector.LineGap)

	head := make([]byte, headMinSize)
	putU16BE(head[headUnitsPerEmOff:], 2048)
	parseHead(head, font)
	assert.Equal(t, 2048, font.Vector.UnitsPerEm)

	maxp := make([]byte, maxpMinSize)
	putU16BE(maxp[maxpNumGlyphsOff:], 64)
	parseMaxp(maxp, font)
	assert.Equal(t, 64, font.Vector.GlyphCount)

	hhea := make([]byte, hheaMinSize)
	putU16BE(hhea[hheaNumHMetricsOff:], 2)
	assert.Equal(t, 2, parseHhea(hhea))

	hmtx := make([]byte, 8)
	putU16BE(hmtx[0:], 500)
	putU16BE(hmtx[4:], 750)
	parseHmtx(hmtx, font, 2)
	require.NotNil(t, font.Vector.Advances)
	assert.Equal(t, 750, font.Vector.Advances['A'])
	assert.Equal(t, 750, font.Vector.Advances['B'])
}

func TestParseNameTable(t *testing.T) {
	font := &ir.Font{}
	data := buildNameTable(
		nameRecord{nameID: nameIDFamily, value: "Raven"},
		nameRecord{nameID: nameIDSubfamily, value: "Regular"},
		nameRecord{nameID: nameIDPostScript, value: "Raven-Regular"},
		nameRecord{nameID: nameIDDesigner, value: "JR"},
	)

	parseNameTable(data, font)
	assert.Equal(t, "Raven", font.Name)
	assert.Equal(t, "Raven", font.Family)
	assert.Equal(t, "Regular", font.Subfamily)
	assert.Equal(t, "Raven-Regular", font.PostScript)
	assert.Equal(t, "JR", font.Metadata[metaDesigner])
}

func TestParseCmapFormats(t *testing.T) {
	fmt4 := buildCmapFmt4(0x0041, 0x0042)
	assert.Equal(t, []rune{'A', 'B'}, parseCmapFmt4(fmt4))

	fmt12 := buildCmapFmt12(0x1F600, 0x1F601)
	assert.Equal(t, []rune{0x1F600, 0x1F601}, parseCmapFmt12(fmt12))

	font := &ir.Font{Vector: &ir.VectorFontData{}}
	parseCmap(buildCmapTable(fmt4), font)
	assert.Equal(t, []rune{'A', 'B'}, font.Vector.Codepoints)
}

func TestParseKern(t *testing.T) {
	font := &ir.Font{Vector: &ir.VectorFontData{}}
	data := make([]byte, kernHeaderSize+kernSubHeaderLen+kernPairSize)
	putU16BE(data[2:4], 1)
	off := kernHeaderSize
	putU16BE(data[off+2:], uint16(len(data)-kernHeaderSize))
	data[off+4] = 0
	putU16BE(data[off+6:], 1)
	putU16BE(data[off+kernSubHeaderLen:], uint16('A'))
	putU16BE(data[off+kernSubHeaderLen+2:], uint16('V'))
	putI16BE(data[off+kernSubHeaderLen+4:], -50)

	parseKern(data, font)
	require.Len(t, font.Vector.Kerning, 1)
	assert.Equal(t, ir.KerningPair{First: 'A', Second: 'V', Amount: -50}, font.Vector.Kerning[0])
}

func TestParseSFNTMetrics(t *testing.T) {
	font := &ir.Font{}
	sfnt := buildSFNT(
		tableEntry{tag: [4]byte{'O', 'S', '/', '2'}, data: func() []byte {
			data := make([]byte, os2MinSize)
			putI16BE(data[68:70], 700)
			putI16BE(data[70:72], -100)
			return data
		}()},
		tableEntry{tag: [4]byte{'n', 'a', 'm', 'e'}, data: buildNameTable(nameRecord{nameID: nameIDFamily, value: "Raven"})},
		tableEntry{tag: [4]byte{'h', 'e', 'a', 'd'}, data: func() []byte {
			data := make([]byte, headMinSize)
			putU16BE(data[headUnitsPerEmOff:], 1024)
			return data
		}()},
		tableEntry{tag: [4]byte{'m', 'a', 'x', 'p'}, data: func() []byte {
			data := make([]byte, maxpMinSize)
			putU16BE(data[maxpNumGlyphsOff:], 2)
			return data
		}()},
		tableEntry{tag: [4]byte{'c', 'm', 'a', 'p'}, data: buildCmapTable(buildCmapFmt4(0x0041, 0x0041))},
		tableEntry{tag: [4]byte{'h', 'h', 'e', 'a'}, data: func() []byte {
			data := make([]byte, hheaMinSize)
			putU16BE(data[hheaNumHMetricsOff:], 1)
			return data
		}()},
		tableEntry{tag: [4]byte{'h', 'm', 't', 'x'}, data: func() []byte {
			data := make([]byte, 4)
			putU16BE(data, 500)
			return data
		}()},
	)

	ParseSFNTMetrics(sfnt, font)
	require.NotNil(t, font.Vector)
	assert.Equal(t, "Raven", font.Family)
	assert.Equal(t, 1024, font.Vector.UnitsPerEm)
	assert.Equal(t, 2, font.Vector.GlyphCount)
	assert.Equal(t, []rune{'A'}, font.Vector.Codepoints)
	assert.Equal(t, 500, font.Vector.Advances['A'])
}

type tableEntry struct {
	tag  [4]byte
	data []byte
}

type nameRecord struct {
	nameID int
	value  string
}

func buildSFNT(entries ...tableEntry) []byte {
	data := make([]byte, SFNTHeaderSize+len(entries)*SFNTTableEntSize)
	putU16BE(data[4:6], uint16(len(entries)))
	offset := len(data)
	var tables bytes.Buffer
	for i, entry := range entries {
		dirOff := SFNTHeaderSize + i*SFNTTableEntSize
		copy(data[dirOff:dirOff+4], entry.tag[:])
		putU32BE(data[dirOff+8:dirOff+12], uint32(offset))
		putU32BE(data[dirOff+12:dirOff+16], uint32(len(entry.data)))
		offset += len(entry.data)
		tables.Write(entry.data)
	}
	return append(data, tables.Bytes()...)
}

func buildNameTable(records ...nameRecord) []byte {
	var stringData bytes.Buffer
	data := make([]byte, nameRecordOff+len(records)*nameRecordSize)
	putU16BE(data[2:4], uint16(len(records)))
	putU16BE(data[4:6], uint16(len(data)))
	for i, record := range records {
		off := nameRecordOff + i*nameRecordSize
		value := []byte(record.value)
		putU16BE(data[off+6:off+8], uint16(record.nameID))
		putU16BE(data[off+8:off+10], uint16(len(value)))
		putU16BE(data[off+10:off+12], uint16(stringData.Len()))
		stringData.Write(value)
	}
	return append(data, stringData.Bytes()...)
}

func buildCmapTable(subtable []byte) []byte {
	data := make([]byte, cmapHeaderSize+cmapRecordSize)
	putU16BE(data[2:4], 1)
	putU16BE(data[4:6], cmapPlatUnicode)
	putU16BE(data[6:8], cmapEncBMP)
	putU32BE(data[8:12], uint32(len(data)))
	return append(data, subtable...)
}

func buildCmapFmt4(start, end uint16) []byte {
	data := make([]byte, 32)
	putU16BE(data[0:2], cmapFmt4ID)
	putU16BE(data[2:4], uint16(len(data)))
	putU16BE(data[6:8], 4)
	putU16BE(data[14:16], end)
	putU16BE(data[16:18], 0xFFFF)
	putU16BE(data[20:22], start)
	putU16BE(data[22:24], 0xFFFF)
	return data
}

func buildCmapFmt12(start, end uint32) []byte {
	data := make([]byte, 28)
	putU16BE(data[0:2], cmapFmt12ID)
	putU32BE(data[4:8], uint32(len(data)))
	putU32BE(data[12:16], 1)
	putU32BE(data[16:20], start)
	putU32BE(data[20:24], end)
	return data
}

func putU16BE(dst []byte, value uint16) {
	dst[0] = byte(value >> 8)
	dst[1] = byte(value)
}

func putI16BE(dst []byte, value int16) {
	putU16BE(dst, uint16(value))
}

func putU32BE(dst []byte, value uint32) {
	dst[0] = byte(value >> 24)
	dst[1] = byte(value >> 16)
	dst[2] = byte(value >> 8)
	dst[3] = byte(value)
}
