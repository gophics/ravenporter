package woff2_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/font/woff2"
	"github.com/gophics/ravenporter/internal/testutil"
	"github.com/gophics/ravenporter/ir"
)

//go:embed testdata/minimal.woff2
var minimalWOFF2 []byte

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid WOFF2", minimalWOFF2, true},
		{"WOFF1 magic", []byte("wOFF\x00\x01"), false},
		{"junk", []byte("not a font"), false},
		{"empty", nil, false},
	}
	dec := &woff2.Decoder{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dec.Probe(bytes.NewReader(tc.data)))
		})
	}
}

func TestDecode(t *testing.T) {
	dec := &woff2.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(minimalWOFF2), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Fonts, 1)

	f := scene.Fonts[0]
	assert.Equal(t, ir.FontWOFF2, f.Format)
	assert.Equal(t, "TestFont", f.Family)
	assert.Equal(t, "TestFont", f.Name)
	assert.Equal(t, ir.FormatWOFF2, scene.Metadata.SourceFormat)
	require.NotNil(t, f.Vector)
	assert.Equal(t, 800, f.Vector.Ascender)
	assert.Equal(t, -200, f.Vector.Descender)
	assert.Equal(t, 90, f.Vector.LineGap)
}

func TestDecodeRejectsOversizedInputBeforeRead(t *testing.T) {
	src := testutil.NewOversizeReadSeeker(8)
	_, err := (&woff2.Decoder{}).Decode(src, detect.DecodeOptions{MaxFileSize: 4})
	require.Error(t, err)
	assert.ErrorContains(t, err, "size")
	assert.Zero(t, src.Reads)
}

func TestExtensions(t *testing.T) {
	dec := &woff2.Decoder{}
	assert.Equal(t, []string{".woff2"}, dec.Extensions())
}

func TestFormatName(t *testing.T) {
	dec := &woff2.Decoder{}
	assert.Equal(t, "WOFF2", dec.FormatName())
}

func BenchmarkDecode(b *testing.B) {
	dec := &woff2.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(minimalWOFF2), opts)
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("wOF2")},
		{"truncated header", append([]byte("wOF2"), make([]byte, 10)...)},
		{"bad magic", append([]byte("XXXX"), make([]byte, 44)...)},
	}
	dec := &woff2.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			assert.Error(t, err)
		})
	}
}

func buildWOFF2Header(numTables uint16, totalSfntSize, totalCompressed uint32) []byte {
	hdr := make([]byte, 48)
	copy(hdr[:4], "wOF2")
	// flavor at 4..8
	hdr[4], hdr[5], hdr[6], hdr[7] = 0, 1, 0, 0
	// length at 8..12 - set to something large
	hdr[8], hdr[9], hdr[10], hdr[11] = 0, 0, 0xFF, 0xFF
	// numTables at 12..14 (big-endian)
	hdr[12] = byte(numTables >> 8)
	hdr[13] = byte(numTables)
	// reserved at 14..16
	// totalSfntSize at 16..20 (big-endian)
	hdr[16] = byte(totalSfntSize >> 24)
	hdr[17] = byte(totalSfntSize >> 16)
	hdr[18] = byte(totalSfntSize >> 8)
	hdr[19] = byte(totalSfntSize)
	// totalCompressedSize at 20..24 (big-endian)
	hdr[20] = byte(totalCompressed >> 24)
	hdr[21] = byte(totalCompressed >> 16)
	hdr[22] = byte(totalCompressed >> 8)
	hdr[23] = byte(totalCompressed)
	return hdr
}

func TestDecompressErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			"truncated table directory",
			buildWOFF2Header(5, 200, 10), // 5 tables but no table directory data
		},
		{
			"invalid tag index",
			func() []byte {
				hdr := buildWOFF2Header(1, 100, 10)
				// flagByte: transform=0 (bits 6-7), tagIdx=0x3E (62, > len(knownTags))
				hdr = append(hdr, 0x3E)
				return hdr
			}(),
		},
		{
			"truncated custom tag",
			func() []byte {
				hdr := buildWOFF2Header(1, 100, 10)
				// flagByte: transform=0, tagIdx=0x3F (means read 4 bytes for custom tag)
				hdr = append(hdr, 0x3F, 'c', 'm')
				return hdr
			}(),
		},
		{
			"truncated base128 origLen",
			func() []byte {
				hdr := buildWOFF2Header(1, 100, 10)
				// flagByte: transform=0, tagIdx=0 (known tag "cmap")
				hdr = append(hdr, 0x00)
				// No base128 data follows - truncated origLen
				return hdr
			}(),
		},
		{
			"truncated compressed data",
			func() []byte {
				hdr := buildWOFF2Header(1, 100, 999)
				// flagByte: transform=0, tagIdx=0
				hdr = append(hdr, 0x00, 100)
				// No compressed data follows but totalCompressed=999
				return hdr
			}(),
		},
		{
			"truncated transform length",
			func() []byte {
				hdr := buildWOFF2Header(1, 100, 10)
				// flagByte: transform=1 (bits 6-7 = 01), tagIdx=0
				hdr = append(hdr, 0x40, 100)
				// No transLen base128 follows - truncated
				return hdr
			}(),
		},
	}
	dec := &woff2.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			assert.Error(t, err)
		})
	}
}
