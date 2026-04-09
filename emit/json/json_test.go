package jsonir_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/emit"
	jsonir "github.com/gophics/ravenporter/emit/json"
	"github.com/gophics/ravenporter/ir"
)

type memFS struct{ files map[string]*bytes.Buffer }

func (m *memFS) Create(path string) (io.WriteCloser, error) {
	buf := &bytes.Buffer{}
	m.files[path] = buf
	return nopCloser{buf}, nil
}

type nopCloser struct{ *bytes.Buffer }

func (nopCloser) Close() error { return nil }

func TestEmitJSON(t *testing.T) {
	asset := &ir.Asset{
		Name: "test",
		Images: []*ir.ImageAsset{{
			Name:     "texture",
			Format:   ir.ImageDDS,
			Width:    16,
			Height:   16,
			Topology: ir.ImageTopologyCube,
			Depth:    1,
			Layers:   6,
		}},
		AudioClips: []*ir.AudioClip{{
			Name:        "clip",
			Format:      ir.AudioWAV,
			SampleRate:  48000,
			Layout:      ir.LayoutStereo,
			ChannelMask: 3,
		}},
		Meshes: []*ir.Mesh{{
			Name: "cube",
			Primitives: []ir.Primitive{{
				Mode:          ir.Triangles,
				MaterialIndex: ir.NoIndex,
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
				},
			}},
		}},
	}

	fs := &memFS{files: make(map[string]*bytes.Buffer)}
	e := &jsonir.Emitter{}

	require.NoError(t, e.Emit(asset, fs, emit.Options{BaseName: "out", PrettyPrint: true}))
	assert.Contains(t, fs.files, "out.json")

	var decoded ir.Asset
	require.NoError(t, json.Unmarshal(fs.files["out.json"].Bytes(), &decoded))
	assert.Equal(t, "test", decoded.Name)
	require.Len(t, decoded.Images, 1)
	assert.Equal(t, ir.ImageTopologyCube, decoded.Images[0].Topology)
	assert.Equal(t, 6, decoded.Images[0].Layers)
	require.Len(t, decoded.AudioClips, 1)
	assert.Equal(t, uint32(3), decoded.AudioClips[0].ChannelMask)
	require.Len(t, decoded.Meshes, 1)
	assert.Equal(t, "cube", decoded.Meshes[0].Name)
}

func TestWriteTo(t *testing.T) {
	scene := &ir.Asset{Name: "minimal"}
	var buf bytes.Buffer
	require.NoError(t, jsonir.WriteTo(scene, &buf, false))
	assert.Contains(t, buf.String(), `"Name":"minimal"`)
}

func TestEmitUsesDefaultBaseName(t *testing.T) {
	fs := &memFS{files: make(map[string]*bytes.Buffer)}
	err := (&jsonir.Emitter{}).Emit(&ir.Asset{Name: "scene"}, fs, emit.Options{})
	require.NoError(t, err)
	assert.Contains(t, fs.files, "scene.json")
}

func TestEmitCreateError(t *testing.T) {
	expected := errors.New("create failed")
	fs := &errFS{createErr: expected}
	err := (&jsonir.Emitter{}).Emit(&ir.Asset{}, fs, emit.Options{})
	require.ErrorIs(t, err, expected)
}

func TestEmitCloseError(t *testing.T) {
	expected := errors.New("close failed")
	fs := &errFS{writer: errCloser{Buffer: &bytes.Buffer{}, closeErr: expected}}
	err := (&jsonir.Emitter{}).Emit(&ir.Asset{}, fs, emit.Options{})
	require.ErrorIs(t, err, expected)
}

func TestWriteToPretty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, jsonir.WriteTo(&ir.Asset{Name: "pretty"}, &buf, true))
	assert.Contains(t, buf.String(), "\n")
}

type errFS struct {
	writer    io.WriteCloser
	createErr error
}

func (e *errFS) Create(_ string) (io.WriteCloser, error) {
	if e.createErr != nil {
		return nil, e.createErr
	}
	return e.writer, nil
}

type errCloser struct {
	*bytes.Buffer
	closeErr error
}

func (e errCloser) Close() error { return e.closeErr }
