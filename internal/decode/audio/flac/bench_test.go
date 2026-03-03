package flac

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func BenchmarkDecode_Metadata(b *testing.B) {
	data := buildFLACMetadata(44100, 2, 16, 44100)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecode_RealFile(b *testing.B) {
	data, err := os.ReadFile("../testdata/minimal.flac")
	if err != nil {
		b.Skip("testdata not available")
	}
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkParseMetadata(b *testing.B) {
	data := buildFLACMetadata(44100, 2, 16, 44100)
	b.ReportAllocs()
	for b.Loop() {
		parseMetadata(data)
	}
}

func BenchmarkBitReader_Read(b *testing.B) {
	data := make([]byte, 1024)
	b.ReportAllocs()
	for b.Loop() {
		br := newBitReader(data)
		for range 512 {
			br.read(16)
		}
	}
}

func BenchmarkBitReader_Unary(b *testing.B) {
	data := make([]byte, 128)
	for i := range data {
		data[i] = 0x55
	}
	b.ReportAllocs()
	for b.Loop() {
		br := newBitReader(data)
		for range 256 {
			br.readUnary()
		}
	}
}

func BenchmarkDecorrelate_MidSide(b *testing.B) {
	left := make([]int32, 4096)
	right := make([]int32, 4096)
	for i := range left {
		left[i] = int32(i * 100)
		right[i] = int32(i * 10)
	}
	subs := [][]int32{left, right}
	b.ReportAllocs()
	for b.Loop() {
		decorrelate(subs, channelMidSide)
	}
}

func BenchmarkProbe(b *testing.B) {
	data := magic
	r := bytes.NewReader(data)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		r.Reset(data)
		d.Probe(r)
	}
}
