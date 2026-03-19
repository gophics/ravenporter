package fbx

import (
	"strings"
)

func findNode(nodes []fbxNode, name string) *fbxNode {
	for i := range nodes {
		if nodes[i].name == name {
			return &nodes[i]
		}
	}
	return nil
}

func extractName(node *fbxNode) string {
	if len(node.properties) > 1 {
		n := node.properties[1].strVal
		if idx := strings.IndexByte(n, 0); idx >= 0 {
			n = n[:idx]
		}
		if n != "" {
			return n
		}
	}
	return formatName
}

func f64ToVec3(data []float64) [][3]float32 {
	count := len(data) / vecStride
	out := make([][3]float32, count)
	for i := range count {
		base := i * vecStride
		out[i] = [3]float32{
			float32(data[base]),
			float32(data[base+1]),
			float32(data[base+2]),
		}
	}
	return out
}

func triangulatePolygons(polyIndices []int32) []uint32 {
	indices := make([]uint32, 0, len(polyIndices))
	face := make([]int32, 0, 4) //nolint:mnd // quad common case

	for _, idx := range polyIndices {
		if idx < 0 {
			face = append(face, ^idx)
			indices = fanTriangulate(indices, face)
			face = face[:0]
		} else {
			face = append(face, idx)
		}
	}

	return indices
}

func fanTriangulate(dst []uint32, face []int32) []uint32 {
	if len(face) < vecStride {
		return dst
	}
	v0 := uint32(face[0]) //nolint:gosec // FBX indices are non-negative after XOR
	for i := 2; i < len(face); i++ {
		dst = append(dst, v0, uint32(face[i-1]), uint32(face[i])) //nolint:gosec // bounded
	}
	return dst
}

const videoContent = "Content"

func extractVideoContent(node *fbxNode) []byte {
	for _, child := range node.children {
		if child.name == videoContent && len(child.properties) > 0 && len(child.properties[0].rawVal) > 0 {
			return child.properties[0].rawVal
		}
	}
	return nil
}
