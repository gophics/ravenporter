package mathx

import "github.com/go-gl/mathgl/mgl32"

const DegToRad = 3.14159265358979323846 / 180.0

// EulerToQuat converts XYZ euler angles (radians) to [x,y,z,w].
func EulerToQuat(x, y, z float64) [4]float32 {
	q := mgl32.AnglesToQuat(float32(x), float32(y), float32(z), mgl32.XYZ).Normalize()
	return QuatToArr(q)
}

// MatToQuat converts a 3×3 rotation matrix to [x,y,z,w].
func MatToQuat(r00, r01, r02, r10, r11, r12, r20, r21, r22 float32) [4]float32 {
	return QuatToArr(quatFromColumns(
		Vec3{r00, r10, r20},
		Vec3{r01, r11, r21},
		Vec3{r02, r12, r22},
	))
}

// ArrToQuat converts [x,y,z,w] to mgl32.Quat.
func ArrToQuat(a [4]float32) Quat {
	return mgl32.Quat{W: a[3], V: Vec3{a[0], a[1], a[2]}}
}

// QuatToArr converts mgl32.Quat to [x,y,z,w].
func QuatToArr(q Quat) [4]float32 {
	return [4]float32{q.V[0], q.V[1], q.V[2], q.W}
}

// QuatMulArr multiplies two [x,y,z,w] quaternions.
func QuatMulArr(a, b [4]float32) [4]float32 {
	return QuatToArr(ArrToQuat(a).Mul(ArrToQuat(b)))
}
