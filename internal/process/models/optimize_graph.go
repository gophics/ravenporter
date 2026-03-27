package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type optimizeGraphStep struct{}

func (s *optimizeGraphStep) Name() string      { return "OptimizeGraph" }
func (s *optimizeGraphStep) Flag() core.PPFlag { return core.PPOptimizeGraph }

// optimizeGraphStep applies tree collapse rules
//
//nolint:gocyclo,gocritic,funlen // Tree traversal natively increases cyclomatic scores
func (s *optimizeGraphStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	if len(asset.Nodes) == 0 {
		return asset, nil
	}

	protected := make(map[int]bool)
	for _, anim := range asset.Animations {
		for _, ch := range anim.Channels {
			protected[ch.NodeIndex] = true
		}
	}

	for _, skel := range asset.Skeletons {
		for _, j := range skel.Joints {
			protected[j] = true
		}
	}

	canRemove := func(idx int, n *ir.Node) bool {
		return n.MeshIndex == ir.NoIndex &&
			n.SkinIndex == ir.NoIndex &&
			n.CameraIndex == ir.NoIndex &&
			n.LightIndex == ir.NoIndex &&
			!n.IsJoint &&
			len(n.Children) == 0 &&
			!protected[idx]
	}

	changed := true
	removeSet := make(map[int]bool, len(asset.Nodes))
	for changed {
		changed = false
		clear(removeSet)
		for i := range asset.Nodes {
			if canRemove(i, &asset.Nodes[i]) {
				removeSet[i] = true
				changed = true
			}
		}

		if !changed {
			break
		}

		remap := make(map[int]int, len(asset.Nodes))
		newNodes := make([]ir.Node, 0, len(asset.Nodes))

		for i := range asset.Nodes {
			if !removeSet[i] {
				remap[i] = len(newNodes)
				newNodes = append(newNodes, asset.Nodes[i])
			} else {
				remap[i] = ir.NoIndex
			}
		}

		for i := range newNodes {
			newChildren := make([]int, 0, len(newNodes[i].Children))
			for _, childIdx := range newNodes[i].Children {
				if mapped := remap[childIdx]; mapped != ir.NoIndex {
					newChildren = append(newChildren, mapped)
				}
			}
			newNodes[i].Children = newChildren
		}

		currentRoots := asset.PrimaryRootNodes()
		newRoots := make([]int, 0, len(currentRoots))
		for _, rootIdx := range currentRoots {
			if mapped := remap[rootIdx]; mapped != ir.NoIndex {
				newRoots = append(newRoots, mapped)
			}
		}

		for i := range asset.Skeletons {
			skel := asset.Skeletons[i]
			if skel == nil {
				continue
			}
			newJoints := make([]int, 0, len(skel.Joints))
			for _, jIdx := range skel.Joints {
				if mapped := remap[jIdx]; mapped != ir.NoIndex {
					newJoints = append(newJoints, mapped)
				}
			}
			skel.Joints = newJoints
		}

		for i := range asset.Animations {
			anim := asset.Animations[i]
			if anim == nil {
				continue
			}
			newChans := make([]ir.AnimationChannel, 0, len(anim.Channels))
			for _, ch := range anim.Channels {
				if mapped := remap[ch.NodeIndex]; mapped != ir.NoIndex {
					ch.NodeIndex = mapped
					newChans = append(newChans, ch)
				}
			}
			anim.Channels = newChans
		}

		asset.Nodes = newNodes
		asset.RootNodes = newRoots
		if scene := asset.PrimaryScene(); len(asset.Scenes) > 0 && scene != nil {
			scene.RootNodes = append(scene.RootNodes[:0], newRoots...)
		}
	}

	return asset, nil
}
