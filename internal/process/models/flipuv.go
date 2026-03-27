package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type flipUVsStep struct{}

func (s *flipUVsStep) Name() string      { return "FlipUVs" }
func (s *flipUVsStep) Flag() core.PPFlag { return core.PPFlipUVs }

func (s *flipUVsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			for k := range p.Data.TexCoord0 {
				p.Data.TexCoord0[k][1] = 1.0 - p.Data.TexCoord0[k][1]
			}
			for k := range p.Data.TexCoord1 {
				p.Data.TexCoord1[k][1] = 1.0 - p.Data.TexCoord1[k][1]
			}
		}
	}
	return asset, nil
}
