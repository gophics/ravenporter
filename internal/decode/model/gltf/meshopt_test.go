package gltf

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fastjson"

	"github.com/gophics/ravenporter/detect"
)

var meshoptAttributesV0Encoded = []byte{
	0xa0, 0x01, 0x3f, 0x00, 0x00, 0x00, 0x58, 0x57, 0x58, 0x01, 0x26, 0x00, 0x00, 0x00, 0x01, 0x0c, 0x00, 0x00, 0x00, 0x58, 0x01, 0x08, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x3f, 0x00, 0x00, 0x00, 0x17, 0x18, 0x17, 0x01, 0x26, 0x00, 0x00, 0x00, 0x01, 0x0c, 0x00, 0x00,
	0x00, 0x17, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

var meshoptAttributesV0Decoded = []byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 1, 0, 0, 0, 0, 0, 0, 244, 1, 0, 0,
	0, 0, 44, 1, 0, 0, 0, 0, 0, 0, 244, 1, 44, 1, 44, 1, 0, 0, 0, 0, 244, 1, 244, 1,
}

var meshoptAttributesMode2Encoded = []byte{
	0xa0, 0x02, 0x08, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x02, 0x0a, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0x02, 0x0c, 0xcc, 0xcc,
	0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0x02, 0x0e, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

var meshoptAttributesMode2Decoded = []byte{
	0, 0, 0, 0, 4, 5, 6, 7, 8, 10, 12, 14, 12, 15, 18, 21, 16, 20, 24, 28, 20, 25, 30, 35, 24, 30, 36, 42, 28, 35, 42, 49, 32, 40, 48, 56,
	36, 45, 54, 63, 40, 50, 60, 70, 44, 55, 66, 77, 48, 60, 72, 84, 52, 65, 78, 91, 56, 70, 84, 98, 60, 75, 90, 105,
}

var meshoptAttributesV1DeltasEncoded = []byte{
	0xa1, 0x99, 0x99, 0x01, 0x2a, 0xaa, 0xaa, 0xaa, 0x02, 0x04, 0x44, 0x44, 0x44, 0x43, 0x33, 0x33, 0x33, 0x02, 0x06, 0x66, 0x66, 0x66, 0x66,
	0x66, 0x66, 0x66, 0x02, 0x08, 0x88, 0x88, 0x88, 0x87, 0x77, 0x77, 0x77, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0xf8, 0x00, 0xf8, 0x00, 0xf0, 0x00, 0xf0, 0x00, 0x01, 0x01,
}

var meshoptTrianglesV0Encoded = []byte{
	0xe0, 0xf0, 0x10, 0xfe, 0xff, 0xf0, 0x0c, 0xff, 0x02, 0x02, 0x02, 0x00, 0x76, 0x87, 0x56, 0x67, 0x78, 0xa9, 0x86, 0x65, 0x89, 0x68, 0x98,
	0x01, 0x69, 0x00, 0x00,
}

var meshoptTrianglesV1Encoded = []byte{
	0xe1, 0xf0, 0x10, 0xfe, 0x1f, 0x3d, 0x00, 0x0a, 0x00, 0x76, 0x87, 0x56, 0x67, 0x78, 0xa9, 0x86, 0x65, 0x89, 0x68, 0x98, 0x01, 0x69, 0x00,
	0x00,
}

var meshoptSequenceEncoded = []byte{0xd1, 0x00, 0x04, 0xcd, 0x01, 0x04, 0x07, 0x98, 0x1f, 0x00, 0x00, 0x00, 0x00}

var meshoptFilterOctEncoded = []byte{
	0xa0, 0x01, 0x07, 0x00, 0x00, 0x00, 0x1e, 0x01, 0x3f, 0x00, 0x00, 0x00, 0x8b, 0x8c, 0xfd, 0x00, 0x01, 0x26, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x01, 0x7f, 0x00,
}

var meshoptFilterOctDecoded = []byte{0, 1, 127, 0, 0, 159, 82, 1, 255, 1, 127, 0, 1, 130, 241, 1}

var meshoptFilterQuatEncoded = []byte{
	0xa0, 0x01, 0x0f, 0x00, 0x00, 0x00, 0x3d, 0x5a, 0x01, 0x0f, 0x00, 0x00, 0x00, 0x0e, 0x0d, 0x01, 0x3f, 0x00, 0x00, 0x00, 0x9a, 0x99, 0x26,
	0x01, 0x3f, 0x00, 0x00, 0x00, 0x0e, 0x0d, 0x0a, 0x00, 0x00, 0x01, 0x2a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
	0xfc, 0x07,
}

var meshoptFilterExpEncoded = []byte{
	0xa0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0xff, 0xf7, 0xff, 0xff, 0x02, 0xff,
	0xff, 0x7f, 0xfe,
}

func TestDecodeMeshoptBuffer(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		filter     string
		count      int
		byteStride int
		encoded    []byte
		expected   []byte
	}{
		{
			name:       "AttributesV0",
			mode:       meshoptModeAttributes,
			filter:     meshoptFilterNone,
			count:      4,
			byteStride: 12,
			encoded:    meshoptAttributesV0Encoded,
			expected:   meshoptAttributesV0Decoded,
		},
		{
			name:       "AttributesMode2",
			mode:       meshoptModeAttributes,
			filter:     meshoptFilterNone,
			count:      16,
			byteStride: 4,
			encoded:    meshoptAttributesMode2Encoded,
			expected:   meshoptAttributesMode2Decoded,
		},
		{
			name:       "AttributesV1Deltas",
			mode:       meshoptModeAttributes,
			filter:     meshoptFilterNone,
			count:      16,
			byteStride: 8,
			encoded:    meshoptAttributesV1DeltasEncoded,
			expected: u16Bytes([]uint16{
				248, 248, 240, 240, 249, 250, 243, 244, 250, 252, 246, 248, 251, 254, 249, 252,
				252, 256, 252, 256, 253, 258, 255, 260, 254, 260, 258, 264, 255, 262, 261, 268,
				256, 264, 264, 272, 257, 262, 267, 268, 258, 260, 270, 264, 259, 258, 273, 260,
				260, 256, 276, 256, 261, 254, 279, 252, 262, 252, 282, 248, 263, 250, 285, 244,
			}),
		},
		{
			name:       "TrianglesV0",
			mode:       meshoptModeTriangles,
			filter:     meshoptFilterNone,
			count:      12,
			byteStride: 4,
			encoded:    meshoptTrianglesV0Encoded,
			expected:   u32Bytes([]uint32{0, 1, 2, 2, 1, 3, 4, 6, 5, 7, 8, 9}),
		},
		{
			name:       "TrianglesV1",
			mode:       meshoptModeTriangles,
			filter:     meshoptFilterNone,
			count:      15,
			byteStride: 4,
			encoded:    meshoptTrianglesV1Encoded,
			expected:   u32Bytes([]uint32{0, 1, 2, 2, 1, 3, 0, 1, 2, 2, 1, 5, 2, 1, 4}),
		},
		{
			name:       "IndexSequence",
			mode:       meshoptModeIndices,
			filter:     meshoptFilterNone,
			count:      6,
			byteStride: 4,
			encoded:    meshoptSequenceEncoded,
			expected:   u32Bytes([]uint32{0, 1, 51, 2, 49, 1000}),
		},
		{
			name:       "FilterOctahedral",
			mode:       meshoptModeAttributes,
			filter:     meshoptFilterOctahedral,
			count:      4,
			byteStride: 4,
			encoded:    meshoptFilterOctEncoded,
			expected:   meshoptFilterOctDecoded,
		},
		{
			name:       "FilterQuaternion",
			mode:       meshoptModeAttributes,
			filter:     meshoptFilterQuaternion,
			count:      4,
			byteStride: 8,
			encoded:    meshoptFilterQuatEncoded,
			expected: u16Bytes([]uint16{
				32767, 0, 11, 0, 0, 25013, 0, 21166, 11, 0, 23504, 22830, 158, 14715, 0, 29277,
			}),
		},
		{
			name:       "FilterExponential",
			mode:       meshoptModeAttributes,
			filter:     meshoptFilterExponential,
			count:      1,
			byteStride: 16,
			encoded:    meshoptFilterExpEncoded,
			expected:   u32Bytes([]uint32{0, 0x3fc00000, 0xc2100000, 0x49fffffe}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]byte, len(tt.expected))
			err := decodeMeshoptBuffer(result, tt.encoded, tt.count, tt.byteStride, tt.mode, tt.filter)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDocResolvesMeshoptBufferViews(t *testing.T) {
	src := fmt.Sprintf(`{
		"asset": {"version": "2.0"},
		"extensionsRequired": ["EXT_meshopt_compression"],
		"buffers": [{"byteLength": %d}, {"byteLength": %d}],
		"bufferViews": [{
			"buffer": 1,
			"byteLength": %d,
			"extensions": {
				"EXT_meshopt_compression": {
					"buffer": 0,
					"byteOffset": 0,
					"byteLength": %d,
					"byteStride": 12,
					"count": 4,
					"mode": "ATTRIBUTES"
				}
			}
		}]
	}`, len(meshoptAttributesV0Encoded), len(meshoptAttributesV0Decoded), len(meshoptAttributesV0Decoded), len(meshoptAttributesV0Encoded))

	d, err := parseDoc(&fastjson.Parser{}, []byte(src), meshoptAttributesV0Encoded, detect.DecodeOptions{})
	require.NoError(t, err)
	require.NoError(t, d.checkRequiredExtensions())

	view := d.bufs.views[0]
	require.Nil(t, view.meshopt)
	require.Equal(t, 12, view.byteStride)
	require.Equal(t, len(meshoptAttributesV0Decoded), view.byteLength)

	data, err := d.bufs.slice(view.buffer, view.byteOffset, view.byteLength)
	require.NoError(t, err)
	assert.Equal(t, meshoptAttributesV0Decoded, data)
}

func TestParseDocRejectsInvalidMeshoptDataWithoutRequiredExtensions(t *testing.T) {
	src := fmt.Sprintf(`{
		"asset": {"version": "2.0"},
		"buffers": [{"byteLength": %d}, {"byteLength": %d}],
		"bufferViews": [{
			"buffer": 1,
			"byteLength": %d,
			"extensions": {
				"EXT_meshopt_compression": {
					"buffer": 0,
					"byteOffset": 0,
					"byteLength": %d,
					"byteStride": 12,
					"count": 4,
					"mode": "ATTRIBUTES"
				}
			}
		}]
	}`, len(meshoptAttributesV0Encoded)-1, len(meshoptAttributesV0Decoded), len(meshoptAttributesV0Decoded), len(meshoptAttributesV0Encoded)-1)

	_, err := parseDoc(&fastjson.Parser{}, []byte(src), meshoptAttributesV0Encoded[:len(meshoptAttributesV0Encoded)-1], detect.DecodeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), extMeshopt)
}

func TestValidateMeshoptBufferViewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		view bufferView
	}{
		{
			name: "InvalidAttributeStride",
			view: bufferView{
				byteLength: 12,
				meshopt: &meshoptBufferView{
					sourceBuffer: 0,
					sourceLength: len(meshoptAttributesV0Encoded),
					byteStride:   6,
					count:        2,
					mode:         meshoptModeAttributes,
					filter:       meshoptFilterNone,
				},
			},
		},
		{
			name: "InvalidTriangleFilter",
			view: bufferView{
				byteLength: 48,
				meshopt: &meshoptBufferView{
					sourceBuffer: 0,
					sourceLength: len(meshoptTrianglesV0Encoded),
					byteStride:   4,
					count:        12,
					mode:         meshoptModeTriangles,
					filter:       meshoptFilterOctahedral,
				},
			},
		},
		{
			name: "InvalidIndexStride",
			view: bufferView{
				byteLength: 18,
				meshopt: &meshoptBufferView{
					sourceBuffer: 0,
					sourceLength: len(meshoptSequenceEncoded),
					byteStride:   3,
					count:        6,
					mode:         meshoptModeIndices,
					filter:       meshoptFilterNone,
				},
			},
		},
		{
			name: "LengthMismatch",
			view: bufferView{
				byteLength: 16,
				meshopt: &meshoptBufferView{
					sourceBuffer: 0,
					sourceLength: len(meshoptAttributesV0Encoded),
					byteStride:   12,
					count:        4,
					mode:         meshoptModeAttributes,
					filter:       meshoptFilterNone,
				},
			},
		},
		{
			name: "UnsupportedFilter",
			view: bufferView{
				byteLength: 48,
				meshopt: &meshoptBufferView{
					sourceBuffer: 0,
					sourceLength: len(meshoptAttributesV0Encoded),
					byteStride:   12,
					count:        4,
					mode:         meshoptModeAttributes,
					filter:       "COLOR",
				},
			},
		},
		{
			name: "UnsupportedMode",
			view: bufferView{
				byteLength: 48,
				meshopt: &meshoptBufferView{
					sourceBuffer: 0,
					sourceLength: len(meshoptAttributesV0Encoded),
					byteStride:   12,
					count:        4,
					mode:         "MORPHS",
					filter:       meshoptFilterNone,
				},
			},
		},
		{
			name: "InvalidQuaternionStride",
			view: bufferView{
				byteLength: 16,
				meshopt: &meshoptBufferView{
					sourceBuffer: 0,
					sourceLength: len(meshoptFilterQuatEncoded),
					byteStride:   4,
					count:        4,
					mode:         meshoptModeAttributes,
					filter:       meshoptFilterQuaternion,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMeshoptBufferView(0, tt.view)
			require.Error(t, err)
			assert.Contains(t, err.Error(), extMeshopt)
		})
	}
}

func TestDecodeMeshoptBufferRejectsMalformedStreams(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		filter     string
		count      int
		byteStride int
		encoded    []byte
	}{
		{
			name:       "InvalidAttributesHeader",
			mode:       meshoptModeAttributes,
			filter:     meshoptFilterNone,
			count:      4,
			byteStride: 12,
			encoded:    append([]byte{0x00}, meshoptAttributesV0Encoded[1:]...),
		},
		{
			name:       "InvalidTrianglesHeader",
			mode:       meshoptModeTriangles,
			filter:     meshoptFilterNone,
			count:      12,
			byteStride: 4,
			encoded:    append([]byte{0x00}, meshoptTrianglesV0Encoded[1:]...),
		},
		{
			name:       "InvalidSequenceHeader",
			mode:       meshoptModeIndices,
			filter:     meshoptFilterNone,
			count:      6,
			byteStride: 4,
			encoded:    append([]byte{0x00}, meshoptSequenceEncoded[1:]...),
		},
		{
			name:       "MalformedSequenceVarint",
			mode:       meshoptModeIndices,
			filter:     meshoptFilterNone,
			count:      1,
			byteStride: 4,
			encoded:    []byte{0xd1, 0x80, 0x80, 0x80, 0x80, 0x80},
		},
		{
			name:       "TruncatedTrianglePayload",
			mode:       meshoptModeTriangles,
			filter:     meshoptFilterNone,
			count:      12,
			byteStride: 4,
			encoded:    meshoptTrianglesV0Encoded[:len(meshoptTrianglesV0Encoded)-1],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]byte, tt.count*tt.byteStride)
			err := decodeMeshoptBuffer(result, tt.encoded, tt.count, tt.byteStride, tt.mode, tt.filter)
			require.Error(t, err)
		})
	}
}

func TestParseDocRejectsMeshoptSourceRangeOverflow(t *testing.T) {
	src := `{
		"asset": {"version": "2.0"},
		"buffers": [{"byteLength": 8}],
		"bufferViews": [{
			"buffer": 0,
			"byteLength": 48,
			"extensions": {
				"EXT_meshopt_compression": {
					"buffer": 0,
					"byteOffset": 0,
					"byteLength": 64,
					"byteStride": 4,
					"count": 12,
					"mode": "TRIANGLES"
				}
			}
		}]
	}`

	_, err := parseDoc(&fastjson.Parser{}, []byte(src), meshoptTrianglesV0Encoded[:8], detect.DecodeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), extMeshopt)
}

func TestDecodeMeshoptBufferRejectsUnsupportedMode(t *testing.T) {
	result := make([]byte, len(meshoptAttributesV0Decoded))
	err := decodeMeshoptBuffer(result, meshoptAttributesV0Encoded, 4, 12, "MORPHS", meshoptFilterNone)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported meshopt mode")
}

func BenchmarkDecodeMeshoptAttributes(b *testing.B) {
	result := make([]byte, len(meshoptAttributesV0Decoded))
	b.ReportAllocs()
	b.SetBytes(int64(len(result)))
	for b.Loop() {
		if err := decodeMeshoptBuffer(result, meshoptAttributesV0Encoded, 4, 12, meshoptModeAttributes, meshoptFilterNone); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMeshoptTriangles(b *testing.B) {
	result := make([]byte, 12*4)
	b.ReportAllocs()
	b.SetBytes(int64(len(result)))
	for b.Loop() {
		if err := decodeMeshoptBuffer(result, meshoptTrianglesV0Encoded, 12, 4, meshoptModeTriangles, meshoptFilterNone); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMeshoptIndices(b *testing.B) {
	result := make([]byte, 6*4)
	b.ReportAllocs()
	b.SetBytes(int64(len(result)))
	for b.Loop() {
		if err := decodeMeshoptBuffer(result, meshoptSequenceEncoded, 6, 4, meshoptModeIndices, meshoptFilterNone); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMeshoptFilteredAttributes(b *testing.B) {
	result := make([]byte, 4*8)
	b.ReportAllocs()
	b.SetBytes(int64(len(result)))
	for b.Loop() {
		if err := decodeMeshoptBuffer(result, meshoptFilterQuatEncoded, 4, 8, meshoptModeAttributes, meshoptFilterQuaternion); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMeshoptScene(b *testing.B) {
	jsonData := []byte(meshoptSceneJSON(meshoptSceneBufferByteLength()))
	binData := meshoptSceneBuffer()
	b.ReportAllocs()
	b.SetBytes(int64(len(jsonData) + len(binData)))
	for b.Loop() {
		d, err := parseDoc(&fastjson.Parser{}, jsonData, binData, detect.DecodeOptions{})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := d.convertDoc(); err != nil {
			b.Fatal(err)
		}
	}
}

func meshoptSceneJSON(bufferLength int) string {
	return fmt.Sprintf(`{
		"asset": {"version": "2.0"},
		"extensionsUsed": ["EXT_meshopt_compression"],
		"buffers": [{"byteLength": %d}],
		"bufferViews": [
			{"buffer": 0, "byteOffset": 0, "byteLength": 120},
			{
				"buffer": 0,
				"byteOffset": 120,
				"byteLength": 48,
				"extensions": {
					"EXT_meshopt_compression": {
						"buffer": 0,
						"byteOffset": 120,
						"byteLength": 27,
						"byteStride": 4,
						"count": 12,
						"mode": "TRIANGLES"
					}
				}
			}
		],
		"accessors": [
			{"bufferView": 0, "componentType": 5126, "count": 10, "type": "VEC3"},
			{"bufferView": 1, "componentType": 5125, "count": 12, "type": "SCALAR"}
		],
		"meshes": [{"primitives": [{"attributes": {"POSITION": 0}, "indices": 1}]}],
		"nodes": [{"mesh": 0}],
		"scenes": [{"nodes": [0]}],
		"scene": 0
	}`, bufferLength)
}

func meshoptSceneBuffer() []byte {
	result := make([]byte, meshoptSceneBufferByteLength())
	for i := 0; i < 10; i++ {
		offset := i * 12
		binary.LittleEndian.PutUint32(result[offset+0:], math32bits(float32(i)))
		binary.LittleEndian.PutUint32(result[offset+4:], math32bits(float32(i%3)))
		binary.LittleEndian.PutUint32(result[offset+8:], 0)
	}
	copy(result[120:], meshoptTrianglesV0Encoded)
	return result
}

func meshoptSceneBufferByteLength() int {
	return 120 + len(meshoptTrianglesV0Encoded)
}

func math32bits(value float32) uint32 {
	return math.Float32bits(value)
}

func u16Bytes(values []uint16) []byte {
	result := make([]byte, len(values)*2)
	for i, value := range values {
		binary.LittleEndian.PutUint16(result[i*2:], value)
	}
	return result
}

func u32Bytes(values []uint32) []byte {
	result := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(result[i*4:], value)
	}
	return result
}
