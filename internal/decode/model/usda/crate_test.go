package usda

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeVR(offset int) uint64 { return uint64(offset) << valueRepPayShift }

func putU64LE(buf []byte, off int, v uint64) {
	binary.LittleEndian.PutUint64(buf[off:], v)
}

func putU32LE(buf []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(buf[off:], v)
}

func putF32LE(buf []byte, off int, v float32) {
	binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
}

func putF64LE(buf []byte, off int, v float64) {
	binary.LittleEndian.PutUint64(buf[off:], math.Float64bits(v))
}

func TestCrate_ReadInlineBool(t *testing.T) {
	cr := &crateReader{}
	assert.True(t, cr.readInlineBool(makeVR(1)))
	assert.False(t, cr.readInlineBool(makeVR(0)))
}

func TestCrate_ReadFloatArray(t *testing.T) {
	buf := make([]byte, 64)
	off := 16
	putU64LE(buf, off, 2) // count=2
	putF32LE(buf, off+8, 1.5)
	putF32LE(buf, off+12, 2.5)

	cr := &crateReader{data: buf}
	result := cr.readFloatArray(makeVR(off))
	require.Len(t, result, 2)
	assert.InDelta(t, float32(1.5), result[0], 0.001)
	assert.InDelta(t, float32(2.5), result[1], 0.001)

	t.Run("Invalid", func(t *testing.T) {
		assert.Nil(t, cr.readFloatArray(makeVR(0)))
	})
}

func TestCrate_ReadFloat64(t *testing.T) {
	buf := make([]byte, 32)
	off := 8
	putF64LE(buf, off, 3.14)

	cr := &crateReader{data: buf}
	assert.InDelta(t, 3.14, cr.readFloat64(makeVR(off)), 0.001)
	assert.Equal(t, 0.0, cr.readFloat64(makeVR(0)))
}

func TestCrate_ReadQuatfArray(t *testing.T) {
	buf := make([]byte, 64)
	off := 8
	putU64LE(buf, off, 1)      // count=1
	putF32LE(buf, off+8, 1.0)  // w
	putF32LE(buf, off+12, 0.0) // x
	putF32LE(buf, off+16, 0.0) // y
	putF32LE(buf, off+20, 0.0) // z

	cr := &crateReader{data: buf}
	result := cr.readQuatfArray(makeVR(off))
	require.Len(t, result, 1)
	// pion layout: x, y, z, w
	assert.InDelta(t, float32(0), result[0][0], 0.001) // x
	assert.InDelta(t, float32(1), result[0][3], 0.001) // w

	t.Run("Invalid", func(t *testing.T) {
		assert.Nil(t, cr.readQuatfArray(makeVR(0)))
	})
}

func TestCrate_ReadJointIndices(t *testing.T) {
	buf := make([]byte, 48)
	off := 8
	putU64LE(buf, off, 4)    // count=4
	putU32LE(buf, off+8, 0)  // joint 0
	putU32LE(buf, off+12, 1) // joint 1
	putU32LE(buf, off+16, 2) // joint 2
	putU32LE(buf, off+20, 0) // joint 3

	cr := &crateReader{data: buf}
	result := cr.readJointIndices(makeVR(off))
	require.Len(t, result, 1)
	assert.Equal(t, uint16(0), result[0][0])
	assert.Equal(t, uint16(1), result[0][1])

	t.Run("Empty", func(t *testing.T) {
		assert.Nil(t, cr.readJointIndices(makeVR(0)))
	})
}

func TestCrate_ReadJointWeights(t *testing.T) {
	buf := make([]byte, 48)
	off := 8
	putU64LE(buf, off, 4)      // count=4
	putF32LE(buf, off+8, 1.0)  // w0
	putF32LE(buf, off+12, 0.0) // w1
	putF32LE(buf, off+16, 0.0) // w2
	putF32LE(buf, off+20, 0.0) // w3

	cr := &crateReader{data: buf}
	result := cr.readJointWeights(makeVR(off))
	require.Len(t, result, 1)
	assert.InDelta(t, float32(1.0), result[0][0], 0.001)

	t.Run("Empty", func(t *testing.T) {
		assert.Nil(t, cr.readJointWeights(makeVR(0)))
	})
}

func TestCrate_ReadVec3dArray(t *testing.T) {
	buf := make([]byte, 64)
	off := 8
	putU64LE(buf, off, 1) // count=1
	putF64LE(buf, off+8, 1.0)
	putF64LE(buf, off+16, 2.0)
	putF64LE(buf, off+24, 3.0)

	cr := &crateReader{data: buf}
	result := cr.readVec3dArray(makeVR(off))
	require.Len(t, result, 1)
	assert.InDelta(t, float32(1.0), result[0][0], 0.001)
	assert.InDelta(t, float32(3.0), result[0][2], 0.001)

	t.Run("Invalid", func(t *testing.T) {
		assert.Nil(t, cr.readVec3dArray(makeVR(0)))
	})
}

func TestCrate_ReadMatrix4d(t *testing.T) {
	buf := make([]byte, 256)
	off := 16
	for i := range mat4dElems {
		putF64LE(buf, off+i*crateF64Size, float64(i+1))
	}

	cr := &crateReader{data: buf}
	m := cr.readMatrix4d(makeVR(off))
	assert.InDelta(t, float32(1), m[0], 0.001)
	assert.InDelta(t, float32(16), m[15], 0.001)

	t.Run("Invalid", func(t *testing.T) {
		m := cr.readMatrix4d(makeVR(0))
		assert.Equal(t, [16]float32{}, m)
	})
}

func TestCrate_ReadMatrix4dArray(t *testing.T) {
	buf := make([]byte, 512)
	off := 16
	putU64LE(buf, off, 1) // count=1
	for i := range mat4dElems {
		putF64LE(buf, off+crateU64Size+i*crateF64Size, float64(i+1))
	}

	cr := &crateReader{data: buf}
	result := cr.readMatrix4dArray(makeVR(off))
	require.Len(t, result, 1)
	assert.InDelta(t, float32(1), result[0][0], 0.001)
	assert.InDelta(t, float32(16), result[0][15], 0.001)

	t.Run("Invalid", func(t *testing.T) {
		assert.Nil(t, cr.readMatrix4dArray(makeVR(0)))
	})
}

func TestCrate_ReadTokenArray(t *testing.T) {
	buf := make([]byte, 32)
	off := 8
	putU64LE(buf, off, 2)    // count=2
	putU32LE(buf, off+8, 0)  // token index 0
	putU32LE(buf, off+12, 1) // token index 1

	cr := &crateReader{
		data:   buf,
		tokens: []string{"Root", "Arm"},
	}
	result := cr.readTokenArray(makeVR(off))
	require.Len(t, result, 2)
	assert.Equal(t, "Root", result[0])
	assert.Equal(t, "Arm", result[1])

	t.Run("Invalid", func(t *testing.T) {
		assert.Nil(t, cr.readTokenArray(makeVR(0)))
	})
}

func TestCrate_ReadInlineString(t *testing.T) {
	cr := &crateReader{strings: []string{"./ref.usda", "./heavy.usda"}}
	assert.Equal(t, "./ref.usda", cr.readInlineString(makeVR(0)))
	assert.Equal(t, "./heavy.usda", cr.readInlineString(makeVR(1)))
	assert.Equal(t, "", cr.readInlineString(makeVR(2)))
}

func TestCrate_ReadStringArray(t *testing.T) {
	buf := make([]byte, 32)
	off := 8
	putU64LE(buf, off, 2)
	putU32LE(buf, off+8, 0)
	putU32LE(buf, off+12, 1)

	cr := &crateReader{
		data:    buf,
		strings: []string{"./ref.usda", "./payload.usda"},
	}
	result := cr.readStringArray(makeVR(off))
	require.Len(t, result, 2)
	assert.Equal(t, "./ref.usda", result[0])
	assert.Equal(t, "./payload.usda", result[1])
}

func TestCrate_ReadTimeSampledVec3(t *testing.T) {
	buf := make([]byte, 128)
	sample0 := 64
	putU64LE(buf, sample0, 2)
	putF32LE(buf, sample0+8, 0)
	putF32LE(buf, sample0+12, 0)
	putF32LE(buf, sample0+16, 0)
	putF32LE(buf, sample0+20, 1)
	putF32LE(buf, sample0+24, 0)
	putF32LE(buf, sample0+28, 0)

	sample1 := 96
	putU64LE(buf, sample1, 2)
	putF32LE(buf, sample1+8, 0.1)
	putF32LE(buf, sample1+12, 0)
	putF32LE(buf, sample1+16, 0)
	putF32LE(buf, sample1+20, 1.1)
	putF32LE(buf, sample1+24, 0)
	putF32LE(buf, sample1+28, 0)

	timeSamples := 8
	putU64LE(buf, timeSamples, 2)
	putF64LE(buf, timeSamples+8, 0)
	putU64LE(buf, timeSamples+16, makeVR(sample0))
	putF64LE(buf, timeSamples+24, 1)
	putU64LE(buf, timeSamples+32, makeVR(sample1))

	cr := &crateReader{data: buf}
	times, frames := cr.readTimeSampledVec3(makeVR(timeSamples))
	require.Len(t, times, 2)
	require.Len(t, frames, 2)
	assert.InDelta(t, float32(1), times[1], 0.001)
	assert.InDelta(t, float32(0.1), frames[1][0][0], 0.001)
	assert.InDelta(t, float32(1.1), frames[1][1][0], 0.001)
}

func TestCrate_ParentPathIdx(t *testing.T) {
	cr := &crateReader{
		paths: []cratePath{
			{tokenIdx: 0, parentIdx: -1},
			{tokenIdx: 1, parentIdx: 0},
		},
	}
	assert.Equal(t, int32(-1), cr.parentPathIdx(0))
	assert.Equal(t, int32(0), cr.parentPathIdx(1))
	assert.Equal(t, int32(-1), cr.parentPathIdx(99))
	assert.Equal(t, int32(-1), cr.parentPathIdx(-1))
}

func TestCrate_ParseMatrix4d(t *testing.T) {
	input := "((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,0,0,1))"
	m := parseMatrix4d(input)
	assert.InDelta(t, float32(1), m[0], 0.001)
	assert.InDelta(t, float32(1), m[5], 0.001)
	assert.InDelta(t, float32(1), m[10], 0.001)
	assert.InDelta(t, float32(1), m[15], 0.001)
	assert.InDelta(t, float32(0), m[1], 0.001)
}
