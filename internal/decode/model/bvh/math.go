package bvh

import (
	"strconv"

	"github.com/gophics/ravenporter/internal/mathx"
)

func readVec3(vals [][]byte, offset int) [3]float32 {
	if offset+axisCount > len(vals) {
		return [3]float32{}
	}
	return [3]float32{
		parseF32B(vals[offset]),
		parseF32B(vals[offset+1]),
		parseF32B(vals[offset+2]),
	}
}

func eulerToQuat(vals [][]byte, offset int, order [axisCount]int) [4]float32 {
	if offset+axisCount > len(vals) {
		return mathx.IdentityQuat
	}

	var angles [axisCount]float64
	for i := range axisCount {
		v, _ := strconv.ParseFloat(string(vals[offset+i]), 64) //nolint:errcheck
		angles[order[i]] = v * mathx.DegToRad                  //nolint:gosec
	}

	return mathx.EulerToQuat(angles[0], angles[1], angles[2]) //nolint:gosec
}

func parseF32B(b []byte) float32 {
	v, _ := strconv.ParseFloat(string(b), 32) //nolint:errcheck
	return float32(v)
}
