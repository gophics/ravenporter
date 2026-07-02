package gltf

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"

	draco "github.com/gophics/go-draco"
	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDracoPositionID = uint32(7)
	testDracoNormalID   = uint32(42)
	testDracoColorID    = uint32(77)
)

func TestDecodeDracoPrimitive(t *testing.T) {
	encoded := encodeDracoTriangle(t, true)
	reporter := &dracoTestReporter{}
	asset, err := (&Decoder{}).Decode(bytes.NewReader([]byte(dracoTriangleGLTF(len(encoded), "", `"POSITION": 7, "NORMAL": 42`))), detect.DecodeOptions{
		FS:       mapFS{"mesh.drc": encoded},
		Reporter: reporter,
	})
	require.NoError(t, err)

	require.Len(t, asset.Meshes, 1)
	require.Len(t, asset.Meshes[0].Primitives, 1)
	prim := asset.Meshes[0].Primitives[0]
	assert.Equal(t, ir.Triangles, prim.Mode)
	assert.Equal(t, 0, prim.MaterialIndex)
	assert.Equal(t, 3, prim.Data.VertexCount)
	assert.Equal(t, []uint32{0, 1, 2}, prim.Data.Indices)
	require.Len(t, prim.Data.Positions, 3)
	assert.InDelta(t, float32(1), prim.Data.Positions[1][0], 0.001)
	require.Len(t, prim.Data.Normals, 3)
	assert.Equal(t, [3]float32{0, 0, 1}, prim.Data.Normals[0])
	assert.Contains(t, reporter.notes[dracoDecodedNoteKey], dracoDecodedNoteValue)
}

func TestDecodeDracoPrimitiveTriangleStripNormalizesToTriangles(t *testing.T) {
	encoded := encodeDracoTriangle(t, false)
	asset, err := (&Decoder{}).Decode(bytes.NewReader([]byte(dracoTriangleGLTF(len(encoded), `, "mode": 5`, `"POSITION": 7`))), detect.DecodeOptions{
		FS: mapFS{"mesh.drc": encoded},
	})
	require.NoError(t, err)

	require.Len(t, asset.Meshes, 1)
	require.Len(t, asset.Meshes[0].Primitives, 1)
	prim := asset.Meshes[0].Primitives[0]
	assert.Equal(t, ir.Triangles, prim.Mode)
	assert.Equal(t, []uint32{0, 1, 2}, prim.Data.Indices)
}

func TestDecodeDracoPrimitiveTriangleStripAcceptsStripAccessorCount(t *testing.T) {
	encoded := encodeDracoGrid(t, 2)
	asset, err := (&Decoder{}).Decode(bytes.NewReader([]byte(dracoGridGLTF(len(encoded), 4, 4, `, "mode": 5`))), detect.DecodeOptions{
		FS: mapFS{"mesh.drc": encoded},
	})
	require.NoError(t, err)

	prim := asset.Meshes[0].Primitives[0]
	assert.Equal(t, ir.Triangles, prim.Mode)
	assert.Equal(t, []uint32{0, 1, 2, 1, 3, 2}, prim.Data.Indices)
}

func TestDecodeDracoPrimitiveReadsFallbackAttributes(t *testing.T) {
	encoded := encodeDracoTriangle(t, false)
	uv := packFloat32s([]float32{
		0, 0,
		1, 0,
		0, 1,
	})
	jsonData := fmt.Sprintf(`{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_draco_mesh_compression"],
  "buffers": [
    {"uri": "mesh.drc", "byteLength": %d},
    {"uri": "uv.bin", "byteLength": %d}
  ],
  "bufferViews": [
    {"buffer": 0, "byteOffset": 0, "byteLength": %d},
    {"buffer": 1, "byteOffset": 0, "byteLength": %d}
  ],
  "accessors": [
    {"componentType": 5126, "count": 3, "type": "VEC3"},
    {"componentType": 5125, "count": 3, "type": "SCALAR"},
    {"bufferView": 1, "componentType": 5126, "count": 3, "type": "VEC2"}
  ],
  "meshes": [{
    "primitives": [{
      "attributes": {"POSITION": 0, "TEXCOORD_0": 2},
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
}`, len(encoded), len(uv), len(encoded), len(uv))

	asset, err := (&Decoder{}).Decode(bytes.NewReader([]byte(jsonData)), detect.DecodeOptions{
		FS: mapFS{"mesh.drc": encoded, "uv.bin": uv},
	})
	require.NoError(t, err)

	prim := asset.Meshes[0].Primitives[0]
	require.Len(t, prim.Data.TexCoord0, 3)
	assert.Equal(t, [2]float32{1, 0}, prim.Data.TexCoord0[1])
}

func TestDecodeDracoPrimitiveReadsQuantizedColors(t *testing.T) {
	encoded := encodeDracoColoredTriangle(t)
	asset, err := (&Decoder{}).Decode(
		bytes.NewReader([]byte(dracoTriangleGLTF(len(encoded), "", `"POSITION": 7, "NORMAL": 42, "COLOR_0": 77`))),
		detect.DecodeOptions{FS: mapFS{"mesh.drc": encoded}},
	)
	require.NoError(t, err)

	prim := asset.Meshes[0].Primitives[0]
	require.Len(t, prim.Data.Positions, 3)
	require.Len(t, prim.Data.Normals, 3)
	require.Len(t, prim.Data.Colors0, 3)
	assert.InDelta(t, 1, prim.Data.Colors0[0][0], 0.01)
	assert.InDelta(t, 0, prim.Data.Colors0[0][1], 0.01)
	assert.InDelta(t, 0, prim.Data.Colors0[0][2], 0.01)
	assert.InDelta(t, 1, prim.Data.Colors0[0][3], 0.01)
	assert.InDelta(t, 0, prim.Data.Colors0[1][0], 0.01)
	assert.InDelta(t, 1, prim.Data.Colors0[1][1], 0.01)
	assert.InDelta(t, 0, prim.Data.Colors0[1][2], 0.01)
}

func TestDecodeDracoPrimitiveRejectsMalformedInputs(t *testing.T) {
	encoded := encodeDracoTriangle(t, false)
	tests := []struct {
		name    string
		json    string
		data    []byte
		opts    detect.DecodeOptions
		wantErr string
	}{
		{
			name:    "MissingBufferView",
			json:    dracoMalformedGLTF(len(encoded), `"attributes": {"POSITION": 7}`, ""),
			data:    encoded,
			wantErr: "missing bufferView",
		},
		{
			name:    "OutOfRangeBufferView",
			json:    dracoMalformedGLTF(len(encoded), `"bufferView": 9, "attributes": {"POSITION": 7}`, ""),
			data:    encoded,
			wantErr: "bufferView index 9 out of bounds",
		},
		{
			name:    "BadBufferViewType",
			json:    dracoMalformedGLTF(len(encoded), `"bufferView": "0", "attributes": {"POSITION": 7}`, ""),
			data:    encoded,
			wantErr: "bufferView must be a number",
		},
		{
			name:    "NegativeBufferView",
			json:    dracoMalformedGLTF(len(encoded), `"bufferView": -1, "attributes": {"POSITION": 7}`, ""),
			data:    encoded,
			wantErr: "bufferView must be non-negative",
		},
		{
			name:    "MissingPosition",
			json:    dracoMissingPositionGLTF(len(encoded)),
			data:    encoded,
			wantErr: "POSITION attribute is required",
		},
		{
			name:    "UnsupportedMode",
			json:    dracoMalformedGLTF(len(encoded), `"bufferView": 0, "attributes": {"POSITION": 7}`, `, "mode": 1`),
			data:    encoded,
			wantErr: "unsupported primitive mode 1",
		},
		{
			name:    "BadPayload",
			json:    dracoMalformedGLTF(3, `"bufferView": 0, "attributes": {"POSITION": 7}`, ""),
			data:    []byte{1, 2, 3},
			wantErr: "decode failed",
		},
		{
			name:    "VertexLimit",
			json:    dracoMalformedGLTF(len(encoded), `"bufferView": 0, "attributes": {"POSITION": 7}`, ""),
			data:    encoded,
			opts:    detect.DecodeOptions{MaxVertices: 2},
			wantErr: "vertex limit exceeded",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.FS = mapFS{"mesh.drc": tc.data}
			_, err := (&Decoder{}).Decode(bytes.NewReader([]byte(tc.json)), tc.opts)
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func BenchmarkDecodeGLTFDracoTiny(b *testing.B) {
	encoded := encodeDracoTriangle(b, false)
	jsonData := []byte(dracoTriangleGLTF(len(encoded), "", `"POSITION": 7`))
	dec := &Decoder{}
	opts := detect.DecodeOptions{FS: mapFS{"mesh.drc": encoded}}

	b.ReportAllocs()
	b.SetBytes(int64(len(jsonData) + len(encoded)))
	for b.Loop() {
		if _, err := dec.Decode(bytes.NewReader(jsonData), opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeGLTFDracoLarge(b *testing.B) {
	encoded := encodeDracoGrid(b, 32)
	jsonData := []byte(dracoGridGLTF(len(encoded), 32*32, (32-1)*(32-1)*6))
	dec := &Decoder{}
	opts := detect.DecodeOptions{FS: mapFS{"mesh.drc": encoded}}

	b.ReportAllocs()
	b.SetBytes(int64(len(jsonData) + len(encoded)))
	for b.Loop() {
		if _, err := dec.Decode(bytes.NewReader(jsonData), opts); err != nil {
			b.Fatal(err)
		}
	}
}

type dracoTestReporter struct {
	notes map[string][]string
}

func (r *dracoTestReporter) AddDependency(_, _, _, _ string) {}

func (r *dracoTestReporter) AddProvenanceNote(key, value string) {
	if r.notes == nil {
		r.notes = make(map[string][]string)
	}
	r.notes[key] = append(r.notes[key], value)
}

func encodeDracoTriangle(tb testing.TB, normals bool) []byte {
	tb.Helper()

	position, err := draco.NewFloat32Attribute(draco.AttributePosition, elemVec3, []float32{
		0, 0, 0,
		1, 0, 0,
		0, 1, 0,
	})
	require.NoError(tb, err)
	position.UniqueID = testDracoPositionID

	attrs := []*draco.Attribute{position}
	if normals {
		normal, err := draco.NewFloat32Attribute(draco.AttributeNormal, elemVec3, []float32{
			0, 0, 1,
			0, 0, 1,
			0, 0, 1,
		})
		require.NoError(tb, err)
		normal.UniqueID = testDracoNormalID
		attrs = append(attrs, normal)
	}

	mesh, err := draco.NewMesh(3, []draco.Face{{0, 1, 2}}, attrs...)
	require.NoError(tb, err)
	return encodeDracoMesh(tb, mesh)
}

func encodeDracoColoredTriangle(tb testing.TB) []byte {
	tb.Helper()

	position, err := draco.NewFloat32Attribute(draco.AttributePosition, elemVec3, []float32{
		0, 0, 0,
		1, 0, 0,
		0, 1, 0,
	})
	require.NoError(tb, err)
	position.UniqueID = testDracoPositionID

	normal, err := draco.NewFloat32Attribute(draco.AttributeNormal, elemVec3, []float32{
		0, 0, 1,
		0, 0, 1,
		0, 0, 1,
	})
	require.NoError(tb, err)
	normal.UniqueID = testDracoNormalID

	color, err := draco.NewFloat32Attribute(draco.AttributeColor, elemVec4, []float32{
		1, 0, 0, 1,
		0, 1, 0, 1,
		0, 0, 1, 1,
	})
	require.NoError(tb, err)
	color.UniqueID = testDracoColorID

	mesh, err := draco.NewMesh(3, []draco.Face{{0, 1, 2}}, position, normal, color)
	require.NoError(tb, err)
	return encodeDracoMesh(
		tb,
		mesh,
		draco.WithAttributeQuantization(draco.AttributePosition, 10),
		draco.WithAttributeQuantization(draco.AttributeNormal, 8),
		draco.WithAttributeQuantization(draco.AttributeColor, 8),
	)
}

func encodeDracoGrid(tb testing.TB, side int) []byte {
	tb.Helper()

	vertexCount := side * side
	positions := make([]float32, 0, vertexCount*elemVec3)
	for y := range side {
		for x := range side {
			positions = append(positions, float32(x), float32(y), 0)
		}
	}
	faces := make([]draco.Face, 0, (side-1)*(side-1)*2)
	for y := 0; y < side-1; y++ {
		for x := 0; x < side-1; x++ {
			i := uint32(y*side + x)
			faces = append(
				faces,
				draco.Face{i, i + 1, i + uint32(side)},
				draco.Face{i + 1, i + uint32(side) + 1, i + uint32(side)},
			)
		}
	}

	position, err := draco.NewFloat32Attribute(draco.AttributePosition, elemVec3, positions)
	require.NoError(tb, err)
	position.UniqueID = testDracoPositionID
	mesh, err := draco.NewMesh(vertexCount, faces, position)
	require.NoError(tb, err)
	return encodeDracoMesh(tb, mesh)
}

func encodeDracoMesh(tb testing.TB, mesh *draco.Mesh, opts ...draco.EncodeOption) []byte {
	tb.Helper()

	opts = append(opts, draco.WithMeshMethod(draco.MeshSequentialEncoding))
	data, err := draco.Encode(context.Background(), mesh, opts...)
	require.NoError(tb, err)
	return data
}

func dracoTriangleGLTF(byteLength int, mode, extAttrs string) string {
	normalAccessor := ""
	normalAttribute := ""
	colorAccessor := ""
	colorAttribute := ""
	nextAccessor := 2
	if strings.Contains(extAttrs, attrNormal) {
		normalAccessor = `,
    {"componentType": 5126, "count": 3, "type": "VEC3"}`
		normalAttribute = fmt.Sprintf(`, "NORMAL": %d`, nextAccessor)
		nextAccessor++
	}
	if strings.Contains(extAttrs, attrColor0) {
		colorAccessor = `,
    {"componentType": 5126, "count": 3, "type": "VEC4"}`
		colorAttribute = fmt.Sprintf(`, "COLOR_0": %d`, nextAccessor)
	}

	return fmt.Sprintf(`{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_draco_mesh_compression"],
  "extensionsRequired": ["KHR_draco_mesh_compression"],
  "buffers": [{"uri": "mesh.drc", "byteLength": %d}],
  "bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": %d}],
  "accessors": [
    {"componentType": 5126, "count": 3, "type": "VEC3"},
    {"componentType": 5125, "count": 3, "type": "SCALAR"}%s%s
  ],
  "materials": [{"name": "M"}],
  "meshes": [{
    "primitives": [{
      "attributes": {"POSITION": 0%s%s},
      "indices": 1,
      "material": 0%s,
      "extensions": {
        "KHR_draco_mesh_compression": {
          "bufferView": 0,
          "attributes": {%s}
        }
      }
    }]
  }],
  "nodes": [{"mesh": 0}],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`, byteLength, byteLength, normalAccessor, colorAccessor, normalAttribute, colorAttribute, mode, extAttrs)
}

func dracoMalformedGLTF(byteLength int, dracoExt, mode string) string {
	return fmt.Sprintf(`{
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
      "indices": 1%s,
      "extensions": {"KHR_draco_mesh_compression": {%s}}
    }]
  }],
  "nodes": [{"mesh": 0}],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`, byteLength, byteLength, mode, dracoExt)
}

func dracoMissingPositionGLTF(byteLength int) string {
	json := dracoMalformedGLTF(byteLength, `"bufferView": 0, "attributes": {}`, "")
	return strings.Replace(json, `"attributes": {"POSITION": 0}`, `"attributes": {}`, 1)
}

func dracoGridGLTF(byteLength, vertexCount, indexCount int, mode ...string) string {
	modeField := ""
	if len(mode) > 0 {
		modeField = mode[0]
	}
	return fmt.Sprintf(`{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_draco_mesh_compression"],
  "buffers": [{"uri": "mesh.drc", "byteLength": %d}],
  "bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": %d}],
  "accessors": [
    {"componentType": 5126, "count": %d, "type": "VEC3"},
    {"componentType": 5125, "count": %d, "type": "SCALAR"}
  ],
  "meshes": [{
    "primitives": [{
      "attributes": {"POSITION": 0},
      "indices": 1%s,
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
}`, byteLength, byteLength, vertexCount, indexCount, modeField)
}

func packFloat32s(values []float32) []byte {
	out := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(value))
	}
	return out
}
