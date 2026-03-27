package models

import (
	"encoding/binary"
	"math"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type weldVerticesStep struct{}

func (s *weldVerticesStep) Name() string      { return "WeldSharedVertices" }
func (s *weldVerticesStep) Flag() core.PPFlag { return core.PPJoinIdenticalVertices }

func (s *weldVerticesStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			weldPrimitive(&p.Data)
		}
	}
	return asset, nil
}

func putFloat32(buf []byte, off int, v float32) int {
	binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
	return off + 4 //nolint:mnd // sizeof(float32)
}

func putVec2(buf []byte, off int, v [2]float32) int {
	off = putFloat32(buf, off, v[0])
	return putFloat32(buf, off, v[1])
}

func putVec3(buf []byte, off int, v [3]float32) int {
	off = putFloat32(buf, off, v[0])
	off = putFloat32(buf, off, v[1])
	return putFloat32(buf, off, v[2])
}

func putVec4(buf []byte, off int, v [4]float32) int {
	off = putFloat32(buf, off, v[0])
	off = putFloat32(buf, off, v[1])
	off = putFloat32(buf, off, v[2])
	return putFloat32(buf, off, v[3])
}

func putJoints4(buf []byte, off int, v [4]uint16) int {
	binary.LittleEndian.PutUint16(buf[off:], v[0])
	binary.LittleEndian.PutUint16(buf[off+2:], v[1])
	binary.LittleEndian.PutUint16(buf[off+4:], v[2])
	binary.LittleEndian.PutUint16(buf[off+6:], v[3])
	return off + 8 //nolint:mnd // sizeof(uint16)*4
}

const maxKeySize = 120

//nolint:funlen // Vertex attribute welding uses a long procedural check structure
func weldPrimitive(d *ir.MeshData) {
	if len(d.Positions) == 0 {
		return
	}

	n := len(d.Positions)
	unique := make(map[string]uint32, n)
	newPositions := make([][3]float32, 0, n)
	newNormals := make([][3]float32, 0, n)
	newTangents := make([][4]float32, 0, n)
	newTexCoord0 := make([][2]float32, 0, n)
	newTexCoord1 := make([][2]float32, 0, n)
	newColors0 := make([][4]float32, 0, n)
	newJoints0 := make([][4]uint16, 0, n)
	newJoints1 := make([][4]uint16, 0, n)
	newWeights0 := make([][4]float32, 0, n)
	newWeights1 := make([][4]float32, 0, n)

	var newIndices []uint32
	hasNormals := len(d.Normals) == n
	hasTangents := len(d.Tangents) == n
	hasTex0 := len(d.TexCoord0) == n
	hasTex1 := len(d.TexCoord1) == n
	hasColors := len(d.Colors0) == n
	hasJoints0 := len(d.Joints0) == n
	hasJoints1 := len(d.Joints1) == n
	hasWeights0 := len(d.Weights0) == n
	hasWeights1 := len(d.Weights1) == n

	var keyBuf [maxKeySize]byte

	addVertex := func(i int) uint32 {
		off := 0
		off = putVec3(keyBuf[:], off, d.Positions[i])
		if hasNormals {
			off = putVec3(keyBuf[:], off, d.Normals[i])
		}
		if hasTangents {
			off = putVec4(keyBuf[:], off, d.Tangents[i])
		}
		if hasTex0 {
			off = putVec2(keyBuf[:], off, d.TexCoord0[i])
		}
		if hasTex1 {
			off = putVec2(keyBuf[:], off, d.TexCoord1[i])
		}
		if hasColors {
			off = putVec4(keyBuf[:], off, d.Colors0[i])
		}
		if hasJoints0 {
			off = putJoints4(keyBuf[:], off, d.Joints0[i])
		}
		if hasJoints1 {
			off = putJoints4(keyBuf[:], off, d.Joints1[i])
		}
		if hasWeights0 {
			off = putVec4(keyBuf[:], off, d.Weights0[i])
		}
		if hasWeights1 {
			off = putVec4(keyBuf[:], off, d.Weights1[i])
		}

		key := string(keyBuf[:off])
		if idx, exists := unique[key]; exists {
			return idx
		}

		idx := uint32(len(newPositions)) //nolint:gosec // hardware verified
		unique[key] = idx
		newPositions = append(newPositions, d.Positions[i])
		if hasNormals {
			newNormals = append(newNormals, d.Normals[i])
		}
		if hasTangents {
			newTangents = append(newTangents, d.Tangents[i])
		}
		if hasTex0 {
			newTexCoord0 = append(newTexCoord0, d.TexCoord0[i])
		}
		if hasTex1 {
			newTexCoord1 = append(newTexCoord1, d.TexCoord1[i])
		}
		if hasColors {
			newColors0 = append(newColors0, d.Colors0[i])
		}
		if hasJoints0 {
			newJoints0 = append(newJoints0, d.Joints0[i])
		}
		if hasJoints1 {
			newJoints1 = append(newJoints1, d.Joints1[i])
		}
		if hasWeights0 {
			newWeights0 = append(newWeights0, d.Weights0[i])
		}
		if hasWeights1 {
			newWeights1 = append(newWeights1, d.Weights1[i])
		}
		return idx
	}

	if d.HasIndices() {
		newIndices = make([]uint32, 0, len(d.Indices))
		for _, oldIdx := range d.Indices {
			newIndices = append(newIndices, addVertex(int(oldIdx)))
		}
	} else {
		newIndices = make([]uint32, 0, n)
		for i := range d.Positions {
			newIndices = append(newIndices, addVertex(i))
		}
	}

	if len(newPositions) < n || !d.HasIndices() {
		d.Positions = newPositions
		d.Normals = newNormals
		d.Tangents = newTangents
		d.TexCoord0 = newTexCoord0
		d.TexCoord1 = newTexCoord1
		d.Colors0 = newColors0
		d.Joints0 = newJoints0
		d.Joints1 = newJoints1
		d.Weights0 = newWeights0
		d.Weights1 = newWeights1
		d.Indices = newIndices
		d.VertexCount = len(newPositions)
	}
}
