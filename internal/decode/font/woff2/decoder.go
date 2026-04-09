package woff2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sort"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/fntutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName  = "WOFF2"
	decodedName = "WOFF2 Font"
	extWOFF2    = ".woff2"

	woff2HeaderSize    = 48
	woff2FlavorOff     = 4
	woff2NumTablesOff  = 12
	woff2SfntSizeOff   = 16
	woff2CompressedOff = 20

	tableAlignMask = 3

	flagTransformMask = 0x3F
	flagKnownTag      = 0x3F

	collectionWordCode     = 253
	collectionOneByteCode1 = 255
	collectionOneByteCode2 = 254
	collectionSmallBase    = uint16(253)
	collectionMediumBase   = uint16(506)
	woff2CollectionFlavor  = "ttcf"
)

var (
	magic        = []byte("wOF2")
	errTruncated = errors.New("truncated WOFF2 data")
	extensions   = []string{extWOFF2}

	knownTags = [...]string{
		"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post",
		"cvt ", "fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT",
		"EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea",
		"vmtx", "BASE", "GDEF", "GPOS", "GSUB", "EBSC", "JSTF", "MATH",
		"CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar",
		"bdat", "bloc", "bsln", "cvar", "fdsc", "feat", "fmtx", "fvar",
		"gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop",
		"trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
	}
)

type tableEntry struct {
	tag       [4]byte
	origLen   uint32
	transLen  uint32
	transform uint8
}

type tableChunk struct {
	tag      [4]byte
	origLen  uint32
	transLen uint32
	dataOff  int
}

type collectionEntry struct {
	flavor  [4]byte
	indices []uint16
}

// Decoder implements detect.Decoder for WOFF2 fonts.
type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatWOFF2, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return decutil.ProbeBytes(r, magic) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatWOFF2, "size", err)
	}
	raw, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatWOFF2, "read", err)
	}

	fontData, err := decompressWOFF2(raw)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatWOFF2, "decompress", err)
	}

	fonts := fntutil.BuildFontsFromMembers(fontData, ir.FontWOFF2, decodedName)
	if len(fontData) == 1 {
		fonts, err = fntutil.BuildFonts(fontData[0], ir.FontWOFF2, decodedName)
		if err != nil {
			return nil, decutil.DecodeErr(ir.FormatWOFF2, "build font", err)
		}
	}

	return &ir.Asset{
		Fonts:    fonts,
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatWOFF2},
	}, nil
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }

type woff2Directory struct {
	entries         []tableEntry
	flavor          []byte
	totalSfntSize   int
	totalCompressed int
	pos             int
}

func decompressWOFF2(data []byte) ([][]byte, error) {
	dir, err := readWOFF2Directory(data)
	if err != nil {
		return nil, err
	}

	var collections []collectionEntry
	if bytes.Equal(dir.flavor, []byte(woff2CollectionFlavor)) {
		collections, dir.pos, err = parseCollectionDirectory(data, dir.pos)
		if err != nil {
			return nil, err
		}
	}

	compStart := dir.pos
	compEnd := compStart + dir.totalCompressed
	if compEnd > len(data) {
		return nil, errTruncated
	}

	decompressed, err := pool.BrotliDecodeSized(data[compStart:compEnd], dir.totalSfntSize)
	if err != nil {
		return nil, err
	}

	chunks, err := buildTableChunks(dir.entries, decompressed)
	if err != nil {
		return nil, err
	}

	return buildWOFF2Fonts(dir.flavor, collections, chunks, decompressed)
}

func readWOFF2Directory(data []byte) (woff2Directory, error) {
	if len(data) < woff2HeaderSize {
		return woff2Directory{}, errTruncated
	}

	numTables := int(binread.ReadU16BE(data[woff2NumTablesOff:]))
	dir := woff2Directory{
		flavor:          data[woff2FlavorOff : woff2FlavorOff+4],
		totalSfntSize:   int(binread.ReadU32BE(data[woff2SfntSizeOff:])),
		totalCompressed: int(binread.ReadU32BE(data[woff2CompressedOff:])),
	}

	var stackEntries [64]tableEntry
	entries := stackEntries[:0]
	if numTables > len(stackEntries) {
		entries = make([]tableEntry, 0, numTables)
	}

	dir.pos = woff2HeaderSize
	for range numTables {
		entry, nextPos, err := readWOFF2TableEntry(data, dir.pos)
		if err != nil {
			return woff2Directory{}, err
		}
		dir.pos = nextPos
		entries = append(entries, entry)
	}

	dir.entries = entries
	return dir, nil
}

func readWOFF2TableEntry(data []byte, pos int) (tableEntry, int, error) {
	if pos >= len(data) {
		return tableEntry{}, 0, errTruncated
	}

	flagByte := data[pos]
	pos++

	entry := tableEntry{transform: (flagByte >> 6) & 0x03} //nolint:mnd // spec bits
	tagIdx := flagByte & flagTransformMask

	switch {
	case tagIdx == flagKnownTag:
		if pos+4 > len(data) {
			return tableEntry{}, 0, errTruncated
		}
		copy(entry.tag[:], data[pos:pos+4])
		pos += 4
	case int(tagIdx) < len(knownTags):
		copy(entry.tag[:], knownTags[tagIdx])
	default:
		return tableEntry{}, 0, errTruncated
	}

	origLen, n := fntutil.ReadBase128(data[pos:])
	if n == 0 {
		return tableEntry{}, 0, errTruncated
	}
	pos += n
	entry.origLen = origLen

	if entry.transform != 0 {
		transLen, tn := fntutil.ReadBase128(data[pos:])
		if tn == 0 {
			return tableEntry{}, 0, errTruncated
		}
		pos += tn
		entry.transLen = transLen
	} else {
		entry.transLen = origLen
	}

	return entry, pos, nil
}

func buildWOFF2Fonts(
	flavor []byte, collections []collectionEntry, chunks []tableChunk, decompressed []byte,
) ([][]byte, error) {
	if len(collections) == 0 {
		sfnt, err := rebuildSFNT(flavor, chunks, decompressed)
		if err != nil {
			return nil, err
		}
		return [][]byte{sfnt}, nil
	}

	fonts := make([][]byte, 0, len(collections))
	for _, entry := range collections {
		sfnt, err := rebuildCollectionMember(entry.flavor[:], entry.indices, chunks, decompressed)
		if err != nil {
			return nil, err
		}
		fonts = append(fonts, sfnt)
	}
	return fonts, nil
}

func buildTableChunks(entries []tableEntry, decompressed []byte) ([]tableChunk, error) {
	chunks := make([]tableChunk, 0, len(entries))
	srcOff := 0
	for _, entry := range entries {
		tblLen := int(entry.transLen)
		if srcOff+tblLen > len(decompressed) {
			return nil, errTruncated
		}
		chunks = append(chunks, tableChunk{
			tag:      entry.tag,
			origLen:  entry.origLen,
			transLen: entry.transLen,
			dataOff:  srcOff,
		})
		srcOff += tblLen
	}
	return chunks, nil
}

func rebuildSFNT(flavor []byte, chunks []tableChunk, decompressed []byte) ([]byte, error) {
	n := len(chunks)

	sfntSize := fntutil.SFNTHeaderSize + n*fntutil.SFNTTableEntSize
	for _, chunk := range chunks {
		sfntSize += (int(chunk.origLen) + tableAlignMask) &^ tableAlignMask
	}

	sfnt := make([]byte, sfntSize)
	copy(sfnt[:4], flavor)
	binary.BigEndian.PutUint16(sfnt[4:6], uint16(n)) //nolint:gosec // bounded by u16 source

	sr, es, rs := fntutil.CalcSFNTSearchParams(n)
	binary.BigEndian.PutUint16(sfnt[6:8], sr)
	binary.BigEndian.PutUint16(sfnt[8:10], es)
	binary.BigEndian.PutUint16(sfnt[10:12], rs)

	dataOff := fntutil.SFNTHeaderSize + n*fntutil.SFNTTableEntSize
	for i, chunk := range chunks {
		entOff := fntutil.SFNTHeaderSize + i*fntutil.SFNTTableEntSize
		copy(sfnt[entOff:], chunk.tag[:])
		binary.BigEndian.PutUint32(sfnt[entOff+8:], uint32(dataOff)) //nolint:gosec // positive int
		binary.BigEndian.PutUint32(sfnt[entOff+12:], chunk.origLen)

		tblLen := int(chunk.transLen)
		if chunk.dataOff+tblLen > len(decompressed) {
			return nil, errTruncated
		}
		if tblLen > 0 {
			copy(sfnt[dataOff:], decompressed[chunk.dataOff:chunk.dataOff+tblLen])
		}

		dataOff += (int(chunk.origLen) + tableAlignMask) &^ tableAlignMask
	}

	return sfnt, nil
}

func parseCollectionDirectory(data []byte, pos int) ([]collectionEntry, int, error) {
	if pos+4 > len(data) {
		return nil, pos, errTruncated
	}
	version := binary.BigEndian.Uint32(data[pos : pos+4])
	pos += 4
	numFonts, err := read255UShort(data, &pos)
	if err != nil {
		return nil, pos, err
	}

	entries := make([]collectionEntry, 0, numFonts)
	for range int(numFonts) {
		numTables, err := read255UShort(data, &pos)
		if err != nil {
			return nil, pos, err
		}
		if pos+4 > len(data) {
			return nil, pos, errTruncated
		}

		var flavor [4]byte
		copy(flavor[:], data[pos:pos+4])
		pos += 4

		indices := make([]uint16, 0, numTables)
		for range int(numTables) {
			idx, err := read255UShort(data, &pos)
			if err != nil {
				return nil, pos, err
			}
			indices = append(indices, idx)
		}

		entries = append(entries, collectionEntry{flavor: flavor, indices: indices})
	}

	_ = version
	return entries, pos, nil
}

func read255UShort(data []byte, pos *int) (uint16, error) {
	if *pos >= len(data) {
		return 0, errTruncated
	}
	code := data[*pos]
	(*pos)++

	switch code {
	case collectionWordCode:
		if *pos+2 > len(data) {
			return 0, errTruncated
		}
		value := binary.BigEndian.Uint16(data[*pos : *pos+2])
		*pos += 2
		return value, nil
	case collectionOneByteCode1:
		if *pos >= len(data) {
			return 0, errTruncated
		}
		value := collectionSmallBase + uint16(data[*pos])
		(*pos)++
		return value, nil
	case collectionOneByteCode2:
		if *pos >= len(data) {
			return 0, errTruncated
		}
		value := collectionMediumBase + uint16(data[*pos])
		(*pos)++
		return value, nil
	default:
		return uint16(code), nil
	}
}

func rebuildCollectionMember(
	flavor []byte, indices []uint16, chunks []tableChunk, decompressed []byte,
) ([]byte, error) {
	selected := make([]tableChunk, 0, len(indices))
	for _, idx := range indices {
		if int(idx) >= len(chunks) {
			return nil, errTruncated
		}
		selected = append(selected, chunks[idx])
	}
	sort.Slice(selected, func(i, j int) bool {
		return bytes.Compare(selected[i].tag[:], selected[j].tag[:]) < 0
	})
	return rebuildSFNT(flavor, selected, decompressed)
}
