package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type fixUpAxisStep struct{}

func (s *fixUpAxisStep) Name() string      { return "FixUpAxis" }
func (s *fixUpAxisStep) Flag() core.PPFlag { return core.PPFixUpAxis }

func (s *fixUpAxisStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	if asset.UpAxis == ir.YUp {
		return asset, nil
	}

	for i := range asset.Nodes {
		t := &asset.Nodes[i].Transform
		t.Translation = [3]float32{t.Translation[0], t.Translation[2], t.Translation[1]}
		t.Scale = [3]float32{t.Scale[0], t.Scale[2], t.Scale[1]}
		t.Rotation = [4]float32{t.Rotation[0], t.Rotation[2], t.Rotation[1], t.Rotation[3]}
	}

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			for k, pos := range p.Data.Positions {
				p.Data.Positions[k] = [3]float32{pos[0], pos[2], pos[1]}
			}
			for k, norm := range p.Data.Normals {
				p.Data.Normals[k] = [3]float32{norm[0], norm[2], norm[1]}
			}
			for k, tan := range p.Data.Tangents {
				p.Data.Tangents[k] = [4]float32{tan[0], tan[2], tan[1], tan[3]}
			}
		}
	}

	asset.UpAxis = ir.YUp
	return asset, nil
}

type makeLeftHandedStep struct{}

func (s *makeLeftHandedStep) Name() string      { return "MakeLeftHanded" }
func (s *makeLeftHandedStep) Flag() core.PPFlag { return core.PPMakeLeftHanded }

func (s *makeLeftHandedStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Nodes {
		t := &asset.Nodes[i].Transform
		t.Translation[2] = -t.Translation[2]
		t.Rotation[0] = -t.Rotation[0]
		t.Rotation[1] = -t.Rotation[1]
	}

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			for k, pos := range p.Data.Positions {
				p.Data.Positions[k] = [3]float32{pos[0], pos[1], -pos[2]}
			}
			for k, norm := range p.Data.Normals {
				p.Data.Normals[k] = [3]float32{norm[0], norm[1], -norm[2]}
			}
			for k, tan := range p.Data.Tangents {
				p.Data.Tangents[k] = [4]float32{tan[0], tan[1], -tan[2], tan[3]}
			}

			if p.Mode == ir.Triangles && p.Data.HasIndices() {
				idx := p.Data.Indices
				for k := 0; k+2 < len(idx); k += 3 {
					idx[k+1], idx[k+2] = idx[k+2], idx[k+1]
				}
			}
		}
	}
	return asset, nil
}
