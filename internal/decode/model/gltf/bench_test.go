package gltf_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gophics/ravenporter/detect"
	gltfdec "github.com/gophics/ravenporter/internal/decode/model/gltf"
)

const benchGLTF = `{
  "asset": {"version": "2.0"},
  "scene": 0,
  "scenes": [{"nodes": [0]}],
  "nodes": [{"mesh": 0, "name": "Bench"}],
  "meshes": [{"primitives": [{"attributes": {"POSITION": 0}, "indices": 1}]}],
  "accessors": [
    {"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3", "max": [1,1,0], "min": [0,0,0]},
    {"bufferView": 1, "componentType": 5123, "count": 3, "type": "SCALAR"}
  ],
  "bufferViews": [
    {"buffer": 0, "byteOffset": 0, "byteLength": 36},
    {"buffer": 0, "byteOffset": 36, "byteLength": 6}
  ],
  "buffers": [{"byteLength": 42}]
}`

func BenchmarkDecodeGLTF(b *testing.B) {
	dec := &gltfdec.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(benchGLTF)))
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(benchGLTF), detect.DecodeOptions{})
	}
}

func BenchmarkProbeGLTF(b *testing.B) {
	dec := &gltfdec.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		dec.Probe(strings.NewReader(benchGLTF))
	}
}

func BenchmarkDecodeGLTF_1K(b *testing.B) {
	const vertCount = 1000
	json := fmt.Sprintf(`{
  "asset": {"version": "2.0"},
  "scene": 0,
  "scenes": [{"nodes": [0]}],
  "nodes": [{"mesh": 0, "name": "Bench1K"}],
  "meshes": [{"primitives": [{"attributes": {"POSITION": 0}, "indices": 1}]}],
  "accessors": [
    {"bufferView": 0, "componentType": 5126, "count": %d, "type": "VEC3", "max": [1,1,1], "min": [0,0,0]},
    {"bufferView": 1, "componentType": 5123, "count": %d, "type": "SCALAR"}
  ],
  "bufferViews": [
    {"buffer": 0, "byteOffset": 0, "byteLength": %d},
    {"buffer": 0, "byteOffset": %d, "byteLength": %d}
  ],
  "buffers": [{"byteLength": %d}]
}`, vertCount, vertCount, vertCount*12, vertCount*12, vertCount*2, vertCount*12+vertCount*2)

	dec := &gltfdec.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(json)))
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(json), detect.DecodeOptions{})
	}
}
