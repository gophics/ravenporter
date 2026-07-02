package fntutil

import (
	"unicode/utf8"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/ir"
)

// SFNT layout constants.
const (
	SFNTHeaderSize   = 12
	SFNTTableEntSize = 16
	sfntMagicLen     = 4

	os2MinSize  = 78
	headMinSize = 54
	maxpMinSize = 6
	hheaMinSize = 36

	headUnitsPerEmOff  = 18
	maxpNumGlyphsOff   = 4
	hheaNumHMetricsOff = 34
	base128MaxBytes    = 5

	nameRecordOff  = 6
	nameRecordSize = 12
	nameCountLimit = 1000

	nameIDCopyright    = 0
	nameIDFamily       = 1
	nameIDSubfamily    = 2
	nameIDPostScript   = 6
	nameIDTrademark    = 7
	nameIDManufacturer = 8
	nameIDDesigner     = 9

	metaCopyright    = "copyright"
	metaTrademark    = "trademark"
	metaManufacturer = "manufacturer"
	metaDesigner     = "designer"

	wantedTables = 7

	cmapHeaderSize   = 4
	cmapRecordSize   = 8
	cmapFmt4MinSize  = 14
	cmapFmt12MinSize = 16
	cmapFmt4ID       = 4
	cmapFmt12ID      = 12
	cmapPlatUnicode  = 0
	cmapPlatWindows  = 3
	cmapEncBMP       = 1
	cmapEncFull      = 10

	kernHeaderSize   = 4
	kernSubMinSize   = 14
	kernPairSize     = 6
	kernFormat0      = 0
	kernSubHeaderLen = 14
)

// ParseSFNTMetrics extracts basic metrics from raw SFNT table data into f.
func ParseSFNTMetrics(data []byte, f *ir.Font) {
	if len(data) < SFNTHeaderSize {
		return
	}
	numTables := int(binread.ReadU16BE(data[4:6]))

	type tableRef struct {
		off, len int
	}
	tables := make(map[[4]byte]tableRef, numTables)

	for i := range numTables {
		off := SFNTHeaderSize + i*SFNTTableEntSize
		if off+SFNTTableEntSize > len(data) {
			break
		}
		var tag [4]byte
		copy(tag[:], data[off:off+sfntMagicLen])
		tblOff := int(binread.ReadU32BE(data[off+8 : off+12]))
		tblLen := int(binread.ReadU32BE(data[off+12 : off+SFNTTableEntSize]))
		if tblOff+tblLen <= len(data) {
			tables[tag] = tableRef{tblOff, tblLen}
		}
	}

	if ref, ok := tables[[4]byte{'O', 'S', '/', '2'}]; ok {
		parseOS2(data[ref.off:ref.off+ref.len], f)
	}
	if ref, ok := tables[[4]byte{'n', 'a', 'm', 'e'}]; ok {
		parseNameTable(data[ref.off:ref.off+ref.len], f)
	}
	if ref, ok := tables[[4]byte{'h', 'e', 'a', 'd'}]; ok {
		parseHead(data[ref.off:ref.off+ref.len], f)
	}
	if ref, ok := tables[[4]byte{'m', 'a', 'x', 'p'}]; ok {
		parseMaxp(data[ref.off:ref.off+ref.len], f)
	}
	if ref, ok := tables[[4]byte{'c', 'm', 'a', 'p'}]; ok {
		parseCmap(data[ref.off:ref.off+ref.len], f)
	}

	var numHMetrics int
	if ref, ok := tables[[4]byte{'h', 'h', 'e', 'a'}]; ok {
		numHMetrics = parseHhea(data[ref.off : ref.off+ref.len])
	}
	if numHMetrics > 0 {
		if ref, ok := tables[[4]byte{'h', 'm', 't', 'x'}]; ok {
			parseHmtx(data[ref.off:ref.off+ref.len], f, numHMetrics)
		}
	}
	if ref, ok := tables[[4]byte{'k', 'e', 'r', 'n'}]; ok {
		parseKern(data[ref.off:ref.off+ref.len], f)
	}
}

func parseOS2(data []byte, f *ir.Font) {
	if len(data) < os2MinSize {
		return
	}
	if f.Vector == nil {
		f.Vector = &ir.VectorFontData{}
	}
	f.Vector.Ascender = int(binread.ReadI16BE(data[68:70]))
	f.Vector.Descender = int(binread.ReadI16BE(data[70:72]))
	f.Vector.LineGap = int(binread.ReadI16BE(data[72:74]))
}

func parseNameTable(data []byte, f *ir.Font) {
	if len(data) < nameRecordOff {
		return
	}
	count := int(binread.ReadU16BE(data[2:4]))
	strOff := int(binread.ReadU16BE(data[4:6]))
	if strOff > len(data) || count > nameCountLimit {
		return
	}

	for i := range count {
		off := nameRecordOff + i*nameRecordSize
		if off+nameRecordSize > len(data) {
			break
		}
		nameID := int(binread.ReadU16BE(data[off+6 : off+8]))
		length := int(binread.ReadU16BE(data[off+8 : off+10]))
		sOff := int(binread.ReadU16BE(data[off+10 : off+12]))

		start := strOff + sOff
		end := start + length
		if end > len(data) {
			continue
		}

		switch nameID {
		case nameIDFamily:
			if f.Family != "" {
				continue
			}
			s := CleanString(data[start:end])
			f.Family = s
			f.Name = s
		case nameIDSubfamily:
			if f.Subfamily != "" {
				continue
			}
			f.Subfamily = CleanString(data[start:end])
		case nameIDPostScript:
			if f.PostScript != "" {
				continue
			}
			f.PostScript = CleanString(data[start:end])
		case nameIDCopyright:
			setFontMeta(f, metaCopyright, data[start:end])
		case nameIDTrademark:
			setFontMeta(f, metaTrademark, data[start:end])
		case nameIDManufacturer:
			setFontMeta(f, metaManufacturer, data[start:end])
		case nameIDDesigner:
			setFontMeta(f, metaDesigner, data[start:end])
		}
	}
}

func parseHead(data []byte, f *ir.Font) {
	if len(data) < headMinSize || f.Vector == nil {
		return
	}
	f.Vector.UnitsPerEm = int(binread.ReadU16BE(data[headUnitsPerEmOff:]))
}

func parseMaxp(data []byte, f *ir.Font) {
	if len(data) < maxpMinSize || f.Vector == nil {
		return
	}
	f.Vector.GlyphCount = int(binread.ReadU16BE(data[maxpNumGlyphsOff:]))
}

func parseCmap(data []byte, f *ir.Font) {
	if len(data) < cmapHeaderSize || f.Vector == nil {
		return
	}
	numSubtables := int(binread.ReadU16BE(data[2:4]))
	for i := range numSubtables {
		recOff := cmapHeaderSize + i*cmapRecordSize
		if recOff+cmapRecordSize > len(data) {
			break
		}
		platID := int(binread.ReadU16BE(data[recOff:]))
		encID := int(binread.ReadU16BE(data[recOff+2:]))
		subtableOff := int(binread.ReadU32BE(data[recOff+4:]))
		if subtableOff+2 > len(data) {
			continue
		}

		isUnicode := platID == cmapPlatUnicode ||
			(platID == cmapPlatWindows && (encID == cmapEncBMP || encID == cmapEncFull))
		if !isUnicode {
			continue
		}

		format := int(binread.ReadU16BE(data[subtableOff:]))
		switch format {
		case cmapFmt4ID:
			f.Vector.Codepoints = parseCmapFmt4(data[subtableOff:])
			if len(f.Vector.Codepoints) > 0 {
				return
			}
		case cmapFmt12ID:
			f.Vector.Codepoints = parseCmapFmt12(data[subtableOff:])
			if len(f.Vector.Codepoints) > 0 {
				return
			}
		}
	}
}

func parseCmapFmt4(data []byte) []rune {
	if len(data) < cmapFmt4MinSize {
		return nil
	}
	segCount := int(binread.ReadU16BE(data[6:8])) / 2 //nolint:mnd // segCount*2 per spec
	baseOff := cmapFmt4MinSize
	if baseOff+segCount*2*4 > len(data) {
		return nil
	}

	endCodes := data[baseOff:]
	startCodes := data[baseOff+segCount*2+2:]

	var codepoints []rune
	for i := range segCount {
		endCode := int(binread.ReadU16BE(endCodes[i*2:]))
		startCode := int(binread.ReadU16BE(startCodes[i*2:]))
		if startCode == 0xFFFF { //nolint:mnd // cmap sentinel
			break
		}
		for c := startCode; c <= endCode; c++ {
			codepoints = append(codepoints, rune(c)) //nolint:gosec // cmap format 4 codepoints are uint16 range.
		}
	}
	return codepoints
}

func parseCmapFmt12(data []byte) []rune {
	if len(data) < cmapFmt12MinSize {
		return nil
	}
	nGroups := int(binread.ReadU32BE(data[12:16]))
	var codepoints []rune
	for i := range nGroups {
		off := cmapFmt12MinSize + i*12 //nolint:mnd // 12 bytes per group
		if off+12 > len(data) {
			break
		}
		startCode := binread.ReadU32BE(data[off:])
		endCode := binread.ReadU32BE(data[off+4:])
		for c := startCode; c <= endCode; c++ {
			if c > utf8.MaxRune {
				break
			}
			codepoints = append(codepoints, rune(c)) //nolint:gosec // bounded by utf8.MaxRune above.
		}
	}
	return codepoints
}

func parseHhea(data []byte) int {
	if len(data) < hheaMinSize {
		return 0
	}
	return int(binread.ReadU16BE(data[hheaNumHMetricsOff:]))
}

func parseHmtx(data []byte, f *ir.Font, numHMetrics int) {
	if f.Vector == nil || len(f.Vector.Codepoints) == 0 {
		return
	}
	hmtxRecordSize := 4
	needed := numHMetrics * hmtxRecordSize
	if needed > len(data) {
		return
	}

	advances := make([]int, numHMetrics)
	for i := range numHMetrics {
		advances[i] = int(binread.ReadU16BE(data[i*hmtxRecordSize:]))
	}

	f.Vector.Advances = make(map[rune]int, len(f.Vector.Codepoints))
	for _, cp := range f.Vector.Codepoints {
		glyphIdx := int(cp) // simplified: codepoint as glyph index
		if glyphIdx >= numHMetrics {
			glyphIdx = numHMetrics - 1
		}
		if glyphIdx >= 0 && glyphIdx < len(advances) {
			f.Vector.Advances[cp] = advances[glyphIdx]
		}
	}
}

func parseKern(data []byte, f *ir.Font) {
	if len(data) < kernHeaderSize || f.Vector == nil {
		return
	}
	nTables := int(binread.ReadU16BE(data[2:4]))
	off := kernHeaderSize
	for range nTables {
		if off+kernSubMinSize > len(data) {
			break
		}
		subLen := int(binread.ReadU16BE(data[off+2:]))
		format := data[off+4] >> 4 //nolint:mnd // upper nibble = format
		if format != kernFormat0 {
			off += subLen
			continue
		}
		nPairs := int(binread.ReadU16BE(data[off+6:]))
		pairOff := off + kernSubHeaderLen
		for i := range nPairs {
			p := pairOff + i*kernPairSize
			if p+kernPairSize > len(data) {
				break
			}
			left := rune(binread.ReadU16BE(data[p:]))
			right := rune(binread.ReadU16BE(data[p+2:]))
			value := int(binread.ReadI16BE(data[p+4:]))
			f.Vector.Kerning = append(f.Vector.Kerning, ir.KerningPair{
				First: left, Second: right, Amount: value,
			})
		}
		off += subLen
	}
}

func setFontMeta(f *ir.Font, key string, raw []byte) {
	if len(raw) == 0 {
		return
	}
	if f.Metadata == nil {
		f.Metadata = make(map[string]string, wantedTables)
	}
	if _, ok := f.Metadata[key]; ok {
		return
	}
	f.Metadata[key] = CleanString(raw)
}

// CleanString strips null bytes from a font name string.
// Zero-alloc fast path when no null bytes are present.
func CleanString(b []byte) string {
	hasNull := false
	for _, c := range b {
		if c == 0 {
			hasNull = true
			break
		}
	}
	if !hasNull {
		return string(b)
	}
	n := 0
	for _, c := range b {
		if c != 0 {
			b[n] = c
			n++
		}
	}
	return string(b[:n])
}

// ReadBase128 decodes a UIntBase128 value per the WOFF2 spec.
func ReadBase128(data []byte) (val uint32, consumed int) {
	for i, b := range data[:min(len(data), base128MaxBytes)] {
		if i == 0 && b == 0x80 {
			return 0, 0
		}
		val = (val << 7) | uint32(b&0x7F) //nolint:mnd // spec encoding
		if b&0x80 == 0 {
			return val, i + 1
		}
	}
	return 0, 0
}

// CalcSFNTSearchParams computes SFNT table directory search parameters.
func CalcSFNTSearchParams(n int) (searchRange, entrySelector, rangeShift uint16) {
	es := 0
	sr := 1
	for sr*2 <= n {
		sr *= 2
		es++
	}
	sr *= SFNTTableEntSize
	return uint16(sr), uint16(es), uint16(n*SFNTTableEntSize - sr) //nolint:gosec // safe arithmetic
}
