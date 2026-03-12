package obj_test

import (
	"strings"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/obj"
)

// BenchmarkDecodeOBJ_Profile is designed for pprof profiling:
//
//	go test -bench=BenchmarkDecodeOBJ_Profile -cpuprofile=cpu.prof -memprofile=mem.prof ./decode/obj/
func BenchmarkDecodeOBJ_Profile(b *testing.B) {
	data := buildBenchOBJ(30_000, 10_000)
	dec := &obj.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(data), detect.DecodeOptions{})
	}
}
