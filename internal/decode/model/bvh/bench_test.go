package bvh_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/bvh"
)

//go:embed testdata/simple.bvh
var simpleBVH []byte

func BenchmarkDecode(b *testing.B) {
	dec := &bvh.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(simpleBVH)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(simpleBVH), detect.DecodeOptions{})
	}
}

func BenchmarkProbe(b *testing.B) {
	dec := &bvh.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		dec.Probe(bytes.NewReader(simpleBVH))
	}
}
