package pipeline

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"testing"

	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/process"
)

const benchSTLHeaderSize = 80

func buildBenchSTL(triCount int) []byte {
	var buf bytes.Buffer
	buf.Write(make([]byte, benchSTLHeaderSize))
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

func BenchmarkPipelineSTL100(b *testing.B) {
	benchPipeline(b, 100)
}

func BenchmarkPipelineSTL10K(b *testing.B) {
	benchPipeline(b, 10_000)
}

func benchPipeline(b *testing.B, triCount int) {
	b.Helper()
	data := buildBenchSTL(triCount)
	benchData(b, data, "bench.stl")
}

func BenchmarkPipelineOBJ(b *testing.B) {
	benchFile(b, "../decode/model/obj/testdata/multi_object.obj", "bench.obj")
}

func BenchmarkPipelineDAE(b *testing.B) {
	benchFile(b, "../decode/model/dae/testdata/animated_cube.dae", "bench.dae")
}

func BenchmarkPipelineGLB(b *testing.B) {
	benchFile(b, "../decode/model/gltf/testdata/Box.glb", "bench.glb")
}

func BenchmarkPipelineFBX(b *testing.B) {
	benchFile(b, "../decode/model/fbx/testdata/box.fbx", "bench.fbx")
}

func BenchmarkPipelineBVH(b *testing.B) {
	benchFile(b, "../decode/model/bvh/testdata/simple.bvh", "bench.bvh")
}

func benchFile(b *testing.B, path, filename string) {
	b.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skipf("testdata not available: %v", err)
	}
	benchData(b, data, filename)
}

func benchData(b *testing.B, data []byte, filename string) {
	b.Helper()
	reg := decode.NewRegistry()

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = importReader(context.Background(), bytes.NewReader(data), filename, config{
			Registry:     reg,
			ProcessFlags: process.PresetFast,
		})
	}
}
