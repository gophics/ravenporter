package usda

import (
	"strings"

	"github.com/gophics/ravenporter/ir"
)

type inheritArc struct {
	nodeIdx  int
	basePath string
}

func extractAssetPath(value string) string {
	start := strings.IndexByte(value, '@')
	if start >= 0 {
		end := strings.IndexByte(value[start+1:], '@')
		if end < 0 {
			return ""
		}
		value = value[start+1 : start+1+end]
	}
	if cut := strings.IndexByte(value, '<'); cut >= 0 {
		value = value[:cut]
	}
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "./")
	return value
}

func resolveInheritedNodes(asset *ir.Asset, inherits []inheritArc) {
	if len(inherits) == 0 {
		return
	}
	nameMap := make(map[string]int, len(asset.Nodes))
	for i := range asset.Nodes {
		nameMap[asset.Nodes[i].Name] = i
	}

	for _, arc := range inherits {
		baseName := arc.basePath
		if slashIdx := strings.LastIndexByte(arc.basePath, '/'); slashIdx >= 0 {
			baseName = arc.basePath[slashIdx+1:]
		}
		baseIdx, ok := nameMap[baseName]
		if !ok || arc.nodeIdx < 0 || arc.nodeIdx >= len(asset.Nodes) {
			continue
		}
		dst := &asset.Nodes[arc.nodeIdx]
		src := &asset.Nodes[baseIdx]
		if dst.MeshIndex == ir.NoIndex && src.MeshIndex != ir.NoIndex {
			dst.MeshIndex = src.MeshIndex
		}
		if dst.SkinIndex == ir.NoIndex && src.SkinIndex != ir.NoIndex {
			dst.SkinIndex = src.SkinIndex
		}
		if dst.CameraIndex == ir.NoIndex && src.CameraIndex != ir.NoIndex {
			dst.CameraIndex = src.CameraIndex
		}
		if dst.LightIndex == ir.NoIndex && src.LightIndex != ir.NoIndex {
			dst.LightIndex = src.LightIndex
		}
	}
}

func mergeRefScene(dst, src *ir.Asset) {
	nodeOff := len(dst.Nodes)
	meshOff := len(dst.Meshes)
	matOff := len(dst.Materials)
	texOff := len(dst.Textures)
	imgOff := len(dst.Images)
	skelOff := len(dst.Skeletons)
	camOff := len(dst.Cameras)
	lightOff := len(dst.Lights)

	dst.Meshes = append(dst.Meshes, src.Meshes...)
	dst.Materials = append(dst.Materials, src.Materials...)
	dst.Textures = append(dst.Textures, src.Textures...)
	dst.Images = append(dst.Images, src.Images...)
	dst.Skeletons = append(dst.Skeletons, src.Skeletons...)
	dst.Cameras = append(dst.Cameras, src.Cameras...)
	dst.Lights = append(dst.Lights, src.Lights...)

	for i := range src.Nodes {
		n := src.Nodes[i]
		if n.MeshIndex != ir.NoIndex {
			n.MeshIndex += meshOff
		}
		if n.SkinIndex != ir.NoIndex {
			n.SkinIndex += skelOff
		}
		if n.CameraIndex != ir.NoIndex {
			n.CameraIndex += camOff
		}
		if n.LightIndex != ir.NoIndex {
			n.LightIndex += lightOff
		}
		for j := range n.Children {
			n.Children[j] += nodeOff
		}
		dst.Nodes = append(dst.Nodes, n)
	}
	for _, ri := range src.RootNodes {
		dst.RootNodes = append(dst.RootNodes, ri+nodeOff)
	}
	for _, tex := range dst.Textures[texOff:] {
		if tex != nil && tex.ImageIndex != ir.NoIndex {
			tex.ImageIndex += imgOff
		}
	}
	for _, m := range dst.Materials[matOff:] {
		offsetTexRef(m.BaseColorTexture, texOff)
		offsetTexRef(m.MetallicTexture, texOff)
		offsetTexRef(m.RoughnessTexture, texOff)
		offsetTexRef(m.NormalTexture, texOff)
		offsetTexRef(m.EmissiveTexture, texOff)
		offsetTexRef(m.OcclusionTexture, texOff)
	}
	for i := range src.Meshes {
		for j := range src.Meshes[i].Primitives {
			if src.Meshes[i].Primitives[j].MaterialIndex != ir.NoIndex {
				dst.Meshes[meshOff+i].Primitives[j].MaterialIndex += matOff
			}
		}
	}
}

func offsetTexRef(ref *ir.TextureRef, off int) {
	if ref != nil {
		ref.TextureIndex += off
	}
}
