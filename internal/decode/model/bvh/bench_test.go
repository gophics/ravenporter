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

var scaleBVH = []byte("HIERARCHY\n" +
	"ROOT Root\n" +
	"{\n" +
	"OFFSET 0 0 0\n" +
	"CHANNELS 9 Xposition Yposition Zposition Xrotation Yrotation Zrotation Xscale Yscale Zscale\n" +
	"}\n" +
	"MOTION\n" +
	"Frames: 2\n" +
	"Frame Time: 0.0333333\n" +
	"0 0 0 0 0 0 1 1 1\n" +
	"0 0 0 0 0 0 2 3 4\n")

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

func BenchmarkDecodeScaleChannels(b *testing.B) {
	dec := &bvh.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(scaleBVH)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(scaleBVH), detect.DecodeOptions{})
	}
}
