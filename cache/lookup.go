package cache

import (
	"slices"

	"github.com/gophics/ravenporter/ir"
)

func (a *Asset) buildIndexes() {
	if a == nil || a.Asset == nil {
		return
	}
	a.meshes = buildMeshIndex(a.Asset.Meshes)
	a.materials = buildMaterialIndex(a.Asset.Materials)
	a.animations = buildAnimationIndex(a.Asset.Animations)
	a.nodes = buildNodeIndex(a.Asset.Nodes)
}

// FindMesh returns all mesh indices matching the given name.
func (a *Asset) FindMesh(name string) []int {
	return cloneIndexSlice(a.meshes[name])
}

// FindMaterial returns all material indices matching the given name.
func (a *Asset) FindMaterial(name string) []int {
	return cloneIndexSlice(a.materials[name])
}

// FindAnimation returns all animation indices matching the given name.
func (a *Asset) FindAnimation(name string) []int {
	return cloneIndexSlice(a.animations[name])
}

// FindNode returns all node indices matching the given name.
func (a *Asset) FindNode(name string) []int {
	return cloneIndexSlice(a.nodes[name])
}

func cloneIndexSlice(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}

func buildMeshIndex(values []*ir.Mesh) map[string][]int {
	index := make(map[string][]int)
	for i, value := range values {
		if value == nil || value.Name == "" {
			continue
		}
		index[value.Name] = append(index[value.Name], i)
	}
	return nilIfEmpty(index)
}

func buildMaterialIndex(values []*ir.Material) map[string][]int {
	index := make(map[string][]int)
	for i, value := range values {
		if value == nil || value.Name == "" {
			continue
		}
		index[value.Name] = append(index[value.Name], i)
	}
	return nilIfEmpty(index)
}

func buildAnimationIndex(values []*ir.Animation) map[string][]int {
	index := make(map[string][]int)
	for i, value := range values {
		if value == nil || value.Name == "" {
			continue
		}
		index[value.Name] = append(index[value.Name], i)
	}
	return nilIfEmpty(index)
}

func buildNodeIndex(values []ir.Node) map[string][]int {
	index := make(map[string][]int)
	for i := range values {
		value := values[i]
		if value.Name == "" {
			continue
		}
		index[value.Name] = append(index[value.Name], i)
	}
	return nilIfEmpty(index)
}

func nilIfEmpty(index map[string][]int) map[string][]int {
	if len(index) == 0 {
		return nil
	}
	return index
}
