package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

const (
	propDiffuseColor    = "DiffuseColor"
	propSpecularPower   = "SpecularPower"
	propSpecularColor   = "SpecularColor"
	propSpecularFactor  = "specularFactor"
	propGlossiness      = "glossinessFactor"
	propRoughnessLegacy = "roughnessFactor"
	propPBRConverted    = "_isPBRConverted"
)

type convertToPBRStep struct{}

func (s *convertToPBRStep) Name() string      { return "ConvertToPBR" }
func (s *convertToPBRStep) Flag() core.PPFlag { return core.PPConvertToPBR }

func (s *convertToPBRStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Materials {
		mat := asset.Materials[i]
		if mat == nil {
			continue
		}

		if isPBRMaterial(mat) {
			continue
		}

		if diff, ok := mat.Properties[propDiffuseColor].([3]float32); ok {
			mat.BaseColorFactor = [4]float32{diff[0], diff[1], diff[2], 1.0}
			delete(mat.Properties, propDiffuseColor)
		}

		if power, ok := mat.Properties[propSpecularPower].(float32); ok {
			mat.RoughnessFactor = float32(1.0 - float64(power)/128.0)
			if mat.RoughnessFactor < 0 {
				mat.RoughnessFactor = 0
			}
			delete(mat.Properties, propSpecularPower)
		}

		const rgbChannels = 3.0
		if specCol, ok := mat.Properties[propSpecularColor].([3]float32); ok {
			mat.MetallicFactor = (specCol[0] + specCol[1] + specCol[2]) / rgbChannels
			delete(mat.Properties, propSpecularColor)
		}

		if spec, ok := mat.Properties[propSpecularFactor].([3]float32); ok {
			mat.MetallicFactor = (spec[0] + spec[1] + spec[2]) / rgbChannels
			delete(mat.Properties, propSpecularFactor)
		}

		if gloss, ok := mat.Properties[propGlossiness].(float32); ok {
			mat.RoughnessFactor = 1.0 - gloss
			delete(mat.Properties, propGlossiness)
		} else if rough, ok := mat.Properties[propRoughnessLegacy].(float32); ok {
			mat.RoughnessFactor = rough
		}

		if mat.Properties == nil {
			mat.Properties = make(map[string]any)
		}
		mat.Properties[propPBRConverted] = true
	}
	return asset, nil
}

var legacyKeys = [...]string{
	propSpecularFactor, propGlossiness,
	propDiffuseColor, propSpecularPower, propSpecularColor,
}

func isPBRMaterial(mat *ir.Material) bool {
	if len(mat.Properties) == 0 {
		return true
	}
	if _, ok := mat.Properties[propPBRConverted]; ok {
		return true
	}
	for _, key := range legacyKeys {
		if _, ok := mat.Properties[key]; ok {
			return false
		}
	}
	return true
}
