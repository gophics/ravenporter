package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type removeRedundantMaterialsStep struct{}

func (s *removeRedundantMaterialsStep) Name() string      { return "RemoveRedundantMaterials" }
func (s *removeRedundantMaterialsStep) Flag() core.PPFlag { return core.PPRemoveRedundantMaterials }

func (s *removeRedundantMaterialsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	if len(asset.Materials) == 0 {
		return asset, nil
	}

	used := make([]bool, len(asset.Materials))
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			idx := mesh.Primitives[j].MaterialIndex
			if idx >= 0 && idx < len(used) {
				used[idx] = true
			}
		}
	}

	newMats := make([]*ir.Material, 0, len(asset.Materials))
	remap := make(map[int]int, len(asset.Materials))

	for i, inUse := range used {
		if inUse {
			remap[i] = len(newMats)
			newMats = append(newMats, asset.Materials[i])
		} else {
			remap[i] = ir.NoIndex
		}
	}

	if len(newMats) == len(asset.Materials) {
		return asset, nil
	}

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if p.MaterialIndex >= 0 && p.MaterialIndex < len(used) {
				p.MaterialIndex = remap[p.MaterialIndex]
			}
		}
	}

	asset.Materials = newMats
	return asset, nil
}
