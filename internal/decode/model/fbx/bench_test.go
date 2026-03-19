package fbx

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func BenchmarkDecodeBinary(b *testing.B) {
	data, err := os.ReadFile("testdata/box.fbx")
	if err != nil {
		b.Skip("testdata/box.fbx not found")
	}
	dec := &Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecodeBinaryCore(b *testing.B) {
	data, err := os.ReadFile("testdata/box.fbx")
	if err != nil {
		b.Skip("testdata/box.fbx not found")
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = decodeBinaryFBX(context.Background(), data)
	}
}

func BenchmarkDecodeASCII(b *testing.B) {
	data, err := os.ReadFile("testdata/ascii.fbx")
	if err != nil {
		b.Skip("ASCII test file not found")
	}
	dec := &Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecodeASCIICore(b *testing.B) {
	data, err := os.ReadFile("testdata/ascii.fbx")
	if err != nil {
		b.Skip("ASCII test file not found")
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = decodeASCIIFBX(context.Background(), data)
	}
}

func BenchmarkProbe(b *testing.B) {
	data := buildMinimalFBX()
	dec := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		dec.Probe(bytes.NewReader(data))
	}
}
