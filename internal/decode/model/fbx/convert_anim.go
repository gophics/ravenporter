package fbx

import (
	"strings"

	"github.com/gophics/ravenporter/ir"
)

func extractSubType(node *fbxNode) string {
	if len(node.properties) > 2 { //nolint:mnd // FBX objects: ID, name, subtype
		return node.properties[2].strVal
	}
	return ""
}

func extractCluster(node *fbxNode, id int64) fbxCluster {
	cl := fbxCluster{id: id}
	for _, child := range node.children {
		switch child.name {
		case clusterIndexes:
			if len(child.properties) > 0 {
				cl.idxs = child.properties[0].arrI32
			}
		case clusterWeights:
			if len(child.properties) > 0 {
				cl.weights = child.properties[0].arrF64
			}
		case clusterTransformLink:
			if len(child.properties) > 0 && child.properties[0].arrF64 != nil {
				raw := child.properties[0].arrF64
				for i := range min(len(raw), 16) { //nolint:mnd // 4x4 matrix
					cl.ibm[i] = float32(raw[i])
				}
			}
		}
	}
	return cl
}

func resolveSkins(asset *ir.Asset, conns []connection, clusters []fbxCluster, modelMap map[int64]int) {
	if len(clusters) == 0 {
		return
	}

	clusterMap := make(map[int64]int, len(clusters))
	for i, c := range clusters {
		clusterMap[c.id] = i
	}

	clusterToJoint := make(map[int64]int, len(clusters))
	for _, c := range conns {
		if _, ok := clusterMap[c.childID]; !ok {
			continue
		}
		if nodeIdx, ok := modelMap[c.parentID]; ok {
			clusterToJoint[c.childID] = nodeIdx
		}
	}

	skel := &ir.Skeleton{Name: defaultSkinName}
	for _, cl := range clusters {
		jointIdx, ok := clusterToJoint[cl.id]
		if !ok {
			continue
		}
		skel.Joints = append(skel.Joints, jointIdx)
		skel.InverseBindMatrices = append(skel.InverseBindMatrices, cl.ibm)
		if jointIdx < len(asset.Nodes) {
			asset.Nodes[jointIdx].IsJoint = true
		}
	}

	if len(skel.Joints) > 0 {
		asset.Skeletons = append(asset.Skeletons, skel)
	}
}

func extractAnimTarget(node *fbxNode) string {
	name := extractName(node)
	switch {
	case strings.HasPrefix(name, animTargetT) || strings.Contains(name, animLongT):
		return animTargetT
	case strings.HasPrefix(name, animTargetR) || strings.Contains(name, animLongR):
		return animTargetR
	case strings.HasPrefix(name, animTargetS) || strings.Contains(name, animLongS):
		return animTargetS
	default:
		return name
	}
}

func extractShapePositions(node *fbxNode, id int64) fbxShape {
	sh := fbxShape{id: id, name: extractName(node)}
	for _, child := range node.children {
		if child.name == geoVertices && len(child.properties) > 0 && child.properties[0].arrF64 != nil {
			sh.positions = f64ToVec3(child.properties[0].arrF64)
		}
	}
	return sh
}

func resolveMorphTargets(asset *ir.Asset, conns []connection, shapes []fbxShape, geoMap map[int64]int) {
	if len(shapes) == 0 {
		return
	}

	parentOf := make(map[int64]int64, len(conns))
	for _, c := range conns {
		parentOf[c.childID] = c.parentID
	}

	for _, sh := range shapes {
		bscID, ok := parentOf[sh.id]
		if !ok {
			continue
		}
		bsID, ok := parentOf[bscID]
		if !ok {
			continue
		}
		geoID, ok := parentOf[bsID]
		if !ok {
			continue
		}
		meshIdx, ok := geoMap[geoID]
		if !ok || meshIdx >= len(asset.Meshes) {
			continue
		}
		mesh := asset.Meshes[meshIdx]
		mt := ir.MorphTarget{
			Name:      sh.name,
			Positions: sh.positions,
		}
		if len(mesh.Primitives) > 0 {
			mesh.Primitives[0].MorphTargets = append(mesh.Primitives[0].MorphTargets, mt)
		}
	}
}

const (
	animKeyAttrFlags  = "KeyAttrFlags"
	fbxInterpConstant = 0x2
	fbxInterpCubic    = 0x8
)

func extractAnimCurve(node *fbxNode, id int64) *fbxAnimCurve {
	var times []float32
	var values []float32
	interp := ir.InterpolationLinear

	for _, child := range node.children {
		switch child.name {
		case animKeyTime:
			if len(child.properties) > 0 && child.properties[0].arrI64 != nil {
				raw := child.properties[0].arrI64
				times = make([]float32, len(raw))
				for i, t := range raw {
					times[i] = float32(float64(t) / fbxKTimeScale)
				}
			}
		case animKeyValue:
			if len(child.properties) > 0 && child.properties[0].arrF64 != nil {
				raw := child.properties[0].arrF64
				values = make([]float32, len(raw))
				for i, v := range raw {
					values[i] = float32(v)
				}
			}
		case animKeyAttrFlags:
			if len(child.properties) > 0 && child.properties[0].arrI32 != nil {
				flags := child.properties[0].arrI32
				if len(flags) > 0 {
					interp = fbxFlagsToInterp(flags[0])
				}
			}
		}
	}

	if len(times) == 0 || len(values) == 0 {
		return nil
	}
	return &fbxAnimCurve{id: id, times: times, values: values, interp: interp}
}

func fbxFlagsToInterp(flag int32) ir.Interpolation {
	switch {
	case flag&fbxInterpConstant != 0 && flag&fbxInterpCubic == 0:
		return ir.InterpolationStep
	case flag&fbxInterpCubic != 0:
		return ir.InterpolationCubicSpline
	default:
		return ir.InterpolationLinear
	}
}

func resolveAnimations( //nolint:funlen // multi-take routing
	asset *ir.Asset, conns []connection,
	curveNodes []fbxAnimCurveNode, curves []fbxAnimCurve,
	modelMap map[int64]int,
	animStacks map[int64]int,
	animLayers map[int64]struct{},
) {
	if len(curveNodes) == 0 || len(curves) == 0 {
		return
	}

	cnMap := make(map[int64]int, len(curveNodes))
	for i, cn := range curveNodes {
		cnMap[cn.id] = i
	}

	curveMap := make(map[int64]int, len(curves))
	for i, c := range curves {
		curveMap[c.id] = i
	}

	cnToNode := make(map[int64]int, len(curveNodes))
	curveToCN := make(map[int64]int64, len(curves))
	layerToStack := make(map[int64]int)
	cnToLayer := make(map[int64]int64)
	for _, c := range conns {
		if _, ok := cnMap[c.childID]; ok {
			if nodeIdx, ok := modelMap[c.parentID]; ok {
				cnToNode[c.childID] = nodeIdx
			}
			if _, ok := animLayers[c.parentID]; ok {
				cnToLayer[c.childID] = c.parentID
			}
		}
		if _, ok := curveMap[c.childID]; ok {
			if _, ok := cnMap[c.parentID]; ok {
				curveToCN[c.childID] = c.parentID
			}
		}
		if _, ok := animLayers[c.childID]; ok {
			if stackIdx, ok := animStacks[c.parentID]; ok {
				layerToStack[c.childID] = stackIdx
			}
		}
	}

	channels := make(map[int64]*ir.AnimationChannel, len(curveNodes))
	cnToAnimIdx := make(map[int64]int, len(curveNodes))
	for curveID, cnID := range curveToCN {
		cnIdx, ok := cnMap[cnID]
		if !ok {
			continue
		}
		nodeIdx, ok := cnToNode[cnID]
		if !ok {
			continue
		}

		ch, exists := channels[cnID]
		if !exists {
			ch = &ir.AnimationChannel{
				NodeIndex:     nodeIdx,
				Target:        animTargetToIR(curveNodes[cnIdx].target),
				Interpolation: curves[curveMap[curveID]].interp,
			}
			channels[cnID] = ch
			animIdx := 0
			if layerID, ok := cnToLayer[cnID]; ok {
				if si, ok := layerToStack[layerID]; ok {
					animIdx = si
				}
			}
			cnToAnimIdx[cnID] = animIdx
		}

		crv := curves[curveMap[curveID]]
		if ch.Times == nil {
			ch.Times = crv.times
		}
		appendCurveValues(ch, crv.values)
	}

	if len(asset.Animations) == 0 {
		asset.Animations = append(asset.Animations, &ir.Animation{Name: defaultAnimName})
	}
	for cnID, ch := range channels {
		animIdx := cnToAnimIdx[cnID]
		if animIdx >= len(asset.Animations) {
			animIdx = 0
		}
		asset.Animations[animIdx].Channels = append(asset.Animations[animIdx].Channels, *ch)
	}

	for _, a := range asset.Animations {
		for i := range a.Channels {
			if n := len(a.Channels[i].Times); n > 0 {
				if t := float64(a.Channels[i].Times[n-1]); t > a.Duration {
					a.Duration = t
				}
			}
		}
	}
}

func animTargetToIR(target string) ir.ChannelTarget {
	switch target {
	case animTargetT:
		return ir.TargetTranslation
	case animTargetR:
		return ir.TargetRotation
	case animTargetS:
		return ir.TargetScale
	default:
		return ir.TargetTranslation
	}
}

func appendCurveValues(ch *ir.AnimationChannel, values []float32) {
	switch ch.Target {
	case ir.TargetTranslation, ir.TargetScale:
		if ch.Translations == nil {
			ch.Translations = make([][3]float32, len(values))
		}
		for i := range min(len(values), len(ch.Translations)) {
			ch.Translations[i][0] = values[i]
		}
	case ir.TargetRotation:
		if ch.Rotations == nil {
			ch.Rotations = make([][4]float32, len(values))
		}
		for i := range min(len(values), len(ch.Rotations)) {
			ch.Rotations[i][0] = values[i]
		}
	}
}
