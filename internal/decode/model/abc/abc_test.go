package abc_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/abc"
	"github.com/gophics/ravenporter/ir"
)

var (
	//go:embed testdata/minimal.abc
	minimalABC []byte

	//go:embed testdata/cube.abc
	cubeABC []byte
)

func TestAlembicProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"ValidOgawa", minimalABC, true},
		{"InvalidMagic", []byte("not alembic"), false},
		{"Empty", []byte(""), false},
	}
	dec := &abc.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dec.Probe(bytes.NewReader(tt.data)))
		})
	}
}

func TestAlembicMeta(t *testing.T) {
	dec := &abc.Decoder{}
	assert.Equal(t, []string{".abc"}, dec.Extensions())
	assert.Equal(t, "Alembic", dec.FormatName())
}

func TestAlembicDecodeAll(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
		check   func(t *testing.T, sc *ir.Asset)
	}{
		{"Minimal ABC", minimalABC, false, func(t *testing.T, sc *ir.Asset) {
			assert.Equal(t, ir.FormatAlembic, sc.Metadata.SourceFormat)
		}},
		{"Cube with Mesh", cubeABC, false, func(t *testing.T, sc *ir.Asset) {
			assert.Equal(t, ir.FormatAlembic, sc.Metadata.SourceFormat)
			assert.NotEmpty(t, sc.Metadata.SourceVersion)
			if len(sc.Meshes) > 0 {
				assert.True(t, len(sc.Meshes[0].Primitives) > 0)
			}
		}},
		{"Rejects Junk", []byte{0x01, 0x02, 0x03}, true, nil},
	}

	dec := &abc.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := dec.Decode(bytes.NewReader(tt.input), detect.DecodeOptions{})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.check(t, sc)
			}
		})
	}
}

func TestDecodeAlembicTruncated(_ *testing.T) {
	dec := &abc.Decoder{}
	// Truncate the file at various byte boundaries to hit safety checks
	for i := len(cubeABC) - 1; i > 0; i -= 25 {
		_, _ = dec.Decode(bytes.NewReader(cubeABC[:i]), detect.DecodeOptions{})
	}
}

func BenchmarkDecodeAlembic(b *testing.B) {
	dec := &abc.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(cubeABC)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(cubeABC), detect.DecodeOptions{})
	}
}
