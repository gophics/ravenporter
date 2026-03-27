package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortByPType(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{
				{Mode: ir.Points, Data: ir.MeshData{VertexCount: 1}},
				{Mode: ir.Triangles, Data: ir.MeshData{VertexCount: 3}},
				{Mode: ir.Lines, Data: ir.MeshData{VertexCount: 2}},
			},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPSortByPtype, process.Options{}))

	modes := make([]ir.PrimitiveMode, len(scene.Meshes[0].Primitives))
	for i, p := range scene.Meshes[0].Primitives {
		modes[i] = p.Mode
	}
	assert.Equal(t, []ir.PrimitiveMode{ir.Triangles, ir.Lines, ir.Points}, modes)
}

func TestSortByPTypeNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPSortByPtype, process.Options{}))
}
