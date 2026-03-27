package models

import (
	"math"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type fixInfacingNormalsStep struct{}

func (s *fixInfacingNormalsStep) Name() string      { return "FixInfacingNormals" }
func (s *fixInfacingNormalsStep) Flag() core.PPFlag { return core.PPFixInfacingNormals }

func (s *fixInfacingNormalsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if len(p.Data.Positions) == 0 || len(p.Data.Normals) == 0 || len(p.Data.Positions) != len(p.Data.Normals) {
				continue
			}

			fixInfacingPrimitive(&p.Data)
		}
	}
	return asset, nil
}

func fixInfacingPrimitive(d *ir.MeshData) {
	originalVol := aabbVolume(d.Positions, d.Normals, false)
	flippedVol := aabbVolume(d.Positions, d.Normals, true)

	if flippedVol < originalVol {
		for k := range d.Normals {
			d.Normals[k] = [3]float32{-d.Normals[k][0], -d.Normals[k][1], -d.Normals[k][2]}
		}
	}
}

const rayStep = 0.01

func aabbVolume(positions, normals [][3]float32, flip bool) float64 {
	var minX, minY, minZ float32 = math.MaxFloat32, math.MaxFloat32, math.MaxFloat32
	var maxX, maxY, maxZ float32 = -math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32

	for k := range positions {
		n := normals[k]
		if flip {
			n = [3]float32{-n[0], -n[1], -n[2]}
		}

		px := positions[k][0] + n[0]*rayStep
		py := positions[k][1] + n[1]*rayStep
		pz := positions[k][2] + n[2]*rayStep

		if px < minX {
			minX = px
		}
		if px > maxX {
			maxX = px
		}
		if py < minY {
			minY = py
		}
		if py > maxY {
			maxY = py
		}
		if pz < minZ {
			minZ = pz
		}
		if pz > maxZ {
			maxZ = pz
		}
	}

	dx := float64(maxX - minX)
	dy := float64(maxY - minY)
	dz := float64(maxZ - minZ)
	return dx * dy * dz
}
