package models

import (
	"math"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type genUVCoordsStep struct{}

func (s *genUVCoordsStep) Name() string      { return "GenUVCoords" }
func (s *genUVCoordsStep) Flag() core.PPFlag { return core.PPGenUVCoords }

func (s *genUVCoordsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if len(p.Data.Positions) == 0 {
				continue
			}
			if len(p.Data.TexCoord0) == 0 {
				p.Data.TexCoord0 = make([][2]float32, len(p.Data.Positions))
				for k, pos := range p.Data.Positions {
					p.Data.TexCoord0[k] = [2]float32{pos[0], pos[1]}
				}
			}
		}
	}
	return asset, nil
}

type transformUVCoordsStep struct{}

func (s *transformUVCoordsStep) Name() string      { return "TransformUVCoords" }
func (s *transformUVCoordsStep) Flag() core.PPFlag { return core.PPTransformUVCoords }

func (s *transformUVCoordsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if len(p.Data.TexCoord0) == 0 {
				continue
			}

			ref := findTextureRef(asset, p.MaterialIndex)
			if ref != nil && hasTransform(ref) {
				bakeUVTransform(p.Data.TexCoord0, ref)
				ref.Offset = [2]float32{0, 0}
				ref.Tiling = [2]float32{1, 1}
				ref.Rotation = 0
			}

			for k, uv := range p.Data.TexCoord0 {
				u := float64(uv[0])
				v := float64(uv[1])
				p.Data.TexCoord0[k] = [2]float32{
					float32(u - math.Floor(u)),
					float32(v - math.Floor(v)),
				}
			}
		}
	}
	return asset, nil
}

func findTextureRef(asset *ir.Asset, matIdx int) *ir.TextureRef {
	if matIdx < 0 || matIdx >= len(asset.Materials) || asset.Materials[matIdx] == nil {
		return nil
	}
	return asset.Materials[matIdx].BaseColorTexture
}

func hasTransform(ref *ir.TextureRef) bool {
	if ref.Rotation != 0 {
		return true
	}
	if ref.Offset[0] != 0 || ref.Offset[1] != 0 {
		return true
	}
	if ref.Tiling[0] != 1 || ref.Tiling[1] != 1 {
		if ref.Tiling[0] != 0 || ref.Tiling[1] != 0 {
			return true
		}
	}
	return false
}

func bakeUVTransform(uvs [][2]float32, ref *ir.TextureRef) {
	cosR := float32(math.Cos(float64(ref.Rotation)))
	sinR := float32(math.Sin(float64(ref.Rotation)))
	tx, ty := ref.Tiling[0], ref.Tiling[1]
	ox, oy := ref.Offset[0], ref.Offset[1]

	if tx == 0 {
		tx = 1
	}
	if ty == 0 {
		ty = 1
	}

	for k, uv := range uvs {
		su := uv[0] * tx
		sv := uv[1] * ty
		uvs[k] = [2]float32{
			cosR*su - sinR*sv + ox,
			sinR*su + cosR*sv + oy,
		}
	}
}
