package woff2

import (
	"encoding/binary"
	"errors"
	"io"

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
	woff2NumTablesOff  = 12
	woff2SfntSizeOff   = 16
	woff2CompressedOff = 20

	tableAlignMask = 3

	flagTransformMask = 0x3F
	flagKnownTag      = 0x3F
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

	sfnt, err := decompressWOFF2(raw)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatWOFF2, "decompress", err)
	}

	f := &ir.Font{
		Name:   decodedName,
		Format: ir.FontWOFF2,
		Vector: &ir.VectorFontData{RawData: sfnt},
	}
	fntutil.ParseSFNTMetrics(sfnt, f)

	return &ir.Asset{
		Fonts:    []*ir.Font{f},
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatWOFF2},
	}, nil
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }

func decompressWOFF2(data []byte) ([]byte, error) {
	if len(data) < woff2HeaderSize {
		return nil, errTruncated
	}

	numTables := int(binread.ReadU16BE(data[woff2NumTablesOff:]))
	totalSfntSize := int(binread.ReadU32BE(data[woff2SfntSizeOff:]))
	totalCompressed := int(binread.ReadU32BE(data[woff2CompressedOff:]))

	var stackEntries [64]tableEntry
	entries := stackEntries[:0]
	if numTables > len(stackEntries) {
		entries = make([]tableEntry, 0, numTables)
	}

	pos := woff2HeaderSize
	for range numTables {
		if pos >= len(data) {
			return nil, errTruncated
		}
		flagByte := data[pos]
		pos++

		var e tableEntry
		e.transform = (flagByte >> 6) & 0x03 //nolint:mnd // spec bits
		tagIdx := flagByte & flagTransformMask

		if tagIdx == flagKnownTag {
			if pos+4 > len(data) {
				return nil, errTruncated
			}
			copy(e.tag[:], data[pos:pos+4])
			pos += 4
		} else if int(tagIdx) < len(knownTags) {
			copy(e.tag[:], knownTags[tagIdx])
		} else {
			return nil, errTruncated
		}

		origLen, n := fntutil.ReadBase128(data[pos:])
		if n == 0 {
			return nil, errTruncated
		}
		pos += n
		e.origLen = origLen

		if e.transform != 0 {
			transLen, tn := fntutil.ReadBase128(data[pos:])
			if tn == 0 {
				return nil, errTruncated
			}
			pos += tn
			e.transLen = transLen
		} else {
			e.transLen = origLen
		}

		entries = append(entries, e)
	}

	compStart := pos
	compEnd := compStart + totalCompressed
	if compEnd > len(data) {
		return nil, errTruncated
	}

	decompressed, err := pool.BrotliDecodeSized(data[compStart:compEnd], totalSfntSize)
	if err != nil {
		return nil, err
	}

	return rebuildSFNT(data[4:8], entries, decompressed)
}

func rebuildSFNT(flavor []byte, entries []tableEntry, decompressed []byte) ([]byte, error) {
	n := len(entries)

	sfntSize := fntutil.SFNTHeaderSize + n*fntutil.SFNTTableEntSize
	for _, e := range entries {
		sfntSize += (int(e.origLen) + tableAlignMask) &^ tableAlignMask
	}

	sfnt := make([]byte, sfntSize)
	copy(sfnt[:4], flavor)
	binary.BigEndian.PutUint16(sfnt[4:6], uint16(n)) //nolint:gosec // bounded by u16 source

	sr, es, rs := fntutil.CalcSFNTSearchParams(n)
	binary.BigEndian.PutUint16(sfnt[6:8], sr)
	binary.BigEndian.PutUint16(sfnt[8:10], es)
	binary.BigEndian.PutUint16(sfnt[10:12], rs)

	dataOff := fntutil.SFNTHeaderSize + n*fntutil.SFNTTableEntSize
	srcOff := 0
	for i, e := range entries {
		entOff := fntutil.SFNTHeaderSize + i*fntutil.SFNTTableEntSize
		copy(sfnt[entOff:], e.tag[:])
		binary.BigEndian.PutUint32(sfnt[entOff+8:], uint32(dataOff)) //nolint:gosec // positive int
		binary.BigEndian.PutUint32(sfnt[entOff+12:], e.origLen)

		tblLen := int(e.transLen)
		if srcOff+tblLen > len(decompressed) {
			tblLen = len(decompressed) - srcOff
			if tblLen < 0 {
				tblLen = 0
			}
		}
		if tblLen > 0 {
			copy(sfnt[dataOff:], decompressed[srcOff:srcOff+tblLen])
		}
		srcOff += tblLen

		dataOff += (int(e.origLen) + tableAlignMask) &^ tableAlignMask
	}

	return sfnt, nil
}
