package ttf_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/internal/decode/font/ttf"
	"github.com/gophics/ravenporter/internal/testutil"
	"github.com/gophics/ravenporter/ir"
)

var (
	//go:embed testdata/minimal.ttf
	minimalTTF []byte

	//go:embed testdata/Roboto-Regular.ttf
	robotoTTF []byte
)

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid TTF", minimalTTF, true},
		{"OTF magic", []byte("OTTO\x00\x0A"), false},
		{"junk", []byte("not a font"), false},
		{"empty", nil, false},
	}
	dec := &ttf.Decoder{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dec.Probe(bytes.NewReader(tc.data)))
		})
	}
}

func TestDecode(t *testing.T) {
	dec, ok := decode.DefaultRegistry().Lookup(ir.FormatTTF)
	require.True(t, ok)

	scene, err := dec.Decode(bytes.NewReader(minimalTTF), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Fonts, 1)

	f := scene.Fonts[0]
	assert.Equal(t, ir.FontTTF, f.Format)
	assert.Equal(t, "TestFont", f.Family)
	assert.Equal(t, "TestFont", f.Name)
	require.NotNil(t, f.Vector)
	assert.Equal(t, 800, f.Vector.Ascender)
	assert.Equal(t, -200, f.Vector.Descender)
	assert.Equal(t, 90, f.Vector.LineGap)
}

func TestDecodeRoboto(t *testing.T) {
	dec, ok := decode.DefaultRegistry().Lookup(ir.FormatTTF)
	require.True(t, ok)

	scene, err := dec.Decode(bytes.NewReader(robotoTTF), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Fonts, 1)

	f := scene.Fonts[0]
	assert.Equal(t, ir.FontTTF, f.Format)
	assert.Equal(t, "Roboto", f.Family)
	require.NotNil(t, f.Vector)
	assert.Equal(t, 2146, f.Vector.Ascender)
	assert.Equal(t, -555, f.Vector.Descender)
	assert.Equal(t, 2048, f.Vector.UnitsPerEm)
	assert.Greater(t, f.Vector.GlyphCount, 0)
	assert.NotEmpty(t, f.Metadata)
}

func TestDecodeRejectsOversizedInputBeforeRead(t *testing.T) {
	src := testutil.NewOversizeReadSeeker(8)
	_, err := (&ttf.Decoder{}).Decode(src, detect.DecodeOptions{MaxFileSize: 4})
	require.Error(t, err)
	assert.ErrorContains(t, err, "size")
	assert.Zero(t, src.Reads)
}

func TestExtensions(t *testing.T) {
	dec := &ttf.Decoder{}
	assert.Equal(t, []string{".ttf"}, dec.Extensions())
}

func TestFormatName(t *testing.T) {
	dec := &ttf.Decoder{}
	assert.Equal(t, "TTF", dec.FormatName())
}

func BenchmarkDecode(b *testing.B) {
	dec := &ttf.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(robotoTTF), opts)
	}
}
