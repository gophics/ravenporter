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
	assert.NotEmpty(t, scene.Meshes)
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
