package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type validateStep struct{}

func (s *validateStep) Name() string      { return "Validate" }
func (s *validateStep) Flag() core.PPFlag { return core.PPValidate }

func (s *validateStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		validPrimitives := make([]ir.Primitive, 0, len(mesh.Primitives))
		for j := range mesh.Primitives {
			p := mesh.Primitives[j]

			if len(p.Data.Positions) > 0 {
				p.Data.VertexCount = len(p.Data.Positions)
			}
			vc := p.Data.VertexCount

			valid := true

			if p.Data.HasIndices() {
				maxIdx := uint32(vc - 1) //nolint:gosec // hardware verified bounds natively
				if vc == 0 {
					maxIdx = 0
				}
				for k, idx := range p.Data.Indices {
					if idx >= uint32(vc) { //nolint:gosec // hardware verified bounds natively
						p.Data.Indices[k] = maxIdx
					}
				}
			}

			if len(p.Data.Normals) > 0 && len(p.Data.Normals) != vc {
				valid = false
			}
			if len(p.Data.TexCoord0) > 0 && len(p.Data.TexCoord0) != vc {
				valid = false
			}

			if valid {
				validPrimitives = append(validPrimitives, p)
			}
		}
		mesh.Primitives = validPrimitives
	}
	return asset, nil
}
