package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type globalScaleStep struct{}

func (s *globalScaleStep) Name() string      { return "GlobalScale" }
func (s *globalScaleStep) Flag() core.PPFlag { return core.PPGlobalScale }

func (s *globalScaleStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	scale := opts.GlobalScale
	if scale == 0 || scale == 1.0 {
		return asset, nil
	}

	asset.Unit = float64(asset.Unit) * scale

	for i := range asset.Nodes {
		t := &asset.Nodes[i].Transform
		t.Translation[0] *= float32(scale)
		t.Translation[1] *= float32(scale)
		t.Translation[2] *= float32(scale)
	}

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			for k := range p.Data.Positions {
				p.Data.Positions[k][0] *= float32(scale)
				p.Data.Positions[k][1] *= float32(scale)
				p.Data.Positions[k][2] *= float32(scale)
			}
		}

		mesh.BoundingBox[0][0] *= float32(scale)
		mesh.BoundingBox[0][1] *= float32(scale)
		mesh.BoundingBox[0][2] *= float32(scale)
		mesh.BoundingBox[1][0] *= float32(scale)
		mesh.BoundingBox[1][1] *= float32(scale)
		mesh.BoundingBox[1][2] *= float32(scale)
	}

	return asset, nil
}
