package stl_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/stl"
)

func buildBenchSTL(triCount int) []byte {
	var buf bytes.Buffer
	buf.Write(make([]byte, stlHeaderSize))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(triCount))
	for i := range triCount {
		fi := float32(i)
		for range 4 { // normal + 3 vertices
			_ = binary.Write(&buf, binary.LittleEndian, fi)
			_ = binary.Write(&buf, binary.LittleEndian, fi+1)
			_ = binary.Write(&buf, binary.LittleEndian, fi+2)
		}
		_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	}
	return buf.Bytes()
}

func BenchmarkDecode100(b *testing.B) {
	benchmarkSTLDecode(b, 100)
}

func BenchmarkDecode10K(b *testing.B) {
	benchmarkSTLDecode(b, 10_000)
}

func BenchmarkProbe(b *testing.B) {
	data := buildBenchSTL(1)
	dec := &stl.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		dec.Probe(bytes.NewReader(data))
	}
}

func benchmarkSTLDecode(b *testing.B, triCount int) {
	b.Helper()
	data := buildBenchSTL(triCount)
	dec := &stl.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}
