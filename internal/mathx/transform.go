package mathx

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// ComposeTRS builds a 4x4 matrix from translation, rotation, and scale.
func ComposeTRS(t Vec3, r Quat, s Vec3) Mat4 {
	return mgl32.Translate3D(t[0], t[1], t[2]).
		Mul4(r.Mat4()).
		Mul4(mgl32.Scale3D(s[0], s[1], s[2]))
}

// DecomposeTRS extracts TRS from a 4x4 matrix. Assumes no shear.
func DecomposeTRS(m Mat4) (t Vec3, r Quat, s Vec3) {
	t = Vec3{m[12], m[13], m[14]}

	col0 := Vec3{m[0], m[1], m[2]}
	col1 := Vec3{m[4], m[5], m[6]}
	col2 := Vec3{m[8], m[9], m[10]}

	s[0] = col0.Len()
	s[1] = col1.Len()
	s[2] = col2.Len()

	det := Mat3{
		m[0], m[1], m[2],
		m[4], m[5], m[6],
		m[8], m[9], m[10],
	}.Det()
	if det < 0 {
		s[0] = -s[0]
	}

	if s[0] != 0 {
		col0 = col0.Mul(1.0 / s[0])
	}
	if s[1] != 0 {
		col1 = col1.Mul(1.0 / s[1])
	}
	if s[2] != 0 {
		col2 = col2.Mul(1.0 / s[2])
	}

	r = quatFromColumns(col0, col1, col2)
	return t, r, s
}

// Shepperd's method for numerically stable quaternion extraction.
func quatFromColumns(col0, col1, col2 Vec3) Quat {
	trace := col0[0] + col1[1] + col2[2]

	if trace > 0 {
		s := float32(math.Sqrt(float64(trace+1.0))) * 2 //nolint:mnd // Shepperd's method
		return mgl32.Quat{
			W: 0.25 * s,
			V: Vec3{
				(col1[2] - col2[1]) / s,
				(col2[0] - col0[2]) / s,
				(col0[1] - col1[0]) / s,
			},
		}.Normalize()
	}
	if col0[0] > col1[1] && col0[0] > col2[2] {
		s := float32(math.Sqrt(float64(1.0+col0[0]-col1[1]-col2[2]))) * 2 //nolint:mnd // Shepperd's method
		return mgl32.Quat{
			W: (col1[2] - col2[1]) / s,
			V: Vec3{
				0.25 * s,
				(col1[0] + col0[1]) / s,
				(col2[0] + col0[2]) / s,
			},
		}.Normalize()
	}
	if col1[1] > col2[2] {
		s := float32(math.Sqrt(float64(1.0+col1[1]-col0[0]-col2[2]))) * 2 //nolint:mnd // Shepperd's method
		return mgl32.Quat{
			W: (col2[0] - col0[2]) / s,
			V: Vec3{
				(col1[0] + col0[1]) / s,
				0.25 * s,
				(col2[1] + col1[2]) / s,
			},
		}.Normalize()
	}
	s := float32(math.Sqrt(float64(1.0+col2[2]-col0[0]-col1[1]))) * 2 //nolint:mnd // Shepperd's method
	return mgl32.Quat{
		W: (col0[1] - col1[0]) / s,
		V: Vec3{
			(col2[0] + col0[2]) / s,
			(col2[1] + col1[2]) / s,
			0.25 * s,
		},
	}.Normalize()
}

// Mat4From3x4 builds a 4x4 column-major matrix from a 3×4 row-major float array.
// Layout: [r0x r0y r0z | r1x r1y r1z | r2x r2y r2z | tx ty tz].
func Mat4From3x4(m [12]float32) Mat4 {
	return Mat4{
		m[0], m[1], m[2], 0,
		m[3], m[4], m[5], 0,
		m[6], m[7], m[8], 0,
		m[9], m[10], m[11], 1,
	}
}
