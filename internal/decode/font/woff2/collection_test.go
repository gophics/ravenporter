package woff2

import (
	"bytes"
	"encoding/binary"
	_ "embed"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/fntutil"
	"github.com/gophics/ravenporter/ir"
)

//go:embed testdata/minimal.woff2
var minimalCollectionWOFF2 []byte

func TestDecodeCollection(t *testing.T) {
	fonts, err := decompressWOFF2(minimalCollectionWOFF2)
	require.NoError(t, err)
	require.Len(t, fonts, 1)

	collection := buildWOFF2Collection(t, fonts[0], 2)
	scene, err := (&Decoder{}).Decode(bytes.NewReader(collection), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Fonts, 2)
	assert.Equal(t, "TestFont #0", scene.Fonts[0].Name)
	assert.Equal(t, "TestFont #1", scene.Fonts[1].Name)
	assert.Equal(t, ir.FontWOFF2, scene.Fonts[0].Format)
}

type sfntTable struct {
	tag  [4]byte
	data []byte
}

func buildWOFF2Collection(t *testing.T, font []byte, count int) []byte {
	t.Helper()

	flavor, tables := parseSFNTTables(t, font)
	var tableDir bytes.Buffer
	var tableData bytes.Buffer

	for _, table := range tables {
		tagIndex := knownTagIndex(table.tag)
		if tagIndex >= 0 {
			tableDir.WriteByte(byte(tagIndex))
		} else {
			tableDir.WriteByte(flagKnownTag)
			tableDir.Write(table.tag[:])
		}
		tableDir.Write(encodeBase128(uint32(len(table.data))))
		tableData.Write(table.data)
	}

	var collectionDir bytes.Buffer
	require.NoError(t, binary.Write(&collectionDir, binary.BigEndian, uint32(0x00010000)))
	collectionDir.Write(encode255UShort(count))
	for range count {
		collectionDir.Write(encode255UShort(len(tables)))
		collectionDir.Write(flavor[:])
		for i := range tables {
			collectionDir.Write(encode255UShort(i))
		}
	}

	var compressed bytes.Buffer
	bw := brotli.NewWriter(&compressed)
	_, err := bw.Write(tableData.Bytes())
	require.NoError(t, err)
	require.NoError(t, bw.Close())

	header := make([]byte, woff2HeaderSize)
	copy(header[:4], magic)
	copy(header[4:8], woff2CollectionFlavor)
	binary.BigEndian.PutUint16(header[woff2NumTablesOff:woff2NumTablesOff+2], uint16(len(tables)))
	binary.BigEndian.PutUint32(header[woff2SfntSizeOff:woff2SfntSizeOff+4], uint32(len(font)*count))
	binary.BigEndian.PutUint32(header[woff2CompressedOff:woff2CompressedOff+4], uint32(compressed.Len()))

	data := header
	data = append(data, tableDir.Bytes()...)
	data = append(data, collectionDir.Bytes()...)
	data = append(data, compressed.Bytes()...)
	binary.BigEndian.PutUint32(data[8:12], uint32(len(data)))

	return data
}

func parseSFNTTables(t *testing.T, font []byte) ([4]byte, []sfntTable) {
	t.Helper()

	require.GreaterOrEqual(t, len(font), fntutil.SFNTHeaderSize)
	var flavor [4]byte
	copy(flavor[:], font[:4])
	numTables := int(binary.BigEndian.Uint16(font[4:6]))
	require.Greater(t, numTables, 0)

	tables := make([]sfntTable, 0, numTables)
	for i := range numTables {
		entryOff := fntutil.SFNTHeaderSize + i*fntutil.SFNTTableEntSize
		require.LessOrEqual(t, entryOff+fntutil.SFNTTableEntSize, len(font))

		var tag [4]byte
		copy(tag[:], font[entryOff:entryOff+4])
		offset := int(binary.BigEndian.Uint32(font[entryOff+8 : entryOff+12]))
		length := int(binary.BigEndian.Uint32(font[entryOff+12 : entryOff+16]))
		require.LessOrEqual(t, offset+length, len(font))
		tables = append(tables, sfntTable{tag: tag, data: append([]byte(nil), font[offset:offset+length]...)})
	}

	return flavor, tables
}

func knownTagIndex(tag [4]byte) int {
	for i, known := range knownTags[:] {
		if string(tag[:]) == known {
			return i
		}
	}
	return -1
}

func encodeBase128(v uint32) []byte {
	if v == 0 {
		return []byte{0}
	}
	var out [5]byte
	pos := len(out)
	for v > 0 {
		pos--
		out[pos] = byte(v & 0x7F)
		if pos != len(out)-1 {
			out[pos] |= 0x80
		}
		v >>= 7
	}
	return append([]byte(nil), out[pos:]...)
}

func encode255UShort(v int) []byte {
	smallBase := int(collectionSmallBase)
	mediumBase := int(collectionMediumBase)
	switch {
	case v < smallBase:
		return []byte{byte(v)}
	case v < mediumBase:
		return []byte{collectionOneByteCode1, byte(v - smallBase)}
	case v < mediumBase+0x100:
		return []byte{collectionOneByteCode2, byte(v - mediumBase)}
	default:
		return []byte{collectionWordCode, byte(v >> 8), byte(v)}
	}
}
