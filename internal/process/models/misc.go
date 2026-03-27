package models

import (
	"math"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type removeComponentStep struct{}

func (s *removeComponentStep) Name() string      { return "RemoveComponent" }
func (s *removeComponentStep) Flag() core.PPFlag { return core.PPRemoveComponent }
func (s *removeComponentStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	flags := opts.RemoveFlags
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if flags&core.CompNormals != 0 {
				p.Data.Normals = nil
			}
			if flags&core.CompTangents != 0 {
				p.Data.Tangents = nil
			}
			if flags&core.CompTexCoord0 != 0 {
				p.Data.TexCoord0 = nil
			}
			if flags&core.CompTexCoord1 != 0 {
				p.Data.TexCoord1 = nil
			}
			if flags&core.CompColors0 != 0 {
				p.Data.Colors0 = nil
			}
			if flags&core.CompJoints != 0 {
				p.Data.Joints0, p.Data.Joints1 = nil, nil
			}
			if flags&core.CompWeights != 0 {
				p.Data.Weights0, p.Data.Weights1 = nil, nil
			}
		}
	}
	return asset, nil
}

type findInvalidStep struct{}

func (s *findInvalidStep) Name() string      { return "FindInvalid" }
func (s *findInvalidStep) Flag() core.PPFlag { return core.PPFindInvalid }
func (s *findInvalidStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			for k, pos := range p.Data.Positions {
				if math.IsNaN(float64(pos[0])) || math.IsNaN(float64(pos[1])) || math.IsNaN(float64(pos[2])) {
					p.Data.Positions[k] = [3]float32{0, 0, 0}
				}
			}
			for k, norm := range p.Data.Normals {
				//nolint:lll // Mathematical expression is cleaner unified
				if math.IsNaN(float64(norm[0])) || math.IsNaN(float64(norm[1])) || math.IsNaN(float64(norm[2])) || (norm[0] == 0 && norm[1] == 0 && norm[2] == 0) {
					p.Data.Normals[k] = [3]float32{0, 0, 1}
				}
			}
			for k, tan := range p.Data.Tangents {
				//nolint:lll // Handled unified logic
				if math.IsNaN(float64(tan[0])) || math.IsNaN(float64(tan[1])) || math.IsNaN(float64(tan[2])) || (tan[0] == 0 && tan[1] == 0 && tan[2] == 0) {
					p.Data.Tangents[k] = [4]float32{1, 0, 0, 1}
				}
			}
		}
	}
	return asset, nil
}

type optimizeMeshesStep struct{}

func (s *optimizeMeshesStep) Name() string      { return "OptimizeMeshes" }
func (s *optimizeMeshesStep) Flag() core.PPFlag { return core.PPOptimizeMeshes }
func (s *optimizeMeshesStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}

		grouped := make(map[int]int, len(mesh.Primitives))

		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if existIdx, ok := grouped[p.MaterialIndex]; ok && mesh.Primitives[existIdx].Mode == p.Mode {
				existing := &mesh.Primitives[existIdx]
				baseV := uint32(existing.Data.VertexCount) //nolint:gosec // Handled bounding size natively

				existing.Data.Positions = append(existing.Data.Positions, p.Data.Positions...)
				existing.Data.Normals = append(existing.Data.Normals, p.Data.Normals...)
				existing.Data.Tangents = append(existing.Data.Tangents, p.Data.Tangents...)
				existing.Data.TexCoord0 = append(existing.Data.TexCoord0, p.Data.TexCoord0...)
				existing.Data.TexCoord1 = append(existing.Data.TexCoord1, p.Data.TexCoord1...)
				existing.Data.Colors0 = append(existing.Data.Colors0, p.Data.Colors0...)
				existing.Data.Joints0 = append(existing.Data.Joints0, p.Data.Joints0...)
				existing.Data.Weights0 = append(existing.Data.Weights0, p.Data.Weights0...)
				existing.Data.VertexCount += p.Data.VertexCount

				if len(p.Data.Indices) > 0 {
					offsetIndices := make([]uint32, len(p.Data.Indices))
					for k, idx := range p.Data.Indices {
						offsetIndices[k] = idx + baseV
					}
					existing.Data.Indices = append(existing.Data.Indices, offsetIndices...)
				}
			} else {
				grouped[p.MaterialIndex] = j
			}
		}

		finalPrims := make([]ir.Primitive, 0, len(grouped))
		for _, idx := range grouped {
			finalPrims = append(finalPrims, mesh.Primitives[idx])
		}

		mesh.Primitives = finalPrims
	}
	return asset, nil
}
