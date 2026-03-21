package dae

import (
	"context"
	"strings"

	"github.com/gophics/ravenporter/ir"
)

const (
	semanticJoint         = "JOINT"
	semanticInvBindMatrix = "INV_BIND_MATRIX"
	matSize               = 16
	semanticMorphTarget   = "MORPH_TARGET"
	semanticMorphWeight   = "MORPH_WEIGHT"
	semanticWeight        = "WEIGHT"
	maxJointsPerVertex    = 4
)

func convertSkins(
	sysCtx context.Context, controllers []xmlController,
	asset *ir.Asset, geoPositionMap map[string][][3]float32,
) []*ir.Skeleton {
	if len(controllers) == 0 {
		return nil
	}

	var skeletons []*ir.Skeleton
	for i := range controllers {
		ctrl := &controllers[i]

		if skel := convertSingleSkin(sysCtx, ctrl, asset); skel != nil {
			skeletons = append(skeletons, skel)
		}

		convertMorphController(sysCtx, ctrl, asset, geoPositionMap)
	}
	return skeletons
}

func convertMorphController(
	sysCtx context.Context, ctrl *xmlController,
	asset *ir.Asset, geoPositionMap map[string][][3]float32,
) {
	morph := &ctrl.Morph
	if len(morph.Sources) == 0 {
		return
	}

	srcMap := buildSourceMap(sysCtx, morph.Sources)

	var targetNames []string
	var weightData []float32
	for _, inp := range morph.Targets.Inputs {
		switch inp.Semantic {
		case semanticMorphTarget:
			targetNames = parseNameArray(morph.Sources, inp.Source)
		case semanticMorphWeight:
			weightData = srcMap[inp.Source]
		}
	}

	if len(targetNames) == 0 {
		return
	}

	var targetMesh *ir.Mesh
	for _, m := range asset.Meshes {
		if m.Name != "" && strings.Contains(morph.Source, m.Name) {
			targetMesh = m
			break
		}
	}
	if targetMesh == nil || len(targetMesh.Primitives) == 0 {
		return
	}

	for i, name := range targetNames {
		mt := ir.MorphTarget{Name: name}
		if positions, ok := geoPositionMap[name]; ok {
			mt.Positions = positions
		}
		targetMesh.Primitives[0].MorphTargets = append(targetMesh.Primitives[0].MorphTargets, mt)
		if i < len(weightData) {
			targetMesh.MorphWeights = append(targetMesh.MorphWeights, weightData[i])
		}
	}
}

func convertSingleSkin(sysCtx context.Context, ctrl *xmlController, asset *ir.Asset) *ir.Skeleton {
	skin := &ctrl.Skin
	srcMap := buildSourceMap(sysCtx, skin.Sources)

	var jointNames []string
	var ibmData []float32
	for _, inp := range skin.Joints.Inputs {
		switch inp.Semantic {
		case semanticJoint:
			jointNames = parseNameArray(skin.Sources, inp.Source)
		case semanticInvBindMatrix:
			ibmData = srcMap[inp.Source]
		}
	}

	if len(jointNames) == 0 {
		return nil
	}

	skel := &ir.Skeleton{
		Name: ctrl.Name,
	}

	for i, name := range jointNames {
		nodeIdx := ir.NoIndex
		for j := range asset.Nodes {
			if asset.Nodes[j].Name == name {
				nodeIdx = j
				asset.Nodes[j].IsJoint = true
				break
			}
		}
		skel.Joints = append(skel.Joints, nodeIdx)

		var ibm [16]float32
		base := i * matSize
		if ibmData != nil && base+matSize <= len(ibmData) {
			copy(ibm[:], ibmData[base:base+matSize])
		}
		skel.InverseBindMatrices = append(skel.InverseBindMatrices, ibm)
	}

	parseAndApplyVertexWeights(sysCtx, skin, srcMap, asset)

	skel.RootIdx = findSkeletonRoot(skel.Joints, asset)
	return skel
}

func findSkeletonRoot(joints []int, asset *ir.Asset) int {
	jointSet := make(map[int]bool, len(joints))
	for _, j := range joints {
		jointSet[j] = true
	}

	childOfJoint := make(map[int]bool, len(joints))
	for _, j := range joints {
		if j < 0 || j >= len(asset.Nodes) {
			continue
		}
		for _, c := range asset.Nodes[j].Children {
			if jointSet[c] {
				childOfJoint[c] = true
			}
		}
	}

	for i, j := range joints {
		if !childOfJoint[j] {
			return i
		}
	}
	return 0
}

func parseAndApplyVertexWeights(sysCtx context.Context, skin *xmlSkin, srcMap map[string][]float32, asset *ir.Asset) {
	vw := &skin.VertexWeights
	if vw.Count == 0 {
		return
	}

	var weightSrc string
	var jointOffset, weightOffset int
	for _, inp := range vw.Inputs {
		switch inp.Semantic {
		case semanticJoint:
			jointOffset = inp.Offset
		case semanticWeight:
			weightSrc = inp.Source
			weightOffset = inp.Offset
		}
	}
	weightData := srcMap[weightSrc]

	vcounts := parseInts(sysCtx, vw.VCount)
	vdata := parseInts(sysCtx, vw.V)

	var targetMesh *ir.Mesh
	for _, m := range asset.Meshes {
		if m.Name != "" && strings.Contains(skin.Source, m.Name) {
			targetMesh = m
			break
		}
	}
	if targetMesh == nil && len(asset.Meshes) > 0 {
		targetMesh = asset.Meshes[0]
	}
	if targetMesh == nil || len(targetMesh.Primitives) == 0 {
		return
	}

	prim := &targetMesh.Primitives[0]
	vertCount := prim.Data.VertexCount
	if vertCount == 0 {
		vertCount = len(vcounts)
	}

	joints := make([][4]uint16, vertCount)
	weights := make([][4]float32, vertCount)

	var stride int
	if jointOffset > weightOffset {
		stride = jointOffset + 1
	} else {
		stride = weightOffset + 1
	}

	vIdx := 0
	for vi, count := range vcounts {
		if vi >= vertCount {
			break
		}
		for j := range min(count, maxJointsPerVertex) {
			base := (vIdx + j) * stride
			if base+stride > len(vdata) {
				break
			}
			jIdx := vdata[base+jointOffset]
			wIdx := vdata[base+weightOffset]
			joints[vi][j] = uint16(jIdx) //nolint:gosec // bounded
			if wIdx >= 0 && wIdx < len(weightData) {
				weights[vi][j] = weightData[wIdx]
			}
		}
		vIdx += count
	}

	prim.Data.Joints0 = joints
	prim.Data.Weights0 = weights
}

func parseNameArray(sources []source, ref string) []string {
	for _, s := range sources {
		if "#"+s.ID == ref {
			return strings.Fields(s.FloatArray.Data) // Name arrays use same chardata field
		}
	}
	return nil
}
