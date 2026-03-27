package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

const defaultMaterialName = "DefaultMaterial"

type validateMaterialsStep struct{}

func (s *validateMaterialsStep) Name() string      { return "ValidateMaterials" }
func (s *validateMaterialsStep) Flag() core.PPFlag { return core.PPValidateMaterials }

func (s *validateMaterialsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	var defaultIndex = ir.NoIndex

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if p.MaterialIndex < 0 || p.MaterialIndex >= len(asset.Materials) || asset.Materials[p.MaterialIndex] == nil {
				if defaultIndex == ir.NoIndex {
					defaultMat := &ir.Material{
						Name:            defaultMaterialName,
						BaseColorFactor: [4]float32{1, 1, 1, 1},
						MetallicFactor:  0.0,
						RoughnessFactor: 0.5,
						DoubleSided:     false,
						AlphaMode:       ir.AlphaOpaque,
					}
					defaultIndex = len(asset.Materials)
					asset.Materials = append(asset.Materials, defaultMat)
				}
				p.MaterialIndex = defaultIndex
			}
		}
	}

	for k := range asset.Materials {
		mat := asset.Materials[k]
		if mat == nil {
			continue
		}
		if mat.RoughnessFactor < 0 {
			mat.RoughnessFactor = 0
		} else if mat.RoughnessFactor > 1 {
			mat.RoughnessFactor = 1
		}

		if mat.MetallicFactor < 0 {
			mat.MetallicFactor = 0
		} else if mat.MetallicFactor > 1 {
			mat.MetallicFactor = 1
		}
	}

	return asset, nil
}
