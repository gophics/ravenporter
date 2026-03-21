package dae

import (
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const matrixSize = 16

func convertVisualScene(vs *visualScene, asset *ir.Asset, geoMap, camMap, lightMap map[string]int) {
	for i := range vs.Nodes {
		idx := len(asset.Nodes)
		convertNode(&vs.Nodes[i], asset, geoMap, camMap, lightMap, -1)
		asset.RootNodes = append(asset.RootNodes, idx)
	}
}

func convertNode(n *xmlNode, asset *ir.Asset, geoMap, camMap, lightMap map[string]int, parentIdx int) {
	nodeIdx := len(asset.Nodes)

	meshIdx := ir.NoIndex
	for _, ig := range n.InstGeom {
		if idx, ok := geoMap[ig.URL]; ok {
			meshIdx = idx
			break
		}
	}

	camIdx := ir.NoIndex
	for _, ic := range n.InstCam {
		if idx, ok := camMap[ic.URL]; ok {
			camIdx = idx
			break
		}
	}

	lightIdx := ir.NoIndex
	for _, il := range n.InstLit {
		if idx, ok := lightMap[il.URL]; ok {
			lightIdx = idx
			break
		}
	}

	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        n.Name,
		MeshIndex:   meshIdx,
		SkinIndex:   ir.NoIndex,
		CameraIndex: camIdx,
		LightIndex:  lightIdx,
		Transform:   parseNodeTransform(n.Matrix),
	}

	asset.Nodes = append(asset.Nodes, node)

	if parentIdx >= 0 {
		asset.Nodes[parentIdx].Children = append(asset.Nodes[parentIdx].Children, nodeIdx)
	}

	for i := range n.Children {
		convertNode(&n.Children[i], asset, geoMap, camMap, lightMap, nodeIdx)
	}
}

func parseNodeTransform(matStr string) ir.Transform {
	t := ir.IdentityTransform()

	if matStr == "" {
		return t
	}

	fields := decutil.SplitFields(matStr, make([]string, 0, matrixSize))
	if len(fields) != matrixSize {
		return t
	}

	var m [matrixSize]float32
	for i, f := range fields {
		m[i] = decutil.ParseF32(f)
	}

	t.Matrix = [matrixSize]float32{
		m[0], m[4], m[8], m[12],
		m[1], m[5], m[9], m[13],
		m[2], m[6], m[10], m[14],
		m[3], m[7], m[11], m[15],
	}

	return t
}
