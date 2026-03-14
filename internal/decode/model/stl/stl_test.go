package stl_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/stl"
	"github.com/gophics/ravenporter/ir"
)

const stlHeaderSize = 80

func buildBinarySTL(triangles [][4][3]float32) []byte {
	var buf bytes.Buffer
	buf.Write(make([]byte, stlHeaderSize))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(triangles)))
	for _, tri := range triangles {
		for _, vec := range tri {
			for _, f := range vec {
				_ = binary.Write(&buf, binary.LittleEndian, f)
			}
		}
		_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	}
	return buf.Bytes()
}

func TestSTLDecodeAll(t *testing.T) {
	ascii := `solid TestSolid
  facet normal 0 0 1
    outer loop
      vertex 0 0 0
      vertex 1 0 0
      vertex 0 1 0
    endloop
  endfacet
  facet normal 0 0 -1
    outer loop
      vertex 0 0 0
      vertex 0 1 0
      vertex 1 0 0
    endloop
  endfacet
endsolid TestSolid
`
	asciiLIM := `solid LIM
  facet normal 0 0 1
    outer loop
      vertex 0 0 0
      vertex 1 0 0
      vertex 0 1 0
    endloop
  endfacet
endsolid LIM
`

	binColors := func() []byte {
		var buf bytes.Buffer
		buf.Write(make([]byte, stlHeaderSize))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(1))
		for range 3 {
			_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		}
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(1))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(1))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, uint16(0x8000|31<<10))
		return buf.Bytes()
	}()

	magicsColor := func() []byte {
		header := make([]byte, stlHeaderSize)
		copy(header[10:], "COLOR=")
		header[16] = 255
		header[17] = 0
		header[18] = 0
		header[19] = 255
		var buf bytes.Buffer
		buf.Write(header)
		_ = binary.Write(&buf, binary.LittleEndian, uint32(1))
		for range 12 {
			_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		}
		_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
		return buf.Bytes()
	}()

	tooMany := func() []byte {
		var buf bytes.Buffer
		buf.Write(make([]byte, stlHeaderSize))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(99_000_000))
		return buf.Bytes()
	}()

	dataSingle := buildBinarySTL([][4][3]float32{{{0, 0, 1}, {0, 0, 0}, {1, 0, 0}, {0, 1, 0}}})
	dataTrunc := dataSingle[:len(dataSingle)-10]

	tests := []struct {
		name          string
		input         []byte
		opts          detect.DecodeOptions
		wantErr       bool
		errorContains string
		check         func(t *testing.T, scene *ir.Asset)
	}{
		{"Single Triangle", dataSingle, detect.DecodeOptions{}, false, "", func(t *testing.T, scene *ir.Asset) {
			assert.Equal(t, ir.FormatSTL, scene.Metadata.SourceFormat)
			require.Len(t, scene.Meshes, 1)
			require.Len(t, scene.Meshes[0].Primitives, 1)
			p := &scene.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, p.Mode)
			assert.Equal(t, ir.NoIndex, p.MaterialIndex)
			assert.Equal(t, 3, p.Data.VertexCount)
			assert.Len(t, p.Data.Positions, 3)
			assert.Len(t, p.Data.Normals, 3)
			assert.Len(t, p.Data.Indices, 3)
			assert.Equal(t, [3]float32{0, 0, 0}, p.Data.Positions[0])
			for _, n := range p.Data.Normals {
				assert.InDelta(t, float32(1), n[2], 0.001)
			}
		}},
		{"Multiple Triangles", buildBinarySTL([][4][3]float32{{{0, 0, 1}, {0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, {{0, 0, -1}, {0, 0, 0}, {0, 1, 0}, {1, 0, 0}}}), detect.DecodeOptions{}, false, "", func(t *testing.T, scene *ir.Asset) {
			p := &scene.Meshes[0].Primitives[0]
			assert.Equal(t, 6, p.Data.VertexCount)
		}},
		{"Zero Triangles", buildBinarySTL(nil), detect.DecodeOptions{}, true, "", nil},
		{"Vertex Limit", buildBinarySTL([][4][3]float32{{{0, 0, 1}, {0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, {{0, 0, -1}, {0, 0, 0}, {0, 1, 0}, {1, 0, 0}}}), detect.DecodeOptions{MaxVertices: 3}, true, "vertex limit exceeded", nil},
		{"Truncated", dataTrunc, detect.DecodeOptions{}, true, "", nil},
		{"NaN Normal", buildBinarySTL([][4][3]float32{{{float32(math.NaN()), 0, 0}, {0, 0, 0}, {1, 0, 0}, {0, 1, 0}}}), detect.DecodeOptions{}, false, "", func(t *testing.T, scene *ir.Asset) {
			assert.Equal(t, 3, scene.Meshes[0].Primitives[0].Data.VertexCount)
		}},
		{"ASCII", []byte(ascii), detect.DecodeOptions{}, false, "", func(t *testing.T, scene *ir.Asset) {
			require.Len(t, scene.Meshes, 1)
			mesh := scene.Meshes[0]
			assert.Equal(t, "TestSolid", mesh.Name)
			assert.Equal(t, 6, mesh.Primitives[0].Data.VertexCount)
		}},
		{"ASCII Vertex Limit", []byte(asciiLIM), detect.DecodeOptions{MaxVertices: 2}, true, "vertex limit exceeded", nil},
		{"ASCII Empty", []byte("solid Empty\nendsolid Empty\n"), detect.DecodeOptions{}, true, "", nil},
		{"Binary Colors", binColors, detect.DecodeOptions{}, false, "", func(t *testing.T, scene *ir.Asset) {
			p := &scene.Meshes[0].Primitives[0]
			require.NotNil(t, p.Data.Colors0)
			assert.InDelta(t, float32(1), p.Data.Colors0[0][0], 0.02)
		}},
		{"Magics Color Header", magicsColor, detect.DecodeOptions{}, false, "", func(t *testing.T, scene *ir.Asset) {
			p := &scene.Meshes[0].Primitives[0]
			require.NotNil(t, p.Data.Colors0)
			assert.InDelta(t, float32(1), p.Data.Colors0[0][0], 0.02)
		}},
		{"Too Many Triangles", tooMany, detect.DecodeOptions{}, true, "", nil},
	}

	dec := &stl.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := dec.Decode(bytes.NewReader(tt.input), tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, scene)
				if tt.check != nil {
					tt.check(t, scene)
				}
			}
		})
	}
}

func TestProbe(t *testing.T) {
	data := buildBinarySTL([][4][3]float32{
		{{0, 0, 1}, {0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
	})
	dec := &stl.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(data)))
}

func TestProbeEmpty(t *testing.T) {
	dec := &stl.Decoder{}
	assert.False(t, dec.Probe(bytes.NewReader(nil)))
}

func TestExtensions(t *testing.T) {
	dec := &stl.Decoder{}
	assert.Equal(t, []string{".stl"}, dec.Extensions())
}

func TestFormatName(t *testing.T) {
	dec := &stl.Decoder{}
	assert.Equal(t, "STL", dec.FormatName())
}

func TestDecodeRealCube(t *testing.T) {
	data, err := os.ReadFile("testdata/cube.stl")
	require.NoError(t, err)

	dec := &stl.Decoder{}
	require.True(t, dec.Probe(bytes.NewReader(data)), "cube.stl should probe as STL")

	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, ir.FormatSTL, scene.Metadata.SourceFormat)
	require.Len(t, scene.Meshes, 1)
	require.Len(t, scene.Meshes[0].Primitives, 1)

	prim := scene.Meshes[0].Primitives[0]
	assert.Equal(t, 36, prim.Data.VertexCount)
	assert.Len(t, prim.Data.Positions, 36)
	assert.Len(t, prim.Data.Normals, 36)
	assert.Len(t, prim.Data.Indices, 36)

	for i, n := range prim.Data.Normals {
		length := n[0]*n[0] + n[1]*n[1] + n[2]*n[2]
		assert.InDelta(t, 1.0, length, 0.01, "normal[%d] should be unit", i)
	}

	t.Logf("cube.stl: %d vertices, %d normals, %d indices",
		prim.Data.VertexCount, len(prim.Data.Normals), len(prim.Data.Indices))
}
