package fbx

import "math"

type layerElement struct {
	data    []float64
	indices []int32
	refMode string
	mapMode string
}

func parseLayerElement(node *fbxNode) *layerElement {
	le := &layerElement{}
	for _, child := range node.children {
		switch child.name {
		case leNormals, leUV, leColors:
			if len(child.properties) > 0 {
				le.data = child.properties[0].arrF64
			}
		case leNormalIdx, leUVIdx, leColorIdx:
			if len(child.properties) > 0 {
				le.indices = child.properties[0].arrI32
			}
		case leRefInfo:
			if len(child.properties) > 0 {
				le.refMode = child.properties[0].strVal
			}
		case leMapInfo:
			if len(child.properties) > 0 {
				le.mapMode = child.properties[0].strVal
			}
		}
	}
	if len(le.data) == 0 {
		return nil
	}
	return le
}

// expandLayer reads layer element data into a typed slice using a generic decode function.
func expandLayer[T any](le *layerElement, polyIndices []int32, stride int, decode func(data []float64, base int) T) []T {
	count := len(polyIndices)
	out := make([]T, count)
	maxDataIdx := len(le.data)/stride - 1

	for vi, idx := range polyIndices {
		vertIdx := idx
		if vertIdx < 0 {
			vertIdx = ^vertIdx
		}

		dataIdx := resolveLayerIndex(le, vi, int(vertIdx))
		if dataIdx > maxDataIdx {
			dataIdx = maxDataIdx
		}
		if dataIdx >= 0 {
			base := dataIdx * stride
			if base+stride <= len(le.data) {
				out[vi] = decode(le.data, base)
			}
		}
	}
	return out
}

func expandNormals(le *layerElement, polyIndices []int32) [][3]float32 {
	return expandLayer(le, polyIndices, vecStride, func(data []float64, base int) [3]float32 {
		return [3]float32{float32(data[base]), float32(data[base+1]), float32(data[base+2])}
	})
}

func expandUVs(le *layerElement, polyIndices []int32) [][2]float32 {
	return expandLayer(le, polyIndices, 2, func(data []float64, base int) [2]float32 { //nolint:mnd // UV stride
		return [2]float32{float32(data[base]), float32(data[base+1])}
	})
}

func expandColors(le *layerElement, polyIndices []int32) [][4]float32 {
	return expandLayer(le, polyIndices, colorVecStride, func(data []float64, base int) [4]float32 {
		return [4]float32{float32(data[base]), float32(data[base+1]), float32(data[base+2]), float32(data[base+3])}
	})
}

func expandTangents(le *layerElement, polyIndices []int32) [][4]float32 {
	return expandLayer(le, polyIndices, vecStride, func(data []float64, base int) [4]float32 {
		return [4]float32{float32(data[base]), float32(data[base+1]), float32(data[base+2]), 1.0}
	})
}

func resolveLayerIndex(le *layerElement, polyVertIdx, controlPtIdx int) int {
	switch {
	case le.refMode == refIndexToDirect && len(le.indices) > 0:
		if polyVertIdx < len(le.indices) {
			return int(le.indices[polyVertIdx])
		}
		// Fallback: use control-point index when index table is shorter than polygon vertices.
		return controlPtIdx
	case le.mapMode == mapByPolygonVtx || le.mapMode == "":
		return polyVertIdx
	case le.mapMode == mapByVertex:
		return controlPtIdx
	default:
		return polyVertIdx
	}
}

func parseSmoothingGroups(node *fbxNode) []int32 {
	for _, child := range node.children {
		if child.name == leSmoothData && len(child.properties) > 0 && child.properties[0].arrI32 != nil {
			return child.properties[0].arrI32
		}
	}
	return nil
}

const (
	smoothTriStride = 3
)

func computeNormalsFromSmoothing(
	positions [][3]float32, indices []uint32, groups []int32,
) [][3]float32 {
	triCount := len(indices) / smoothTriStride
	if triCount == 0 {
		return nil
	}

	faceNormals := make([][3]float32, triCount)
	for i := range triCount {
		base := i * smoothTriStride
		ai := indices[base]
		bi := indices[base+1]
		ci := indices[base+2]
		ax, ay, az := positions[ai][0], positions[ai][1], positions[ai][2]
		bx, by, bz := positions[bi][0], positions[bi][1], positions[bi][2]
		cx, cy, cz := positions[ci][0], positions[ci][1], positions[ci][2]
		e1x, e1y, e1z := bx-ax, by-ay, bz-az
		e2x, e2y, e2z := cx-ax, cy-ay, cz-az
		faceNormals[i] = [3]float32{
			e1y*e2z - e1z*e2y,
			e1z*e2x - e1x*e2z,
			e1x*e2y - e1y*e2x,
		}
	}

	normals := make([][3]float32, len(indices))
	for vi := range indices {
		faceA := vi / smoothTriStride
		sgA := int32(0)
		if faceA < len(groups) {
			sgA = groups[faceA]
		}
		var acc [3]float32
		vtxIdx := indices[vi]

		for fi := range triCount {
			sgB := int32(0)
			if fi < len(groups) {
				sgB = groups[fi]
			}
			if sgA&sgB == 0 && sgA != 0 && sgB != 0 {
				continue
			}
			base := fi * smoothTriStride
			if indices[base] == vtxIdx || indices[base+1] == vtxIdx || indices[base+2] == vtxIdx {
				acc[0] += faceNormals[fi][0]
				acc[1] += faceNormals[fi][1]
				acc[2] += faceNormals[fi][2]
			}
		}

		length := acc[0]*acc[0] + acc[1]*acc[1] + acc[2]*acc[2] //nolint:gosec // fixed [3]float32 array
		if length > 0 {
			inv := 1.0 / float32(math.Sqrt(float64(length)))
			acc[0] *= inv
			acc[1] *= inv
			acc[2] *= inv
		}
		normals[vi] = acc
	}
	return normals
}
