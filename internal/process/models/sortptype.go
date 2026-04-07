package models

import (
	"cmp"
	"slices"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type sortPTypeStep struct{}

func (s *sortPTypeStep) Name() string      { return "SortByPType" }
func (s *sortPTypeStep) Flag() core.PPFlag { return core.PPSortByPtype }

func (s *sortPTypeStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}

		slices.SortStableFunc(mesh.Primitives, func(a, b ir.Primitive) int {
			return cmp.Compare(a.Mode, b.Mode)
		})
	}
	return asset, nil
}
