package fbx

import (
	"bytes"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

// BenchmarkDecodeFBX_Profile is designed for pprof profiling:
//
//	go test -bench=BenchmarkDecodeFBX_Profile -cpuprofile=cpu.prof -memprofile=mem.prof ./decode/fbx/
func BenchmarkDecodeFBX_Profile(b *testing.B) {
	data := buildMinimalFBX()
	dec := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}
