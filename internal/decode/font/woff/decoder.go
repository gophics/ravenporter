package woff

import (
	"bytes"
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
	formatName  = "WOFF"
	decodedName = "WOFF Font"
	extWOFF     = ".woff"

	woffHeaderSize    = 44
	woffTableEntSize  = 20
	woffNumTablesOff  = 12
	woffTableDirStart = 44

	tableAlignMask = 3
)

var (
	magic           = []byte("wOFF")
	errTruncated    = errors.New("truncated WOFF data")
	errDecompress   = errors.New("zlib decompression failed")
	errInvalidMagic = errors.New("invalid WOFF signature")
	extensions      = []string{extWOFF}

	zlibPool pool.ZlibReader //nolint:gochecknoglobals // low-alloc reader reuse
)

// Decoder implements detect.Decoder for WOFF fonts.
type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatWOFF, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return decutil.ProbeBytes(r, magic) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatWOFF, "size", err)
	}
	raw, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatWOFF, "read", err)
	}

	sfnt, err := decompressWOFF(raw)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatWOFF, "decompress", err)
	}

	f := &ir.Font{
		Name:   decodedName,
		Format: ir.FontWOFF,
		Vector: &ir.VectorFontData{RawData: sfnt},
	}
	fntutil.ParseSFNTMetrics(sfnt, f)

	return &ir.Asset{
		Fonts:    []*ir.Font{f},
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatWOFF},
	}, nil
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }

func decompressWOFF(data []byte) ([]byte, error) {
	if len(data) < woffHeaderSize {
		return nil, errTruncated
	}
	if !bytes.Equal(data[:4], magic) {
		return nil, errInvalidMagic
	}

	numTables := int(binread.ReadU16BE(data[woffNumTablesOff : woffNumTablesOff+2]))
	dirEnd := woffTableDirStart + numTables*woffTableEntSize
	if dirEnd > len(data) {
		return nil, errTruncated
	}

	sfntSize := fntutil.SFNTHeaderSize + numTables*fntutil.SFNTTableEntSize
	for i := range numTables {
		off := woffTableDirStart + i*woffTableEntSize
		origLen := int(binread.ReadU32BE(data[off+12 : off+16]))
		sfntSize += (origLen + tableAlignMask) &^ tableAlignMask
	}

	sfnt := make([]byte, sfntSize)
	copy(sfnt[:4], data[4:8])
	binary.BigEndian.PutUint16(sfnt[4:6], uint16(numTables)) //nolint:gosec // bounded by u16 source

	searchRange, entrySelector, rangeShift := fntutil.CalcSFNTSearchParams(numTables)
	binary.BigEndian.PutUint16(sfnt[6:8], searchRange)
	binary.BigEndian.PutUint16(sfnt[8:10], entrySelector)
	binary.BigEndian.PutUint16(sfnt[10:12], rangeShift)

	dataOff := fntutil.SFNTHeaderSize + numTables*fntutil.SFNTTableEntSize
	for i := range numTables {
		woffOff := woffTableDirStart + i*woffTableEntSize
		tag := data[woffOff : woffOff+4]
		tblOff := int(binread.ReadU32BE(data[woffOff+4 : woffOff+8]))
		compLen := int(binread.ReadU32BE(data[woffOff+8 : woffOff+12]))
		origLen := int(binread.ReadU32BE(data[woffOff+12 : woffOff+16]))

		if tblOff+compLen > len(data) {
			return nil, errTruncated
		}

		sfntEntOff := fntutil.SFNTHeaderSize + i*fntutil.SFNTTableEntSize
		copy(sfnt[sfntEntOff:], tag)
		binary.BigEndian.PutUint32(sfnt[sfntEntOff+4:], binread.ReadU32BE(data[woffOff+16:woffOff+20]))
		binary.BigEndian.PutUint32(sfnt[sfntEntOff+8:], uint32(dataOff))  //nolint:gosec // positive int
		binary.BigEndian.PutUint32(sfnt[sfntEntOff+12:], uint32(origLen)) //nolint:gosec // positive int

		if compLen == origLen {
			copy(sfnt[dataOff:], data[tblOff:tblOff+origLen])
		} else {
			if err := zlibPool.DecompressInto(sfnt[dataOff:dataOff+origLen], data[tblOff:tblOff+compLen]); err != nil {
				return nil, errDecompress
			}
		}
		dataOff += (origLen + tableAlignMask) &^ tableAlignMask
	}

	return sfnt, nil
}
