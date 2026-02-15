package mathx

import "math"

// VecLen3 returns the length of a 3D vector.
func VecLen3(x, y, z float32) float32 {
	return float32(math.Sqrt(float64(x*x + y*y + z*z)))
}

// FloatsToVec3s reinterprets a flat float32 slice as [][3]float32 (stride 3).
func FloatsToVec3s(data []float32) [][3]float32 {
	const stride = 3
	count := len(data) / stride
	out := make([][3]float32, count)
	for i := range count {
		base := i * stride
		out[i] = [3]float32{data[base], data[base+1], data[base+2]}
	}
	return out
}

// FloatsToVec4s reinterprets a flat float32 slice as [][4]float32 (stride 4).
func FloatsToVec4s(data []float32) [][4]float32 {
	const stride = 4
	count := len(data) / stride
	out := make([][4]float32, count)
	for i := range count {
		base := i * stride
		out[i] = [4]float32{data[base], data[base+1], data[base+2], data[base+3]}
	}
	return out
}

func Sub3(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

func Cross3(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func Normalize3(v [3]float32) [3]float32 {
	l := VecLen3(v[0], v[1], v[2])
	if l == 0 {
		return [3]float32{0, 1, 0}
	}
	return [3]float32{v[0] / l, v[1] / l, v[2] / l}
}
