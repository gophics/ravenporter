package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	draco "github.com/gophics/go-draco"
	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/detect"
	gltfdec "github.com/gophics/ravenporter/internal/decode/model/gltf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReadRoundTripFromDracoGLTF(t *testing.T) {
	encoded := encodeCacheTestDracoTriangle(t)
	src := fmt.Sprintf(`{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_draco_mesh_compression"],
  "buffers": [{"uri": "mesh.drc", "byteLength": %d}],
  "bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": %d}],
  "accessors": [
    {"componentType": 5126, "count": 3, "type": "VEC3"},
    {"componentType": 5125, "count": 3, "type": "SCALAR"}
  ],
  "meshes": [{
    "primitives": [{
      "attributes": {"POSITION": 0},
      "indices": 1,
      "extensions": {
        "KHR_draco_mesh_compression": {
          "bufferView": 0,
          "attributes": {"POSITION": 7}
        }
      }
    }]
  }],
  "nodes": [{"mesh": 0}],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`, len(encoded), len(encoded))

	asset, err := (&gltfdec.Decoder{}).Decode(bytes.NewReader([]byte(src)), detect.DecodeOptions{
		FS: cacheTestFS{"mesh.drc": encoded},
	})
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, &ravenporter.Result{Asset: asset}))

	cooked, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	defer func() { require.NoError(t, cooked.Close()) }()

	require.Len(t, cooked.Meshes, 1)
	require.Len(t, cooked.Meshes[0].Primitives, 1)
	prim := cooked.Meshes[0].Primitives[0]
	assert.Equal(t, asset.Meshes[0].Primitives[0].Data.Positions, prim.Data.Positions)
	assert.Equal(t, []uint32{0, 1, 2}, prim.Data.Indices)
}

type cacheTestFS map[string][]byte

func (m cacheTestFS) Open(name string) (io.ReadCloser, error) {
	data, ok := m[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func encodeCacheTestDracoTriangle(tb testing.TB) []byte {
	tb.Helper()

	position, err := draco.NewFloat32Attribute(draco.AttributePosition, 3, []float32{
		0, 0, 0,
		1, 0, 0,
		0, 1, 0,
	})
	require.NoError(tb, err)
	position.UniqueID = 7

	mesh, err := draco.NewMesh(3, []draco.Face{{0, 1, 2}}, position)
	require.NoError(tb, err)
	data, err := draco.Encode(context.Background(), mesh, draco.WithMeshMethod(draco.MeshSequentialEncoding))
	require.NoError(tb, err)
	return data
}
