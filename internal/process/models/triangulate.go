package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type triangulateStep struct{}

func (s *triangulateStep) Name() string      { return "Triangulate" }
func (s *triangulateStep) Flag() core.PPFlag { return core.PPTriangulate }

func (s *triangulateStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}

		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]

			if p.Mode == ir.Triangles || p.Mode == ir.Lines || p.Mode == ir.LineLoop || p.Mode == ir.Points {
				continue
			}

			const minTriVerts = 3
			if len(p.Data.Indices) < minTriVerts {
				continue
			}

			var newIndices []uint32

			switch p.Mode {
			case ir.TriangleFan:
				newIndices = make([]uint32, 0, (len(p.Data.Indices)-2)*3) //nolint:mnd // fan→tri expansion
				for k := 2; k < len(p.Data.Indices); k++ {
					newIndices = append(newIndices, p.Data.Indices[0], p.Data.Indices[k-1], p.Data.Indices[k])
				}
			case ir.TriangleStrip:
				newIndices = make([]uint32, 0, (len(p.Data.Indices)-2)*3) //nolint:mnd // strip→tri expansion
				for k := 2; k < len(p.Data.Indices); k++ {
					if k%2 == 1 {
						newIndices = append(newIndices, p.Data.Indices[k-2], p.Data.Indices[k], p.Data.Indices[k-1])
					} else {
						newIndices = append(newIndices, p.Data.Indices[k-2], p.Data.Indices[k-1], p.Data.Indices[k])
					}
				}
			default:
				newIndices = make([]uint32, 0, (len(p.Data.Indices)-2)*3) //nolint:mnd // default→tri expansion
				for k := 2; k < len(p.Data.Indices); k++ {
					newIndices = append(newIndices, p.Data.Indices[0], p.Data.Indices[k-1], p.Data.Indices[k])
				}
			}

			p.Data.Indices = newIndices
			p.Mode = ir.Triangles
		}
	}
	return asset, nil
}
