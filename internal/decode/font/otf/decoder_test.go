package otf_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/internal/decode/font/otf"
	"github.com/gophics/ravenporter/internal/testutil"
	"github.com/gophics/ravenporter/ir"
)

//go:embed testdata/minimal.otf
var minimalOTF []byte

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid OTF", minimalOTF, true},
		{"TTF magic", []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x0A}, false},
		{"junk", []byte("not a font"), false},
		{"empty", nil, false},
	}
	dec := &otf.Decoder{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dec.Probe(bytes.NewReader(tc.data)))
		})
	}
}

func TestDecode(t *testing.T) {
	dec, ok := decode.DefaultRegistry().Lookup(ir.FormatOTF)
	require.True(t, ok)

	scene, err := dec.Decode(bytes.NewReader(minimalOTF), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Fonts, 1)

	f := scene.Fonts[0]
	assert.Equal(t, ir.FontOTF, f.Format)
	assert.Equal(t, ir.FormatOTF, scene.Metadata.SourceFormat)
}

func TestDecodeRejectsOversizedInputBeforeRead(t *testing.T) {
	src := testutil.NewOversizeReadSeeker(8)
	_, err := (&otf.Decoder{}).Decode(src, detect.DecodeOptions{MaxFileSize: 4})
	require.Error(t, err)
	assert.ErrorContains(t, err, "size")
	assert.Zero(t, src.Reads)
}

func BenchmarkDecode(b *testing.B) {
	dec := &otf.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(minimalOTF), opts)
	}
}
