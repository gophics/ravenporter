package fbx

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

func TestFBXBasics(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"ProbeBinary", func(t *testing.T) {
			d := &Decoder{}
			buf := bytes.NewReader(append([]byte("Kaydara FBX Binary  \x00"), make([]byte, 20)...))
			assert.True(t, d.Probe(buf))
		}},
		{"ProbeASCII", func(t *testing.T) {
			d := &Decoder{}
			header := []byte("; FBX 7.5.0 project file\nFBXHeaderExtension: {\n")
			assert.True(t, d.Probe(bytes.NewReader(header)))
		}},
		{"ProbeRejectsNonFBX", func(t *testing.T) {
			d := &Decoder{}
			assert.False(t, d.Probe(bytes.NewReader([]byte("glTF\x02\x00\x00\x00"))))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.check(t) })
	}
}

func TestDecodeMinimal(t *testing.T) {
	data := buildMinimalFBX()
	d := &Decoder{}

	asset, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, ir.FormatFBX, asset.Metadata.SourceFormat)
}

func TestParseProperties(t *testing.T) {
	var buf []byte
	buf = append(buf, propInt32)
	buf = binary.LittleEndian.AppendUint32(buf, 42)

	pa := pool.NewArena[fbxProp](4)
	props, n, err := parseProperties(buf, 0, 1, &pa)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	require.Len(t, props, 1)
	assert.Equal(t, int64(42), props[0].intVal)
}

func TestTriangulatePolygons(t *testing.T) {
	tests := []struct {
		name   string
		poly   []int32
		expect []uint32
	}{
		{"Triangle", []int32{0, 1, ^int32(2)}, []uint32{0, 1, 2}},
		{"Quad", []int32{0, 1, 2, ^int32(3)}, []uint32{0, 1, 2, 0, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, triangulatePolygons(tt.poly))
		})
	}
}

func TestFanTriangulateQuad(t *testing.T) {
	face := []int32{0, 1, 2, 3}
	result := fanTriangulate(nil, face)
	assert.Equal(t, []uint32{0, 1, 2, 0, 2, 3}, result)
}

func buildMinimalFBX() []byte {
	var buf []byte
	buf = append(buf, []byte("Kaydara FBX Binary  \x00")...)
	buf = append(buf, 0x1A, 0x00)
	buf = binary.LittleEndian.AppendUint32(buf, 7400)
	buf = append(buf, make([]byte, nullNodeSize32)...)
	return buf
}

func TestFBXDecodeAll(t *testing.T) {
	tests := []struct {
		name    string
		inputFn func() ([]byte, error)
		check   func(t *testing.T, asset *ir.Asset)
	}{
		{
			"Minimal",
			func() ([]byte, error) { return buildMinimalFBX(), nil },
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.FormatFBX, asset.Metadata.SourceFormat)
			},
		},
		{
			"BoxFBX",
			func() ([]byte, error) { return os.ReadFile("testdata/box.fbx") },
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.FormatFBX, asset.Metadata.SourceFormat)
				require.Len(t, asset.Meshes, 1)
				mesh := asset.Meshes[0]
				assert.Equal(t, "mesh_id43", mesh.Name)
				require.Len(t, mesh.Primitives, 1)
				prim := mesh.Primitives[0]
				assert.Equal(t, 36, prim.Data.VertexCount)
				assert.Len(t, prim.Data.Positions, 36)
				assert.Len(t, prim.Data.Indices, 36)
				assert.InDelta(t, float32(100), prim.Data.Positions[0][0], 0.1)
				assert.InDelta(t, float32(100), prim.Data.Positions[0][1], 0.1)
				assert.InDelta(t, float32(-100), prim.Data.Positions[0][2], 0.1)
				require.True(t, prim.Data.HasNormals())
				assert.Len(t, prim.Data.Normals, 36)
				require.Len(t, asset.Materials, 1)
				mat := asset.Materials[0]
				assert.Equal(t, "Material_50", mat.Name)
				assert.InDelta(t, float32(0.99), mat.BaseColorFactor[0], 0.02)
				require.Len(t, asset.Nodes, 1)
				assert.Equal(t, "root", asset.Nodes[0].Name)
				assert.Equal(t, 0, asset.Nodes[0].MeshIndex)
			},
		},
		{
			"ASCIIFBX",
			func() ([]byte, error) { return os.ReadFile("testdata/ascii.fbx") },
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.FormatFBX, asset.Metadata.SourceFormat)
				assert.Equal(t, "7500", asset.Metadata.SourceVersion)
				assert.Greater(t, len(asset.Meshes), 0)
				for _, mesh := range asset.Meshes {
					require.Len(t, mesh.Primitives, 1)
					prim := mesh.Primitives[0]
					assert.Greater(t, prim.Data.VertexCount, 0)
					assert.Greater(t, len(prim.Data.Positions), 0)
				}
			},
		},
	}

	d := &Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.inputFn()
			require.NoError(t, err)

			asset, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
			require.NoError(t, err)

			tt.check(t, asset)
		})
	}
}
