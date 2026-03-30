package pipeline

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
)

func TestImportReportDependenciesGLTF(t *testing.T) {
	src := `{
	  "asset": {"version": "2.0"},
	  "extensionsUsed": ["KHR_materials_variants", "MSFT_lod"],
	  "buffers": [{"uri": "mesh.bin", "byteLength": 0}],
	  "images": [{"uri": "albedo.png", "mimeType": "image/png"}],
	  "textures": [{"source": 0}]
	}`

	result, err := importReader(context.Background(), bytes.NewReader([]byte(src)), "scene.gltf", config{
		DecodeOpts: detect.DecodeOptions{FS: stringMapFS{
			"mesh.bin": "",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Report.Source.Notes)
	assert.Equal(t, []string{"KHR_materials_variants", "MSFT_lod"}, result.Report.Source.Notes["gltf.extensions_used"])
	assert.Contains(t, result.Report.Dependencies, Dependency{
		Kind:       "buffer",
		Path:       "mesh.bin",
		Relation:   "buffer",
		ReportedBy: "decode:model/gltf",
	})
	assert.Contains(t, result.Report.Dependencies, Dependency{
		Kind:       "image",
		Path:       "albedo.png",
		Relation:   "image",
		ReportedBy: "asset",
	})
}

func TestImportReportDependenciesOBJ(t *testing.T) {
	src := "mtllib materials.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

	result, err := importReader(context.Background(), bytes.NewReader([]byte(src)), "scene.obj", config{
		DecodeOpts: detect.DecodeOptions{FS: stringMapFS{
			"materials.mtl": "newmtl Default\nmap_Kd albedo.png\n",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Report.Dependencies, Dependency{
		Kind:       "material-library",
		Path:       "materials.mtl",
		Relation:   "material-library",
		ReportedBy: "decode:model/obj",
	})
	assert.Contains(t, result.Report.Dependencies, Dependency{
		Kind:       "image",
		Path:       "albedo.png",
		Relation:   "image",
		ReportedBy: "asset",
	})
}

func TestImportReportDependenciesUSDA(t *testing.T) {
	src := `#usda 1.0
(
    subLayers = [@./base.usda@]
)
def Xform "Root" {
    references = @./ref.usda@</Model>
    payload = @./payload.usda@</Geo>
}`

	result, err := importReader(context.Background(), bytes.NewReader([]byte(src)), "scene.usda", config{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Report.Dependencies, Dependency{
		Kind:       "scene",
		Path:       "base.usda",
		Relation:   "sublayer",
		ReportedBy: "decode:model/usda",
	})
	assert.Contains(t, result.Report.Dependencies, Dependency{
		Kind:       "scene",
		Path:       "ref.usda",
		Relation:   "reference",
		ReportedBy: "decode:model/usda",
	})
	assert.Contains(t, result.Report.Dependencies, Dependency{
		Kind:       "scene",
		Path:       "payload.usda",
		Relation:   "payload",
		ReportedBy: "decode:model/usda",
	})
}

type stringMapFS map[string]string

func (fs stringMapFS) Open(name string) (io.ReadCloser, error) {
	content, ok := fs[name]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader([]byte(content))), nil
}
