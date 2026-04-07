package ply_test

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/ply"
	"github.com/gophics/ravenporter/ir"
)

var (
	//go:embed testdata/ascii_triangle.ply
	asciiTriangle string

	//go:embed testdata/ascii_normals_colors.ply
	asciiNormalsColors string

	//go:embed testdata/ascii_quad.ply
	asciiQuad string

	//go:embed testdata/ascii_empty.ply
	asciiEmpty string

	//go:embed testdata/ascii_pointcloud.ply
	asciiPointCloud string

	//go:embed testdata/ascii_nan.ply
	asciiNaN string

	//go:embed testdata/ascii_edges.ply
	asciiEdges string
)

func TestDecodePLY(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		opts      detect.DecodeOptions
		binaryTst bool
		wantErr   bool
		check     func(t *testing.T, scene *ir.Asset)
	}{
		{"ASCII Triangle", asciiTriangle, detect.DecodeOptions{}, false, false, func(t *testing.T, s *ir.Asset) {
			assert.Equal(t, ir.FormatPLY, s.Metadata.SourceFormat)
			require.Len(t, s.Meshes, 1)
			p := &s.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, p.Mode)
			assert.Equal(t, 3, p.Data.VertexCount)
			assert.Equal(t, [3]float32{0, 0, 0}, p.Data.Positions[0])
			assert.Nil(t, p.Data.Normals)
		}},
		{"ASCII Normals Colors", asciiNormalsColors, detect.DecodeOptions{}, false, false, func(t *testing.T, s *ir.Asset) {
			p := &s.Meshes[0].Primitives[0]
			require.NotNil(t, p.Data.Normals)
			require.NotNil(t, p.Data.Colors0)
			assert.InDelta(t, float32(1), p.Data.Colors0[0][0], 0.01)
		}},
		{"ASCII Quad", asciiQuad, detect.DecodeOptions{}, false, false, func(t *testing.T, s *ir.Asset) {
			p := &s.Meshes[0].Primitives[0]
			assert.Equal(t, 4, p.Data.VertexCount)
			assert.Len(t, p.Data.Indices, 6)
		}},
		{"ASCII Empty (Zero Vertices)", asciiEmpty, detect.DecodeOptions{}, false, true, nil},
		{"Vertex Limit Exceeded", asciiTriangle, detect.DecodeOptions{MaxVertices: 2}, false, true, nil},
		{"ASCII Point Cloud", asciiPointCloud, detect.DecodeOptions{}, false, false, func(t *testing.T, s *ir.Asset) {
			p := &s.Meshes[0].Primitives[0]
			assert.Equal(t, 3, p.Data.VertexCount)
			assert.Nil(t, p.Data.Indices)
		}},
		{"ASCII NaN Handling", asciiNaN, detect.DecodeOptions{}, false, false, func(t *testing.T, s *ir.Asset) {
			assert.True(t, math.IsNaN(float64(s.Meshes[0].Primitives[0].Data.Positions[0][0])))
		}},
		{"ASCII Edges (Lines)", asciiEdges, detect.DecodeOptions{}, false, false, func(t *testing.T, s *ir.Asset) {
			require.Len(t, s.Meshes[0].Primitives, 2)
			assert.Equal(t, ir.Triangles, s.Meshes[0].Primitives[0].Mode)
			assert.Equal(t, ir.Lines, s.Meshes[0].Primitives[1].Mode)
			assert.Equal(t, []uint32{0, 1, 1, 2, 2, 3, 3, 0}, s.Meshes[0].Primitives[1].Data.Indices)
		}},
		{"Binary Little Endian", "", detect.DecodeOptions{}, true, false, func(t *testing.T, s *ir.Asset) {
			p := &s.Meshes[0].Primitives[0]
			assert.Equal(t, 3, p.Data.VertexCount)
			assert.Equal(t, [3]float32{1, 0, 0}, p.Data.Positions[1])
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &ply.Decoder{}
			var reader detect.ReadSeekerAt

			if tt.binaryTst {
				var buf bytes.Buffer
				buf.WriteString("ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nelement face 1\nproperty list uchar int vertex_indices\nend_header\n")
				verts := [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
				for _, v := range verts {
					for _, f := range v {
						_ = binary.Write(&buf, binary.LittleEndian, f)
					}
				}
				buf.WriteByte(3)
				for i := range 3 {
					_ = binary.Write(&buf, binary.LittleEndian, int32(i))
				}
				reader = bytes.NewReader(buf.Bytes())
			} else {
				reader = strings.NewReader(tt.input)
			}

			scene, err := dec.Decode(reader, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
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
	dec := &ply.Decoder{}
	assert.True(t, dec.Probe(strings.NewReader("ply\n")))
	assert.False(t, dec.Probe(strings.NewReader("not ply")))
	assert.False(t, dec.Probe(strings.NewReader("")))
}

func TestDecodeBinaryLEDouble(t *testing.T) {
	header := "ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty double x\nproperty double y\nproperty double z\nelement face 1\nproperty list uchar int vertex_indices\nend_header\n"

	var buf bytes.Buffer
	buf.WriteString(header)

	verts := [][3]float64{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	for _, v := range verts {
		for _, f := range v {
			_ = binary.Write(&buf, binary.LittleEndian, f)
		}
	}
	buf.WriteByte(3)
	for i := range 3 {
		_ = binary.Write(&buf, binary.LittleEndian, int32(i))
	}

	dec := &ply.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)

	p := &scene.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
	assert.Equal(t, [3]float32{1, 0, 0}, p.Data.Positions[1])
}

func TestDecodeBinaryLEWithShortIndices(t *testing.T) {
	header := "ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nelement face 1\nproperty list uchar short vertex_indices\nend_header\n"

	var buf bytes.Buffer
	buf.WriteString(header)

	verts := [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	for _, v := range verts {
		for _, f := range v {
			_ = binary.Write(&buf, binary.LittleEndian, f)
		}
	}
	buf.WriteByte(3)
	for i := range 3 {
		_ = binary.Write(&buf, binary.LittleEndian, int16(i))
	}

	dec := &ply.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	p := &scene.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
}

func TestDecodeInvalidFormat(t *testing.T) {
	dec := &ply.Decoder{}
	_, err := dec.Decode(strings.NewReader("ply\nformat unknown 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nend_header\n0 0 0\n1 0 0\n0 1 0\n"), detect.DecodeOptions{})
	assert.Error(t, err)
}

func TestDecodeBinaryLENormalsColors(t *testing.T) {
	header := "ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nproperty float nx\nproperty float ny\nproperty float nz\nproperty uchar red\nproperty uchar green\nproperty uchar blue\nproperty uchar alpha\nelement face 1\nproperty list uchar int vertex_indices\nend_header\n"

	var buf bytes.Buffer
	buf.WriteString(header)

	type vert struct {
		X, Y, Z    float32
		NX, NY, NZ float32
		R, G, B, A uint8
	}
	verts := []vert{
		{0, 0, 0, 0, 0, 1, 255, 0, 0, 255},
		{1, 0, 0, 0, 0, 1, 0, 255, 0, 255},
		{0, 1, 0, 0, 0, 1, 0, 0, 255, 255},
	}
	for _, v := range verts {
		_ = binary.Write(&buf, binary.LittleEndian, v)
	}
	buf.WriteByte(3)
	for i := range 3 {
		_ = binary.Write(&buf, binary.LittleEndian, int32(i))
	}

	dec := &ply.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)

	p := &scene.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
	require.NotNil(t, p.Data.Normals)
	assert.InDelta(t, float32(1), p.Data.Normals[0][2], 0.01)
	require.NotNil(t, p.Data.Colors0)
	assert.InDelta(t, float32(1), p.Data.Colors0[0][0], 0.01)
	assert.InDelta(t, float32(1), p.Data.Colors0[1][1], 0.01)
}

func TestDecodeBinaryLEEdges(t *testing.T) {
	header := "ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nelement edge 2\nproperty int vertex1\nproperty int vertex2\nend_header\n"

	var buf bytes.Buffer
	buf.WriteString(header)

	verts := [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	for _, v := range verts {
		for _, f := range v {
			_ = binary.Write(&buf, binary.LittleEndian, f)
		}
	}
	edges := [][2]int32{{0, 1}, {1, 2}}
	for _, e := range edges {
		_ = binary.Write(&buf, binary.LittleEndian, e[0])
		_ = binary.Write(&buf, binary.LittleEndian, e[1])
	}

	dec := &ply.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, scene.Meshes[0].Primitives, 2)
	edgePrim := &scene.Meshes[0].Primitives[1]
	assert.Equal(t, ir.Lines, edgePrim.Mode)
	assert.Equal(t, []uint32{0, 1, 1, 2}, edgePrim.Data.Indices)
}

func TestDecodeBinaryTruncated(t *testing.T) {
	header := "ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nend_header\n"

	var buf bytes.Buffer
	buf.WriteString(header)
	_ = binary.Write(&buf, binary.LittleEndian, float32(0))

	dec := &ply.Decoder{}
	_, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	assert.Error(t, err)
}

func BenchmarkDecode(b *testing.B) {
	dec := &ply.Decoder{}
	data := []byte(asciiTriangle)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(strings.NewReader(string(data)), detect.DecodeOptions{})
	}
}

func TestDecodeBinaryLEShortCoords(t *testing.T) {
	header := "ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty short x\nproperty short y\nproperty short z\nproperty ushort s\nproperty ushort t\nelement face 1\nproperty list uchar uint vertex_indices\nend_header\n"

	var buf bytes.Buffer
	buf.WriteString(header)

	type vert struct {
		X, Y, Z int16
		S, T    uint16
	}
	verts := []vert{
		{0, 0, 0, 0, 0},
		{100, 0, 0, 32768, 0},
		{0, 100, 0, 0, 32768},
	}
	for _, v := range verts {
		_ = binary.Write(&buf, binary.LittleEndian, v)
	}
	buf.WriteByte(3)
	for i := range 3 {
		_ = binary.Write(&buf, binary.LittleEndian, uint32(i))
	}

	dec := &ply.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)

	p := &scene.Meshes[0].Primitives[0]
	assert.Equal(t, 3, p.Data.VertexCount)
	assert.InDelta(t, float32(100), p.Data.Positions[1][0], 0.01)
	require.NotNil(t, p.Data.TexCoord0)
	assert.InDelta(t, float32(32768), p.Data.TexCoord0[1][0], 1)
}

func TestDecodeBinaryLEIntCoords(t *testing.T) {
	header := "ply\nformat binary_little_endian 1.0\nelement vertex 3\nproperty int x\nproperty int y\nproperty int z\nelement face 1\nproperty list uchar int vertex_indices\nend_header\n"

	var buf bytes.Buffer
	buf.WriteString(header)

	verts := [][3]int32{{0, 0, 0}, {10, 0, 0}, {0, 10, 0}}
	for _, v := range verts {
		for _, c := range v {
			_ = binary.Write(&buf, binary.LittleEndian, c)
		}
	}
	buf.WriteByte(3)
	for i := range 3 {
		_ = binary.Write(&buf, binary.LittleEndian, int32(i))
	}

	dec := &ply.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)

	p := &scene.Meshes[0].Primitives[0]
	assert.InDelta(t, float32(10), p.Data.Positions[1][0], 0.01)
}
