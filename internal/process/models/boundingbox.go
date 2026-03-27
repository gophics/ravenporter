package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type genBoundingBoxesStep struct{}

func (s *genBoundingBoxesStep) Name() string      { return "GenBoundingBoxes" }
func (s *genBoundingBoxesStep) Flag() core.PPFlag { return core.PPGenBoundingBoxes }

func (s *genBoundingBoxesStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil || len(mesh.Primitives) == 0 {
			continue
		}

		computeBounds(mesh)
	}
	return asset, nil
}

func computeBounds(mesh *ir.Mesh) {
	var initialized bool
	var minX, minY, minZ float32
	var maxX, maxY, maxZ float32

	for i := range mesh.Primitives {
		p := &mesh.Primitives[i]
		for _, pos := range p.Data.Positions {
			if !initialized {
				minX, minY, minZ = pos[0], pos[1], pos[2]
				maxX, maxY, maxZ = pos[0], pos[1], pos[2]
				initialized = true
				continue
			}
			if pos[0] < minX {
				minX = pos[0]
			}
			if pos[0] > maxX {
				maxX = pos[0]
			}
			if pos[1] < minY {
				minY = pos[1]
			}
			if pos[1] > maxY {
				maxY = pos[1]
			}
			if pos[2] < minZ {
				minZ = pos[2]
			}
			if pos[2] > maxZ {
				maxZ = pos[2]
			}
		}
	}

	mesh.BoundingBox[0] = [3]float32{minX, minY, minZ}
	mesh.BoundingBox[1] = [3]float32{maxX, maxY, maxZ}
}
