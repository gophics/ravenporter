package ir_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gophics/ravenporter/ir"
)

func TestHasHelpers(t *testing.T) {
	d := &ir.MeshData{}
	assert.False(t, d.HasNormals())
	assert.False(t, d.HasTangents())
	assert.False(t, d.HasUVs())
	assert.False(t, d.HasColors())
	assert.False(t, d.HasBones())
	assert.False(t, d.HasIndices())

	d.Normals = make([][3]float32, 1)
	d.Tangents = make([][4]float32, 1)
	d.TexCoord0 = [][2]float32{{0, 0}}
	d.Colors0 = [][4]float32{{1, 1, 1, 1}}
	d.Joints0 = make([][4]uint16, 1)
	d.Indices = []uint32{0}

	assert.True(t, d.HasNormals())
	assert.True(t, d.HasTangents())
	assert.True(t, d.HasUVs())
	assert.True(t, d.HasColors())
	assert.True(t, d.HasBones())
	assert.True(t, d.HasIndices())
}
