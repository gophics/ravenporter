package ply_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/ply"
)

func buildASCIIPLY(vertCount, faceCount int) string {
	var b strings.Builder
	b.WriteString("ply\n")
	b.WriteString("format ascii 1.0\n")
	fmt.Fprintf(&b, "element vertex %d\n", vertCount)
	b.WriteString("property float x\nproperty float y\nproperty float z\n")
	fmt.Fprintf(&b, "element face %d\n", faceCount)
	b.WriteString("property list uchar int vertex_indices\n")
	b.WriteString("end_header\n")

	for i := range vertCount {
		fmt.Fprintf(&b, "%f %f %f\n", float32(i), float32(i+1), float32(i+2))
	}
	for i := range faceCount {
		v := i * 3
		fmt.Fprintf(&b, "3 %d %d %d\n", v, v+1, v+2)
	}
	return b.String()
}

func buildBinaryPLY(vertCount, faceCount int) []byte {
	header := fmt.Sprintf("ply\nformat binary_little_endian 1.0\nelement vertex %d\nproperty float x\nproperty float y\nproperty float z\nelement face %d\nproperty list uchar int vertex_indices\nend_header\n", vertCount, faceCount)

	var buf bytes.Buffer
	buf.WriteString(header)

	for i := range vertCount {
		_ = binary.Write(&buf, binary.LittleEndian, float32(i))
		_ = binary.Write(&buf, binary.LittleEndian, float32(i+1))
		_ = binary.Write(&buf, binary.LittleEndian, float32(i+2))
	}
	for i := range faceCount {
		buf.WriteByte(3) // face vertex count
		v := int32(i * 3)
		_ = binary.Write(&buf, binary.LittleEndian, v)
		_ = binary.Write(&buf, binary.LittleEndian, v+1)
		_ = binary.Write(&buf, binary.LittleEndian, v+2)
	}
	return buf.Bytes()
}

func BenchmarkDecodeASCII100(b *testing.B) {
	benchmarkDecodeASCII(b, 300, 100)
}

func BenchmarkDecodeASCII10K(b *testing.B) {
	benchmarkDecodeASCII(b, 30_000, 10_000)
}

func BenchmarkDecodeBinaryLE100(b *testing.B) {
	benchmarkDecodeBinaryLE(b, 300, 100)
}

func BenchmarkDecodeBinaryLE10K(b *testing.B) {
	benchmarkDecodeBinaryLE(b, 30_000, 10_000)
}

func BenchmarkProbe(b *testing.B) {
	data := buildBinaryPLY(3, 1)
	dec := &ply.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		dec.Probe(bytes.NewReader(data))
	}
}

func benchmarkDecodeASCII(b *testing.B, verts, faces int) {
	b.Helper()
	data := buildASCIIPLY(verts, faces)
	dec := &ply.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(data), detect.DecodeOptions{})
	}
}

func benchmarkDecodeBinaryLE(b *testing.B, verts, faces int) {
	b.Helper()
	data := buildBinaryPLY(verts, faces)
	dec := &ply.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}
