package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type splitLargeMeshesStep struct{}

func (s *splitLargeMeshesStep) Name() string      { return "SplitLargeMeshes" }
func (s *splitLargeMeshesStep) Flag() core.PPFlag { return core.PPSplitLargeMeshes }

func (s *splitLargeMeshesStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	limit := opts.MaxVerticesPerMesh
	if limit <= 0 {
		limit = 65535
	}

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}

		var newPrimitives []ir.Primitive

		for j := range mesh.Primitives {
			p := mesh.Primitives[j]
			if p.Data.VertexCount <= limit {
				newPrimitives = append(newPrimitives, p)
				continue
			}

			if p.Mode != ir.Triangles {
				newPrimitives = append(newPrimitives, p)
				continue
			}

			chunks := splitPrimitive(&p, limit)
			newPrimitives = append(newPrimitives, chunks...)
		}

		mesh.Primitives = newPrimitives
	}

	return asset, nil
}

//nolint:funlen // Splitting logic inherently requires multiple attribute operations
func splitPrimitive(p *ir.Primitive, limit int) []ir.Primitive {
	var chunks []ir.Primitive

	hasIndices := p.Data.HasIndices()
	pos := p.Data.Positions

	const vertsPerTri = 3
	triCount := len(pos) / vertsPerTri
	if hasIndices {
		triCount = len(p.Data.Indices) / vertsPerTri
	}

	var currentChunk ir.Primitive
	currentChunk.Mode = p.Mode
	currentChunk.MaterialIndex = p.MaterialIndex

	remap := make(map[uint32]uint32, limit)

	addVertex := func(oldIdx uint32) uint32 {
		if newIdx, ok := remap[oldIdx]; ok {
			return newIdx
		}
		newIdx := uint32(len(currentChunk.Data.Positions)) //nolint:gosec // bounds fit within 32-bit limits natively
		remap[oldIdx] = newIdx

		currentChunk.Data.Positions = append(currentChunk.Data.Positions, p.Data.Positions[oldIdx])
		if uint32(len(p.Data.Normals)) > oldIdx { //nolint:gosec // validation bound
			currentChunk.Data.Normals = append(currentChunk.Data.Normals, p.Data.Normals[oldIdx])
		}
		if uint32(len(p.Data.Tangents)) > oldIdx { //nolint:gosec // validation bound
			currentChunk.Data.Tangents = append(currentChunk.Data.Tangents, p.Data.Tangents[oldIdx])
		}
		if uint32(len(p.Data.TexCoord0)) > oldIdx { //nolint:gosec // array bound
			currentChunk.Data.TexCoord0 = append(currentChunk.Data.TexCoord0, p.Data.TexCoord0[oldIdx])
		}
		if uint32(len(p.Data.TexCoord1)) > oldIdx { //nolint:gosec // array bound
			currentChunk.Data.TexCoord1 = append(currentChunk.Data.TexCoord1, p.Data.TexCoord1[oldIdx])
		}
		if uint32(len(p.Data.Colors0)) > oldIdx { //nolint:gosec // array bound
			currentChunk.Data.Colors0 = append(currentChunk.Data.Colors0, p.Data.Colors0[oldIdx])
		}
		if uint32(len(p.Data.Joints0)) > oldIdx { //nolint:gosec // array bound
			currentChunk.Data.Joints0 = append(currentChunk.Data.Joints0, p.Data.Joints0[oldIdx])
		}
		if uint32(len(p.Data.Weights0)) > oldIdx { //nolint:gosec // array bound
			currentChunk.Data.Weights0 = append(currentChunk.Data.Weights0, p.Data.Weights0[oldIdx])
		}

		currentChunk.Data.VertexCount++
		return newIdx
	}

	for t := 0; t < triCount; t++ {
		var i0, i1, i2 uint32
		if hasIndices {
			idx := t * vertsPerTri
			i0 = p.Data.Indices[idx]
			i1 = p.Data.Indices[idx+1]
			i2 = p.Data.Indices[idx+2]
		} else {
			const idxOffset = 2
			i0 = uint32(t * vertsPerTri) //nolint:gosec // t fits in uint32 bounds here
			i1 = i0 + 1
			i2 = i0 + idxOffset
		}

		if currentChunk.Data.VertexCount+3 > limit {
			if len(currentChunk.Data.Positions) > 0 {
				chunks = append(chunks, currentChunk)
			}
			currentChunk = ir.Primitive{Mode: p.Mode, MaterialIndex: p.MaterialIndex}
			clear(remap)
		}

		currentChunk.Data.Indices = append(currentChunk.Data.Indices, addVertex(i0), addVertex(i1), addVertex(i2))
	}

	if !hasIndices {
		for i := triCount * vertsPerTri; i < len(pos); i++ {
			addVertex(uint32(i)) //nolint:gosec // i is bound check
		}
	}

	if currentChunk.Data.VertexCount > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}
