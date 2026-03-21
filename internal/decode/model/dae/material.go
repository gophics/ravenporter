package dae

import (
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/ir"
)

const (
	colorComponents  = 4
	propSpecular     = "specular"
	propAmbient      = "ambient"
	defaultShininess = 100.0
)

func buildEffectMap(effects []effect) map[string]*shading {
	m := make(map[string]*shading, len(effects))
	for _, e := range effects {
		if len(e.Profiles) == 0 {
			continue
		}
		t := &e.Profiles[0].Technique
		switch {
		case t.Phong != nil:
			m["#"+e.ID] = t.Phong
		case t.Lambert != nil:
			m["#"+e.ID] = t.Lambert
		case t.Blinn != nil:
			m["#"+e.ID] = t.Blinn
		}
	}
	return m
}

func convertMaterials(
	mats []xmlMaterial, effectMap map[string]*shading, imageMap map[string]string, asset *ir.Asset,
) []*ir.Material {
	out := make([]*ir.Material, 0, len(mats))
	for _, m := range mats {
		shd := effectMap[m.Inst.URL]
		mat := &ir.Material{
			Name:            m.Name,
			MetallicFactor:  0,
			RoughnessFactor: 1,
			AlphaMode:       ir.AlphaOpaque,
		}
		if shd != nil {
			mat.BaseColorFactor = parseColor(shd.Diffuse.Color)
			mat.EmissiveFactor = parseColor3(shd.Emission.Color)

			if shd.Specular.Color != "" {
				if mat.Properties == nil {
					mat.Properties = make(map[string]any)
				}
				mat.Properties[propSpecular] = parseColor3(shd.Specular.Color)
			}
			if shd.Ambient.Color != "" {
				if mat.Properties == nil {
					mat.Properties = make(map[string]any)
				}
				mat.Properties[propAmbient] = parseColor3(shd.Ambient.Color)
			}
			if shd.Shininess.Float > 0 {
				mat.RoughnessFactor = max(0, 1.0-float32(shd.Shininess.Float)/defaultShininess)
			}

			resolveTexture(&shd.Diffuse, imageMap, asset, func(ref *ir.TextureRef) {
				mat.BaseColorTexture = ref
			})
			resolveTexture(&shd.Emission, imageMap, asset, func(ref *ir.TextureRef) {
				mat.EmissiveTexture = ref
			})
		}
		out = append(out, mat)
	}
	return out
}

func resolveTexture(ct *colorOrTexture, imageMap map[string]string, asset *ir.Asset, assign func(*ir.TextureRef)) {
	if ct.Texture.Texture == "" {
		return
	}
	path := imageMap[ct.Texture.Texture]
	if path == "" {
		return
	}
	imageIndex := len(asset.Images)
	asset.Images = append(asset.Images, &ir.ImageAsset{
		Name:       ct.Texture.Texture,
		SourcePath: path,
	})
	texIdx := len(asset.Textures)
	asset.Textures = append(asset.Textures, &ir.Texture{
		Name:       ct.Texture.Texture,
		ImageIndex: imageIndex,
	})
	assign(&ir.TextureRef{TextureIndex: texIdx, Tiling: [2]float32{1, 1}})
}

func parseColor(s string) [4]float32 {
	fields := strings.Fields(s)
	var c [4]float32
	for i := range min(len(fields), colorComponents) {
		v, _ := strconv.ParseFloat(fields[i], 32) //nolint:errcheck // 0.0 on failure
		c[i] = float32(v)                         //nolint:gosec // i bounded by colorComponents
	}
	if len(fields) < colorComponents {
		c[3] = 1
	}
	return c
}

func parseColor3(s string) [3]float32 {
	fields := strings.Fields(s)
	var c [3]float32
	for i := range min(len(fields), 3) { //nolint:mnd // RGB
		v, _ := strconv.ParseFloat(fields[i], 32) //nolint:errcheck // 0.0 on failure
		c[i] = float32(v)                         //nolint:gosec // i bounded by 3
	}
	return c
}

func buildMaterialNameMap(mats []xmlMaterial, irMats []*ir.Material) map[string]int {
	m := make(map[string]int, len(mats))
	for i, mat := range mats {
		if i < len(irMats) {
			m[mat.ID] = i
			m[mat.Name] = i
		}
	}
	return m
}

func buildImageMap(images []xmlImage) map[string]string {
	m := make(map[string]string, len(images))
	for _, img := range images {
		m[img.ID] = strings.TrimSpace(img.Init.Path)
	}
	return m
}

func resolveMaterialSymbol(symbol string, matNameMap map[string]int) int {
	if idx, ok := matNameMap[symbol]; ok {
		return idx
	}
	return ir.NoIndex
}
