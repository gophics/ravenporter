package models

import (
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type flattenHierarchyStep struct{}

func (s *flattenHierarchyStep) Name() string      { return "FlattenHierarchy" }
func (s *flattenHierarchyStep) Flag() core.PPFlag { return core.PPFlattenHierarchy }

func (s *flattenHierarchyStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	worldTrans := make(map[int][3]float32, len(asset.Nodes))
	var accumulate func(idx int, parentT [3]float32)
	accumulate = func(idx int, parentT [3]float32) {
		n := &asset.Nodes[idx]
		wt := [3]float32{
			parentT[0] + n.Transform.Translation[0],
			parentT[1] + n.Transform.Translation[1],
			parentT[2] + n.Transform.Translation[2],
		}
		worldTrans[idx] = wt
		for _, child := range n.Children {
			accumulate(child, wt)
		}
	}
	for _, root := range asset.PrimaryRootNodes() {
		accumulate(root, [3]float32{})
	}

	var newNodes []ir.Node
	if len(asset.Nodes) > 0 {
		newNodes = make([]ir.Node, 0, len(asset.Nodes))
	}
	asset.WalkNodes(0, func(idx int, n *ir.Node) bool {
		baked := *n
		wt := worldTrans[idx]

		if n.MeshIndex != ir.NoIndex && n.SkinIndex == ir.NoIndex {
			mesh := asset.Meshes[n.MeshIndex]
			for j := range mesh.Primitives {
				p := &mesh.Primitives[j]
				for k, pos := range p.Data.Positions {
					p.Data.Positions[k] = [3]float32{pos[0] + wt[0], pos[1] + wt[1], pos[2] + wt[2]}
				}
			}
		}

		baked.Transform.Matrix = mathx.IdentityMat4
		baked.Transform.Translation = [3]float32{}
		baked.Transform.Rotation = [4]float32{}
		baked.Transform.Scale = [3]float32{}
		baked.Children = nil
		newNodes = append(newNodes, baked)
		return true
	})

	asset.Nodes = newNodes
	newRoots := make([]int, len(newNodes))
	for i := range newNodes {
		newRoots[i] = i
	}
	asset.RootNodes = newRoots
	if scene := asset.PrimaryScene(); len(asset.Scenes) > 0 && scene != nil {
		scene.RootNodes = append(scene.RootNodes[:0], newRoots...)
	}

	return asset, nil
}
