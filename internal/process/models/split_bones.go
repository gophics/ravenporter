package models

import (
	"sync"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

var remapPool = sync.Pool{New: func() any { return &remapBuf{} }}

type remapBuf struct {
	remap []int32
	used  []int
}

const (
	defaultMaxBones  = 64
	boneSetInitScale = 2
)

type splitByBoneCountStep struct{}

func (s *splitByBoneCountStep) Name() string      { return "SplitByBoneCount" }
func (s *splitByBoneCountStep) Flag() core.PPFlag { return core.PPSplitByBoneCount }

func (s *splitByBoneCountStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	maxBones := opts.MaxBonesPerMesh
	if maxBones <= 0 {
		maxBones = defaultMaxBones
	}

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}

		var newPrims []ir.Primitive
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if !p.Data.HasBones() || p.Mode != ir.Triangles || !p.Data.HasIndices() {
				newPrims = append(newPrims, *p)
				continue
			}

			uniqueBones := countUniqueBones(&p.Data)
			if uniqueBones <= maxBones {
				newPrims = append(newPrims, *p)
				continue
			}

			splits := splitPrimitiveByBones(p, maxBones)
			newPrims = append(newPrims, splits...)
		}
		mesh.Primitives = newPrims
	}
	return asset, nil
}

const vec4Stride = 4

func countUniqueBones(d *ir.MeshData) int {
	var maxBone uint16
	for _, j := range d.Joints0 {
		for k := range vec4Stride {
			if j[k] > maxBone {
				maxBone = j[k]
			}
		}
	}
	for _, j := range d.Joints1 {
		for k := range vec4Stride {
			if j[k] > maxBone {
				maxBone = j[k]
			}
		}
	}

	seen := make([]bool, int(maxBone)+1)
	count := 0
	for _, j := range d.Joints0 {
		for k := range vec4Stride {
			if !seen[j[k]] {
				seen[j[k]] = true
				count++
			}
		}
	}
	for _, j := range d.Joints1 {
		for k := range vec4Stride {
			if !seen[j[k]] {
				seen[j[k]] = true
				count++
			}
		}
	}
	return count
}

type boneSet struct {
	flags []bool
	used  []uint16
	count int
}

func newBoneSet(capacity int) boneSet {
	return boneSet{flags: make([]bool, capacity), used: make([]uint16, 0, capacity)}
}

func (bs *boneSet) reset() {
	for _, b := range bs.used {
		bs.flags[b] = false
	}
	bs.used = bs.used[:0]
	bs.count = 0
}

func (bs *boneSet) add(b uint16) {
	if int(b) >= len(bs.flags) {
		grown := make([]bool, int(b)+1)
		copy(grown, bs.flags)
		bs.flags = grown
	}
	if !bs.flags[b] {
		bs.flags[b] = true
		bs.used = append(bs.used, b)
		bs.count++
	}
}

func (bs *boneSet) countMerged(other *boneSet) int {
	n := bs.count
	for _, b := range other.used {
		if int(b) >= len(bs.flags) || !bs.flags[b] {
			n++
		}
	}
	return n
}

func (bs *boneSet) mergeFrom(other *boneSet) {
	for _, b := range other.used {
		bs.add(b)
	}
}

func (bs *boneSet) cloneInto(dst *boneSet) {
	dst.reset()
	for _, b := range bs.used {
		dst.add(b)
	}
}

func splitPrimitiveByBones(
	p *ir.Primitive,
	maxBones int,
) []ir.Primitive {
	const triStride = 3
	numTris := len(p.Data.Indices) / triStride
	numVerts := len(p.Data.Positions)

	type triGroup struct {
		triStart int
		triCount int
		bones    boneSet
	}

	triBuf := make([]int, numTris)
	groups := make([]triGroup, 0, (numTris/maxBones)+1)
	triBufUsed := 0

	scratch := newBoneSet(maxBones * boneSetInitScale)

	for t := 0; t < numTris; t++ {
		baseIdx := t * triStride
		scratch.reset()
		collectTriBoneSet(&p.Data, baseIdx, &scratch)

		placed := false
		for gi := range groups {
			g := &groups[gi]
			if g.bones.countMerged(&scratch) <= maxBones {
				g.bones.mergeFrom(&scratch)
				triBuf[triBufUsed] = t
				triBufUsed++
				g.triCount++
				placed = true
				break
			}
		}

		if !placed {
			gs := newBoneSet(maxBones)
			scratch.cloneInto(&gs)
			groups = append(groups, triGroup{
				triStart: triBufUsed,
				triCount: 1,
				bones:    gs,
			})
			triBuf[triBufUsed] = t
			triBufUsed++
		}
	}

	rb := remapPool.Get().(*remapBuf) //nolint:errcheck // pool New guarantees type
	if cap(rb.remap) < numVerts {
		rb.remap = make([]int32, numVerts)
		rb.used = make([]int, 0, numVerts)
	} else {
		rb.remap = rb.remap[:numVerts]
		rb.used = rb.used[:0]
	}
	for i := range rb.remap {
		rb.remap[i] = -1
	}

	splits := make([]ir.Primitive, 0, len(groups))
	for _, g := range groups {
		tris := triBuf[g.triStart : g.triStart+g.triCount]
		np := buildSplitPrimitiveFlat(p, tris, rb.remap, &rb.used)
		splits = append(splits, np)

		for _, idx := range rb.used {
			rb.remap[idx] = -1
		}
		rb.used = rb.used[:0]
	}
	remapPool.Put(rb)
	return splits
}

func collectTriBoneSet(d *ir.MeshData, baseIdx int, bs *boneSet) {
	const triStride = 3
	for k := range triStride {
		vi := int(d.Indices[baseIdx+k])
		if vi < len(d.Joints0) {
			for c := range vec4Stride {
				if vi < len(d.Weights0) && d.Weights0[vi][c] > 0 {
					bs.add(d.Joints0[vi][c])
				}
			}
		}
		if vi < len(d.Joints1) {
			for c := range vec4Stride {
				if vi < len(d.Weights1) && d.Weights1[vi][c] > 0 {
					bs.add(d.Joints1[vi][c])
				}
			}
		}
	}
}

//nolint:funlen // vertex remapping requires checking all attribute channels
func buildSplitPrimitiveFlat(
	src *ir.Primitive,
	tris []int,
	remap []int32,
	remapUsed *[]int,
) ir.Primitive {
	const triStride = 3
	estimatedVerts := len(tris) * triStride

	np := ir.Primitive{
		Mode:          src.Mode,
		MaterialIndex: src.MaterialIndex,
	}

	np.Data.Positions = make([][3]float32, 0, estimatedVerts)
	np.Data.Indices = make([]uint32, 0, estimatedVerts)
	if len(src.Data.Normals) > 0 {
		np.Data.Normals = make([][3]float32, 0, estimatedVerts)
	}
	if len(src.Data.Tangents) > 0 {
		np.Data.Tangents = make([][4]float32, 0, estimatedVerts)
	}
	if len(src.Data.TexCoord0) > 0 {
		np.Data.TexCoord0 = make([][2]float32, 0, estimatedVerts)
	}
	if len(src.Data.TexCoord1) > 0 {
		np.Data.TexCoord1 = make([][2]float32, 0, estimatedVerts)
	}
	if len(src.Data.Colors0) > 0 {
		np.Data.Colors0 = make([][4]float32, 0, estimatedVerts)
	}
	if len(src.Data.Joints0) > 0 {
		np.Data.Joints0 = make([][4]uint16, 0, estimatedVerts)
	}
	if len(src.Data.Joints1) > 0 {
		np.Data.Joints1 = make([][4]uint16, 0, estimatedVerts)
	}
	if len(src.Data.Weights0) > 0 {
		np.Data.Weights0 = make([][4]float32, 0, estimatedVerts)
	}
	if len(src.Data.Weights1) > 0 {
		np.Data.Weights1 = make([][4]float32, 0, estimatedVerts)
	}

	for _, t := range tris {
		base := t * triStride
		for k := range triStride {
			oldIdx := src.Data.Indices[base+k]
			if mapped := remap[oldIdx]; mapped >= 0 {
				np.Data.Indices = append(np.Data.Indices, uint32(mapped))
				continue
			}
			newIdx := int32(len(np.Data.Positions)) //nolint:gosec // chunk size is capped by MaxVerticesPerMesh.
			remap[oldIdx] = newIdx
			*remapUsed = append(*remapUsed, int(oldIdx))
			np.Data.Indices = append(np.Data.Indices, uint32(newIdx))
			oi := int(oldIdx)
			np.Data.Positions = append(np.Data.Positions, src.Data.Positions[oi])
			if np.Data.Normals != nil {
				np.Data.Normals = append(np.Data.Normals, src.Data.Normals[oi])
			}
			if np.Data.Tangents != nil {
				np.Data.Tangents = append(np.Data.Tangents, src.Data.Tangents[oi])
			}
			if np.Data.TexCoord0 != nil {
				np.Data.TexCoord0 = append(np.Data.TexCoord0, src.Data.TexCoord0[oi])
			}
			if np.Data.TexCoord1 != nil {
				np.Data.TexCoord1 = append(np.Data.TexCoord1, src.Data.TexCoord1[oi])
			}
			if np.Data.Colors0 != nil {
				np.Data.Colors0 = append(np.Data.Colors0, src.Data.Colors0[oi])
			}
			if np.Data.Joints0 != nil {
				np.Data.Joints0 = append(np.Data.Joints0, src.Data.Joints0[oi])
			}
			if np.Data.Joints1 != nil {
				np.Data.Joints1 = append(np.Data.Joints1, src.Data.Joints1[oi])
			}
			if np.Data.Weights0 != nil {
				np.Data.Weights0 = append(np.Data.Weights0, src.Data.Weights0[oi])
			}
			if np.Data.Weights1 != nil {
				np.Data.Weights1 = append(np.Data.Weights1, src.Data.Weights1[oi])
			}
		}
	}

	np.Data.VertexCount = len(np.Data.Positions)
	return np
}
