package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenBoundingBoxes(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{
					VertexCount: 3,
					Positions:   [][3]float32{{-1, -2, -3}, {4, 5, 6}, {0, 0, 0}},
				},
			}},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPGenBoundingBoxes, process.Options{}))

	bb := scene.Meshes[0].BoundingBox
	assert.Equal(t, [3]float32{-1, -2, -3}, bb[0])
	assert.Equal(t, [3]float32{4, 5, 6}, bb[1])
}

func TestGenBoundingBoxesNilMesh(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{nil}}
	require.NoError(t, process.Apply(scene, process.PPGenBoundingBoxes, process.Options{}))
}

func TestGenBoundingBoxesEmpty(t *testing.T) {
	scene := &ir.Asset{Meshes: []*ir.Mesh{{Primitives: []ir.Primitive{}}}}
	require.NoError(t, process.Apply(scene, process.PPGenBoundingBoxes, process.Options{}))
}
