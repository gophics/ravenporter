package dae_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/dae"
)

//go:embed testdata/triangle.dae
var triangleDAE []byte

func BenchmarkDecode(b *testing.B) {
	dec := &dae.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(triangleDAE)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(triangleDAE), detect.DecodeOptions{})
	}
}

func BenchmarkProbe(b *testing.B) {
	dec := &dae.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		dec.Probe(bytes.NewReader(triangleDAE))
	}
}
