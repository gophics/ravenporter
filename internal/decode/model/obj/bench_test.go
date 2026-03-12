package obj_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/obj"
)

func buildBenchOBJ(vertCount, faceCount int) string {
	var b strings.Builder
	for i := range vertCount {
		fmt.Fprintf(&b, "v %f %f %f\n", float32(i), float32(i+1), float32(i+2))
	}
	for i := range faceCount {
		v := i*3 + 1 // 1-based
		fmt.Fprintf(&b, "f %d %d %d\n", v, v+1, v+2)
	}
	return b.String()
}

func BenchmarkDecode100(b *testing.B) {
	benchmarkOBJDecode(b, 300, 100)
}

func BenchmarkDecode10K(b *testing.B) {
	benchmarkOBJDecode(b, 30_000, 10_000)
}

func BenchmarkProbe(b *testing.B) {
	dec := &obj.Decoder{}
	data := "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"
	b.ReportAllocs()
	for b.Loop() {
		dec.Probe(strings.NewReader(data))
	}
}

func BenchmarkDecode100K(b *testing.B) {
	benchmarkOBJDecode(b, 300_000, 100_000)
}

func benchmarkOBJDecode(b *testing.B, verts, faces int) {
	b.Helper()
	data := buildBenchOBJ(verts, faces)
	dec := &obj.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecodeFreeformCurve(b *testing.B) {
	src := "v 0 0 0\nv 0.5 1 0\nv 1 1.5 0\nv 1.5 1.5 0\nv 2 1 0\nv 2.5 0.5 0\n" +
		"v 3 0 0\nv 3.5 -0.5 0\nv 4 -1 0\nv 4.5 -0.5 0\nv 5 0 0\nv 5.5 0.5 0\nv 6 0 0\n" +
		"cstype bezier\ndeg 3\ncurv 0.0 1.0 1 2 3 4 5 6 7 8 9 10 11 12 13\n" +
		"parm u 0.0 0.25 0.5 0.75 1.0\nend\n"
	dec := &obj.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(src), detect.DecodeOptions{})
	}
}

func BenchmarkDecodeFreeformSurface(b *testing.B) {
	src := "v 0 0 0\nv 1 0 0\nv 2 0 0\nv 3 0 0\n" +
		"v 0 0 1\nv 1 1 1\nv 2 1 1\nv 3 0 1\n" +
		"v 0 0 2\nv 1 1 2\nv 2 1 2\nv 3 0 2\n" +
		"v 0 0 3\nv 1 0 3\nv 2 0 3\nv 3 0 3\n" +
		"cstype bezier\ndeg 3 3\nsurf 0.0 1.0 0.0 1.0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16\n" +
		"parm u 0.0 1.0\nparm v 0.0 1.0\nend\n"
	dec := &obj.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(src), detect.DecodeOptions{})
	}
}
