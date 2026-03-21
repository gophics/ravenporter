package tds_test

import (
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/tds"
)

func BenchmarkDecode3DS(b *testing.B) {
	r := buildTestScene(testOpts{
		materials: []testMaterial{
			{name: "mat0", diffR: 200, diffG: 100, diffB: 50, texFile: "tex.png"},
		},
		objects: []testObject{{
			name:    "bench_mesh",
			kind:    kindMesh,
			verts:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}},
			faces:   [][3]uint16{{0, 1, 2}, {1, 3, 2}},
			uvs:     [][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			matName: "mat0",
		}},
	})
	data := make([]byte, r.Len())
	_, _ = r.Read(data)

	dec := &tds.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		r.Reset(data)
		_, _ = dec.Decode(r, detect.DecodeOptions{})
	}
}

func BenchmarkProbe3DS(b *testing.B) {
	r := buildTestScene(testOpts{
		objects: []testObject{{
			name:  "tri",
			kind:  "mesh",
			verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
			faces: [][3]uint16{{0, 1, 2}},
		}},
	})
	data := make([]byte, r.Len())
	_, _ = r.Read(data)

	dec := &tds.Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		r.Reset(data)
		dec.Probe(r)
	}
}
