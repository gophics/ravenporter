package models

import (
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type flipWindingOrderStep struct{}

func (s *flipWindingOrderStep) Name() string      { return "FlipWindingOrder" }
func (s *flipWindingOrderStep) Flag() core.PPFlag { return core.PPFlipWindingOrder }

func (s *flipWindingOrderStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if p.Mode == ir.Triangles && p.Data.HasIndices() {
				indices := p.Data.Indices
				for k := 0; k+2 < len(indices); k += 3 {
					indices[k+1], indices[k+2] = indices[k+2], indices[k+1]
				}
			}
		}
	}
	return asset, nil
}

type fixWindingCCWStep struct{}

func (s *fixWindingCCWStep) Name() string      { return "FixWindingCCW" }
func (s *fixWindingCCWStep) Flag() core.PPFlag { return core.PPFixWinding }

func (s *fixWindingCCWStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if p.Mode == ir.Triangles && p.Data.HasIndices() && len(p.Data.Normals) == len(p.Data.Positions) {
				indices := p.Data.Indices
				for k := 0; k+2 < len(indices); k += 3 {
					i0, i1, i2 := indices[k], indices[k+1], indices[k+2]
					v0 := p.Data.Positions[i0]
					v1 := p.Data.Positions[i1]
					v2 := p.Data.Positions[i2]
					geoN := mathx.Cross3(mathx.Sub3(v1, v0), mathx.Sub3(v2, v0))
					geoN = mathx.Normalize3(geoN)

					n0 := p.Data.Normals[i0]
					n1 := p.Data.Normals[i1]
					n2 := p.Data.Normals[i2]

					avgN := [3]float32{
						(n0[0] + n1[0] + n2[0]) / 3.0,
						(n0[1] + n1[1] + n2[1]) / 3.0,
						(n0[2] + n1[2] + n2[2]) / 3.0,
					}
					avgN = mathx.Normalize3(avgN)

					dot := avgN[0]*geoN[0] + avgN[1]*geoN[1] + avgN[2]*geoN[2]
					if dot < 0 {
						indices[k+1], indices[k+2] = indices[k+2], indices[k+1]
					}
				}
			}
		}
	}
	return asset, nil
}
