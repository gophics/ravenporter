package decoder

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	fbxdec "github.com/gophics/ravenporter/internal/decode/model/fbx"
	"github.com/gophics/ravenporter/ir"
)

func TestDecodeASCIIFBXRealFile(t *testing.T) {
	scene, err := (&fbxdec.Decoder{}).Decode(bytes.NewReader(sourceData(t, "FBX", "cubes_with_names.fbx")), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, ir.FormatFBX, scene.Metadata.SourceFormat)
	assert.Equal(t, "7500", scene.Metadata.SourceVersion)
	require.Len(t, scene.RootNodes, 2)
	assert.Equal(t, []int{0, 2}, scene.RootNodes)
	require.Len(t, scene.Nodes, 4)
	assert.Equal(t, "Cube2", scene.Nodes[0].Name)
	assert.Equal(t, "Cube3", scene.Nodes[2].Name)
	require.Len(t, scene.Meshes, 4)
	assert.Equal(t, 24, scene.Meshes[0].Primitives[0].Data.VertexCount)
	assert.Len(t, scene.Meshes[0].Primitives[0].Data.Indices, 36)
	assert.Equal(t, 768, scene.Meshes[3].Primitives[0].Data.VertexCount)
	assert.Len(t, scene.Meshes[3].Primitives[0].Data.Indices, 1152)
	require.Len(t, scene.Materials, 2)
	assert.Equal(t, "Mat_Green", scene.Materials[0].Name)
	assert.Equal(t, "Mat_Red", scene.Materials[1].Name)
	require.Len(t, scene.Animations, 1)
	assert.Equal(t, "Take 001", scene.Animations[0].Name)
}

func BenchmarkDecodeASCIIFBX(b *testing.B) {
	data, err := os.ReadFile(sourcePath("FBX", "cubes_with_names.fbx"))
	if err != nil {
		b.Skip("ASCII test file not found")
	}

	dec := &fbxdec.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}
