package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

const defaultDeboneThreshold = 0.01

type deboneStep struct{}

func (s *deboneStep) Name() string      { return "Debone" }
func (s *deboneStep) Flag() core.PPFlag { return core.PPDebone }

func (s *deboneStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	threshold := opts.DeboneThreshold
	if threshold <= 0 {
		threshold = defaultDeboneThreshold
	}

	animTargets := buildAnimTargets(asset)

	for si := range asset.Skeletons {
		skel := asset.Skeletons[si]
		if skel == nil || len(skel.Joints) == 0 {
			continue
		}

		maxWeights := computeMaxBoneWeights(asset, skel)
		removable := identifyRemovableBones(skel, maxWeights, threshold, animTargets, asset)

		if countTrue(removable) == 0 {
			continue
		}

		parentMap := buildBoneParentMap(asset, skel)
		remapJointIndices(asset, skel, removable, parentMap)
		pruneSkeleton(skel, removable)
	}
	return asset, nil
}

func buildAnimTargets(asset *ir.Asset) map[int]bool {
	targets := make(map[int]bool, len(asset.Animations)*4) //nolint:mnd // rough estimate
	for _, anim := range asset.Animations {
		if anim == nil {
			continue
		}
		for ci := range anim.Channels {
			targets[anim.Channels[ci].NodeIndex] = true
		}
	}
	return targets
}

func computeMaxBoneWeights(asset *ir.Asset, skel *ir.Skeleton) []float32 {
	maxW := make([]float32, len(skel.Joints))
	jointSet := make(map[int]int, len(skel.Joints))
	for li, ni := range skel.Joints {
		jointSet[ni] = li
	}

	const vec4Len = 4
	for _, mesh := range asset.Meshes {
		if mesh == nil {
			continue
		}
		for pi := range mesh.Primitives {
			d := &mesh.Primitives[pi].Data
			for vi := range d.Weights0 {
				for k := range vec4Len {
					if d.Weights0[vi][k] > 0 {
						ji := int(d.Joints0[vi][k])
						if li, ok := jointSet[ji]; ok && d.Weights0[vi][k] > maxW[li] {
							maxW[li] = d.Weights0[vi][k]
						}
					}
				}
				if vi < len(d.Weights1) {
					for k := range vec4Len {
						if d.Weights1[vi][k] > 0 {
							ji := int(d.Joints1[vi][k])
							if li, ok := jointSet[ji]; ok && d.Weights1[vi][k] > maxW[li] {
								maxW[li] = d.Weights1[vi][k]
							}
						}
					}
				}
			}
		}
	}
	return maxW
}

func identifyRemovableBones(
	skel *ir.Skeleton,
	maxWeights []float32,
	threshold float32,
	animTargets map[int]bool,
	asset *ir.Asset,
) []bool {
	removable := make([]bool, len(skel.Joints))
	rootNodeIdx := ir.NoIndex
	if skel.RootIdx >= 0 && skel.RootIdx < len(skel.Joints) {
		rootNodeIdx = skel.Joints[skel.RootIdx]
	}

	for li, nodeIdx := range skel.Joints {
		if nodeIdx == rootNodeIdx {
			continue
		}
		if animTargets[nodeIdx] {
			continue
		}
		if nodeIdx >= 0 && nodeIdx < len(asset.Nodes) && len(asset.Nodes[nodeIdx].Children) > 0 {
			continue
		}
		if maxWeights[li] < threshold {
			removable[li] = true
		}
	}
	return removable
}

func buildBoneParentMap(asset *ir.Asset, skel *ir.Skeleton) map[int]int {
	parentMap := make(map[int]int, len(skel.Joints))
	for _, nodeIdx := range skel.Joints {
		parentMap[nodeIdx] = ir.NoIndex
	}
	for _, nodeIdx := range skel.Joints {
		if nodeIdx < 0 || nodeIdx >= len(asset.Nodes) {
			continue
		}
		for _, childIdx := range asset.Nodes[nodeIdx].Children {
			if _, ok := parentMap[childIdx]; ok {
				parentMap[childIdx] = nodeIdx
			}
		}
	}
	return parentMap
}

func remapJointIndices(asset *ir.Asset, skel *ir.Skeleton, removable []bool, parentMap map[int]int) {
	remap := make(map[int]int, len(skel.Joints))
	for li, nodeIdx := range skel.Joints {
		if removable[li] {
			target := parentMap[nodeIdx]
			for target != ir.NoIndex {
				found := false
				for tli, tni := range skel.Joints {
					if tni == target && !removable[tli] {
						remap[nodeIdx] = target
						found = true
						break
					}
				}
				if found {
					break
				}
				target = parentMap[target]
			}
			if _, ok := remap[nodeIdx]; !ok {
				remap[nodeIdx] = skel.Joints[skel.RootIdx]
			}
		} else {
			remap[nodeIdx] = nodeIdx
		}
	}

	const vec4Len = 4
	for _, mesh := range asset.Meshes {
		if mesh == nil {
			continue
		}
		for pi := range mesh.Primitives {
			d := &mesh.Primitives[pi].Data
			for vi := range d.Joints0 {
				for k := range vec4Len {
					ji := int(d.Joints0[vi][k])
					if target, ok := remap[ji]; ok {
						d.Joints0[vi][k] = uint16(target) //nolint:gosec // joint index fits
					}
				}
			}
			for vi := range d.Joints1 {
				for k := range vec4Len {
					ji := int(d.Joints1[vi][k])
					if target, ok := remap[ji]; ok {
						d.Joints1[vi][k] = uint16(target) //nolint:gosec // joint index fits
					}
				}
			}
		}
	}
}

func pruneSkeleton(skel *ir.Skeleton, removable []bool) {
	newJoints := make([]int, 0, len(skel.Joints))
	newIBMs := make([][16]float32, 0, len(skel.InverseBindMatrices))

	newRootIdx := 0
	for li, nodeIdx := range skel.Joints {
		if removable[li] {
			continue
		}
		if li == skel.RootIdx {
			newRootIdx = len(newJoints)
		}
		newJoints = append(newJoints, nodeIdx)
		if li < len(skel.InverseBindMatrices) {
			newIBMs = append(newIBMs, skel.InverseBindMatrices[li])
		}
	}

	skel.Joints = newJoints
	skel.InverseBindMatrices = newIBMs
	skel.RootIdx = newRootIdx
}

func countTrue(bs []bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}
