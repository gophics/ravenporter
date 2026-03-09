package woff_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/font/woff"
	"github.com/gophics/ravenporter/internal/testutil"
	"github.com/gophics/ravenporter/ir"
)

//go:embed testdata/minimal.woff
var minimalWOFF []byte

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid WOFF", minimalWOFF, true},
		{"TTF magic", []byte{0x00, 0x01, 0x00, 0x00}, false},
		{"junk", []byte("not a font"), false},
		{"empty", nil, false},
	}
	dec := &woff.Decoder{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dec.Probe(bytes.NewReader(tc.data)))
		})
	}
}

func TestDecode(t *testing.T) {
	dec := &woff.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(minimalWOFF), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Fonts, 1)

	f := scene.Fonts[0]
	assert.Equal(t, ir.FontWOFF, f.Format)
	assert.Equal(t, "TestFont", f.Family)
	assert.Equal(t, "TestFont", f.Name)
	assert.Equal(t, ir.FormatWOFF, scene.Metadata.SourceFormat)
	require.NotNil(t, f.Vector)
	assert.Equal(t, 800, f.Vector.Ascender)
	assert.Equal(t, -200, f.Vector.Descender)
	assert.Equal(t, 90, f.Vector.LineGap)
}

func TestDecodeRejectsOversizedInputBeforeRead(t *testing.T) {
	src := testutil.NewOversizeReadSeeker(8)
	_, err := (&woff.Decoder{}).Decode(src, detect.DecodeOptions{MaxFileSize: 4})
	require.Error(t, err)
	assert.ErrorContains(t, err, "size")
	assert.Zero(t, src.Reads)
}

func TestExtensions(t *testing.T) {
	dec := &woff.Decoder{}
	assert.Equal(t, []string{".woff"}, dec.Extensions())
}

func TestFormatName(t *testing.T) {
	dec := &woff.Decoder{}
	assert.Equal(t, "WOFF", dec.FormatName())
}

func BenchmarkDecode(b *testing.B) {
	dec := &woff.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(minimalWOFF), opts)
	}
}
