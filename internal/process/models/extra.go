package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type preTransformStep struct{}

func (s *preTransformStep) Name() string      { return "PreTransform" }
func (s *preTransformStep) Flag() core.PPFlag { return core.PPPreTransform }

func (s *preTransformStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	asset.WalkNodes(0, func(_ int, n *ir.Node) bool {
		if n.MeshIndex != ir.NoIndex {
			mesh := asset.Meshes[n.MeshIndex]
			bakeTransform(mesh, n.Transform)

			n.Transform = ir.Transform{
				Scale:    [3]float32{1, 1, 1},
				Rotation: [4]float32{0, 0, 0, 1},
			}
		}
		return true
	})
	return asset, nil
}

func bakeTransform(mesh *ir.Mesh, t ir.Transform) {
	for pi := range mesh.Primitives {
		p := &mesh.Primitives[pi].Data
		for vi := range p.Positions {
			p.Positions[vi][0] += t.Translation[0]
			p.Positions[vi][1] += t.Translation[1]
			p.Positions[vi][2] += t.Translation[2]
		}
	}
}
