package models

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type limitBoneWeightsStep struct{}

func (s *limitBoneWeightsStep) Name() string      { return "LimitBoneWeights" }
func (s *limitBoneWeightsStep) Flag() core.PPFlag { return core.PPLimitBoneWeights }

func (s *limitBoneWeightsStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	const (
		defaultMaxWeights  = 4
		absoluteMaxWeights = 8
	)
	maxWeights := opts.MaxBoneWeights
	if maxWeights <= 0 {
		maxWeights = defaultMaxWeights
	}

	if maxWeights > absoluteMaxWeights {
		maxWeights = absoluteMaxWeights
	}

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if len(p.Data.Weights0) == 0 {
				continue
			}
			limitVertexWeights(&p.Data, maxWeights)
		}
	}
	return asset, nil
}

type weightEntry struct {
	w float32
	j uint16
}

const maxBoneSlots = 8

func insertionSortWeights(arr *[maxBoneSlots]weightEntry, count int) {
	for i := 1; i < count; i++ {
		key := arr[i]
		j := i - 1
		for j >= 0 && arr[j].w < key.w {
			arr[j+1] = arr[j]
			j--
		}
		arr[j+1] = key
	}
}

func limitVertexWeights(d *ir.MeshData, maxW int) {
	const vec4Len = 4
	hasWeights1 := len(d.Weights1) > 0

	for i := range d.Weights0 {
		w0 := d.Weights0[i]
		j0 := d.Joints0[i]
		var w1 [4]float32
		var j1 [4]uint16
		if hasWeights1 && i < len(d.Weights1) {
			w1 = d.Weights1[i]
			j1 = d.Joints1[i]
		}

		var active [maxBoneSlots]weightEntry
		count := 0
		for k := range vec4Len {
			if w0[k] > 0 { //nolint:gosec // k bounded by [4]float32 array size
				active[count] = weightEntry{w0[k], j0[k]}
				count++
			}
		}
		if hasWeights1 && i < len(d.Weights1) {
			for k := range vec4Len {
				if w1[k] > 0 { //nolint:gosec // k bounded by [4]float32 array size
					active[count] = weightEntry{w1[k], j1[k]}
					count++
				}
			}
		}

		if count <= maxW {
			continue
		}

		insertionSortWeights(&active, count)
		count = maxW

		var sum float32
		for k := 0; k < count; k++ {
			sum += active[k].w
		}

		newW0 := [4]float32{}
		newJ0 := [4]uint16{}
		var newW1 [4]float32
		var newJ1 [4]uint16

		scale := 1.0 / sum
		for k := 0; k < count; k++ {
			if k < vec4Len {
				newW0[k] = active[k].w * scale
				newJ0[k] = active[k].j
			} else {
				newW1[k-vec4Len] = active[k].w * scale
				newJ1[k-vec4Len] = active[k].j
			}
		}

		d.Weights0[i] = newW0
		d.Joints0[i] = newJ0
		if hasWeights1 && i < len(d.Weights1) {
			d.Weights1[i] = newW1
			d.Joints1[i] = newJ1
		}
	}

	if maxW <= vec4Len {
		d.Weights1 = nil
		d.Joints1 = nil
	}
}
