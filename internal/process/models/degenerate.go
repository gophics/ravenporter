package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type removeDegeneratesStep struct{}

func (s *removeDegeneratesStep) Name() string      { return "RemoveDegenerates" }
func (s *removeDegeneratesStep) Flag() core.PPFlag { return core.PPRemoveDegenerates }

func (s *removeDegeneratesStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if p.Mode == ir.Triangles && p.Data.HasIndices() {
				tris := make([]uint32, 0, len(p.Data.Indices))
				var lines []uint32
				var points []uint32

				idx := p.Data.Indices
				for k := 0; k+2 < len(idx); k += 3 {
					i0, i1, i2 := idx[k], idx[k+1], idx[k+2]

					if i0 == i1 && i1 == i2 {
						points = append(points, i0)
					} else if i0 == i1 {
						lines = append(lines, i1, i2)
					} else if i1 == i2 {
						lines = append(lines, i0, i1)
					} else if i2 == i0 {
						lines = append(lines, i0, i1)
					} else {
						tris = append(tris, i0, i1, i2)
					}
				}

				switch opts.DegenerateMode {
				case core.DegenerateModeRemove:
					p.Data.Indices = tris
				case core.DegenerateModeConvert:
					if len(tris) > 0 {
						p.Data.Indices = tris
					} else if len(lines) > 0 {
						p.Data.Indices = lines
						p.Mode = ir.Lines
					} else if len(points) > 0 {
						p.Data.Indices = points
						p.Mode = ir.Points
					}
				}
			}
		}
	}
	return asset, nil
}
