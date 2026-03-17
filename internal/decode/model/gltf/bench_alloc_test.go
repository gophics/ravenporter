package gltf_test

import (
	"strings"
	"testing"

	"github.com/gophics/ravenporter/detect"
	gltfdec "github.com/gophics/ravenporter/internal/decode/model/gltf"
)

func BenchmarkDecodeGLTF_Allocs(b *testing.B) {
	const json = `{
  "asset": {"version": "2.0"},
  "scene": 0,
  "scenes": [{"nodes": [0]}],
  "nodes": [{"mesh": 0, "name": "AllocTest"}],
  "meshes": [{"primitives": [{"attributes": {"POSITION": 0}}]}],
  "accessors": [{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3", "max": [1,1,1], "min": [0,0,0]}],
  "bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": 36}],
  "buffers": [{"byteLength": 36}]
}`
	dec := &gltfdec.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(json)))
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(json), detect.DecodeOptions{})
	}
}
