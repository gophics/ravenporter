package mathx

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClampAndNaN(t *testing.T) {
	assert.Equal(t, 1.0, Clamp(0.5, 1, 3))
	assert.Equal(t, 3.0, Clamp(4, 1, 3))
	assert.Equal(t, 2.0, Clamp(2, 1, 3))
	assert.True(t, IsNaN32(float32(math.NaN())))
	assert.False(t, IsNaN32(1))
}

func TestVectorHelpers(t *testing.T) {
	assert.InDelta(t, 5, VecLen3(3, 4, 0), 0.0001)
	assert.Equal(t, [][3]float32{{1, 2, 3}, {4, 5, 6}}, FloatsToVec3s([]float32{1, 2, 3, 4, 5, 6}))
	assert.Equal(t, [][4]float32{{1, 2, 3, 4}}, FloatsToVec4s([]float32{1, 2, 3, 4}))
	assert.Equal(t, [3]float32{1, 1, 1}, Sub3([3]float32{2, 3, 4}, [3]float32{1, 2, 3}))
	assert.Equal(t, [3]float32{0, 0, 1}, Cross3([3]float32{1, 0, 0}, [3]float32{0, 1, 0}))
	assert.Equal(t, [3]float32{0, 1, 0}, Normalize3([3]float32{}))
	assert.Equal(t, [3]float32{0, 0, 1}, Normalize3([3]float32{0, 0, 5}))
}

func TestQuaternionHelpers(t *testing.T) {
	q := EulerToQuat(0, 0, 0)
	assert.Equal(t, [4]float32{0, 0, 0, 1}, q)
	assert.Equal(t, q, QuatToArr(ArrToQuat(q)))
	assert.Equal(t, q, QuatMulArr(q, q))

	matQuat := MatToQuat(1, 0, 0, 0, 1, 0, 0, 0, 1)
	assert.Equal(t, [4]float32{0, 0, 0, 1}, matQuat)
}

func TestTransformHelpers(t *testing.T) {
	matrix := ComposeTRS(Vec3{1, 2, 3}, Quat{W: 1}, Vec3{2, 3, 4})
	translation, rotation, scale := DecomposeTRS(matrix)
	assert.InDeltaSlice(t, []float64{1, 2, 3}, []float64{float64(translation[0]), float64(translation[1]), float64(translation[2])}, 0.0001)
	assert.InDeltaSlice(t, []float64{2, 3, 4}, []float64{float64(scale[0]), float64(scale[1]), float64(scale[2])}, 0.0001)
	assert.Equal(t, [4]float32{0, 0, 0, 1}, QuatToArr(rotation))

	mat := Mat4From3x4([12]float32{1, 0, 0, 0, 1, 0, 0, 0, 1, 4, 5, 6})
	assert.Equal(t, float32(4), mat[12])
	assert.Equal(t, float32(5), mat[13])
	assert.Equal(t, float32(6), mat[14])
}
