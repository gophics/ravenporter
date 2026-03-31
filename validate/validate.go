package validate

import (
	"fmt"
	"math"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/rperr"
)

// Validation issue codes returned by structural and semantic validation.
const (
	CodeNilAsset           = "NIL_ASSET"
	CodeNilMesh            = "NIL_MESH"
	CodeEmptyMesh          = "EMPTY_MESH"
	CodeAttrLengthMismatch = "ATTR_LENGTH_MISMATCH"
	CodeIndexOutOfBounds   = "INDEX_OUT_OF_BOUNDS"
	CodeNaNInfPosition     = "NAN_INF_POSITION"
	CodeDegenerateTriangle = "DEGENERATE_TRIANGLE"
	CodeOrphanMaterial     = "ORPHAN_MATERIAL"
	CodeCyclicGraph        = "CYCLIC_GRAPH"
	CodePBROutOfRange      = "PBR_OUT_OF_RANGE"
	CodeTextureRefBounds   = "TEXTURE_REF_BOUNDS"
	CodeNodeRefBounds      = "NODE_REF_BOUNDS"
	CodeAnimationNaN       = "ANIMATION_NAN"

	posComponents = 3
)

// Result collects validation findings.
type Result struct {
	Errors   []*rperr.ValidationError
	Warnings []*rperr.ValidationError
}

// OK returns true if no errors were found.
func (r *Result) OK() bool { return len(r.Errors) == 0 }

func (r *Result) addError(code, msg string) {
	r.Errors = append(r.Errors, &rperr.ValidationError{
		Severity: rperr.SeverityError, Code: code, Message: msg,
	})
}

func (r *Result) addWarning(code, msg string) {
	r.Warnings = append(r.Warnings, &rperr.ValidationError{
		Severity: rperr.SeverityWarning, Code: code, Message: msg,
	})
}

func (r *Result) merge(other *Result) {
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}

// Asset validates an asset by running both structural and semantic passes.
func Asset(a *ir.Asset) *Result {
	r := Structural(a)
	r.merge(Semantic(a))
	return r
}

// Structural validates data integrity before post-processing.
func Structural(a *ir.Asset) *Result {
	r := &Result{}
	if a == nil {
		r.addError(CodeNilAsset, "asset is nil")
		return r
	}

	checkDAGCycles(r, a)
	checkNodeRefBounds(r, a)

	for mi, mesh := range a.Meshes {
		if mesh == nil {
			r.addError(CodeNilMesh, fmt.Sprintf("mesh[%d] is nil", mi))
			continue
		}
		for pi := range mesh.Primitives {
			prim := &mesh.Primitives[pi]
			data := &prim.Data
			vertexCount := data.VertexCount
			prefix := fmt.Sprintf("mesh[%d].prim[%d]", mi, pi)

			if vertexCount == 0 {
				r.addWarning(CodeEmptyMesh, prefix+" has zero vertices")
				continue
			}

			if data.Normals != nil && len(data.Normals) != vertexCount {
				r.addError(CodeAttrLengthMismatch,
					fmt.Sprintf("%s normals length %d != vertex count %d", prefix, len(data.Normals), vertexCount))
			}
			if data.Tangents != nil && len(data.Tangents) != vertexCount {
				r.addError(CodeAttrLengthMismatch,
					fmt.Sprintf("%s tangents length %d != vertex count %d", prefix, len(data.Tangents), vertexCount))
			}

			for ii, index := range data.Indices {
				if int(index) >= vertexCount {
					r.addError(CodeIndexOutOfBounds,
						fmt.Sprintf("%s index[%d]=%d >= vertex count %d", prefix, ii, index, vertexCount))
					break
				}
			}

			for vi, position := range data.Positions {
				for axis := 0; axis < posComponents; axis++ {
					if math.IsNaN(float64(position[axis])) || math.IsInf(float64(position[axis]), 0) {
						r.addError(CodeNaNInfPosition,
							fmt.Sprintf("%s vertex[%d] component %d is NaN/Inf", prefix, vi, axis))
						break
					}
				}
			}
		}
	}

	return r
}

// Semantic validates logical correctness after post-processing.
func Semantic(a *ir.Asset) *Result {
	r := &Result{}
	if a == nil {
		return r
	}

	for mi, mesh := range a.Meshes {
		if mesh == nil {
			continue
		}
		for pi := range mesh.Primitives {
			prim := &mesh.Primitives[pi]
			data := &prim.Data
			prefix := fmt.Sprintf("mesh[%d].prim[%d]", mi, pi)
			if prim.Mode == ir.Triangles && data.HasIndices() {
				checkDegenerateTriangles(r, data, prefix)
			}
		}
	}

	checkOrphanMaterials(r, a)
	checkPBRRanges(r, a)
	checkMaterialExtRanges(r, a)
	checkTextureRefBounds(r, a)
	checkAnimationNaN(r, a)
	return r
}

func checkDAGCycles(r *Result, a *ir.Asset) {
	count := len(a.Nodes)
	if count == 0 {
		return
	}

	const (
		colorWhite = 0
		colorGray  = 1
		colorBlack = 2
	)
	color := make([]int, count)

	var dfs func(idx int) bool
	dfs = func(idx int) bool {
		if idx < 0 || idx >= count {
			return false
		}
		if color[idx] == colorGray {
			return true
		}
		if color[idx] == colorBlack {
			return false
		}
		color[idx] = colorGray
		for _, child := range a.Nodes[idx].Children {
			if dfs(child) {
				return true
			}
		}
		color[idx] = colorBlack
		return false
	}

	for i := range count {
		if color[i] == colorWhite && dfs(i) {
			r.addError(CodeCyclicGraph, fmt.Sprintf("cyclic node graph detected at node %d (%q)", i, a.Nodes[i].Name))
			return
		}
	}
}

func checkNodeRefBounds(r *Result, a *ir.Asset) {
	meshLen := len(a.Meshes)
	cameraLen := len(a.Cameras)
	skeletonLen := len(a.Skeletons)
	lodLen := len(a.LODGroups)

	for i := range a.Nodes {
		node := &a.Nodes[i]
		if node.MeshIndex >= 0 && node.MeshIndex >= meshLen {
			r.addError(CodeNodeRefBounds, fmt.Sprintf("node[%d] mesh index %d out of range [0,%d)", i, node.MeshIndex, meshLen))
		}
		if node.CameraIndex >= 0 && node.CameraIndex >= cameraLen {
			r.addError(CodeNodeRefBounds, fmt.Sprintf("node[%d] camera index %d out of range [0,%d)", i, node.CameraIndex, cameraLen))
		}
		if node.SkinIndex >= 0 && node.SkinIndex >= skeletonLen {
			r.addError(CodeNodeRefBounds, fmt.Sprintf("node[%d] skin index %d out of range [0,%d)", i, node.SkinIndex, skeletonLen))
		}
		if node.LODGroupIndex >= 0 && node.LODGroupIndex >= lodLen {
			r.addError(CodeNodeRefBounds, fmt.Sprintf("node[%d] LODGroup index %d out of range [0,%d)", i, node.LODGroupIndex, lodLen))
		}
		if node.ParentIndex != ir.NoIndex && (node.ParentIndex < 0 || node.ParentIndex >= len(a.Nodes)) {
			r.addError(CodeNodeRefBounds, fmt.Sprintf("node[%d] parent index %d out of range [0,%d)", i, node.ParentIndex, len(a.Nodes)))
		}
	}

	for i, scene := range a.Scenes {
		if scene == nil {
			continue
		}
		for _, root := range scene.RootNodes {
			if root < 0 || root >= len(a.Nodes) {
				r.addError(CodeNodeRefBounds, fmt.Sprintf("scene[%d] root node index %d out of range [0,%d)", i, root, len(a.Nodes)))
			}
		}
	}

	for i, collision := range a.CollisionMeshes {
		if collision == nil {
			continue
		}
		if collision.MeshIndex >= 0 && collision.MeshIndex >= meshLen {
			r.addError(CodeNodeRefBounds, fmt.Sprintf("collision[%d] mesh index %d out of range [0,%d)", i, collision.MeshIndex, meshLen))
		}
	}
}

func checkDegenerateTriangles(r *Result, data *ir.MeshData, prefix string) {
	const stride = 3
	for i := 0; i+2 < len(data.Indices); i += stride {
		a, b, c := data.Indices[i], data.Indices[i+1], data.Indices[i+2]
		if a == b || b == c || a == c {
			r.addWarning(CodeDegenerateTriangle, fmt.Sprintf("%s triangle %d has duplicate indices", prefix, i/stride))
		}
	}
}

func checkOrphanMaterials(r *Result, a *ir.Asset) {
	if len(a.Meshes) == 0 {
		return
	}
	used := make(map[int]bool)
	for _, mesh := range a.Meshes {
		if mesh == nil {
			continue
		}
		for i := range mesh.Primitives {
			if mesh.Primitives[i].MaterialIndex >= 0 {
				used[mesh.Primitives[i].MaterialIndex] = true
			}
		}
	}
	for i, material := range a.Materials {
		if material == nil {
			continue
		}
		if !used[i] {
			r.addWarning(CodeOrphanMaterial, fmt.Sprintf("material[%d] %q is not referenced", i, material.Name))
		}
	}
}

func checkPBRRanges(r *Result, a *ir.Asset) {
	for i, mat := range a.Materials {
		if mat == nil {
			continue
		}
		if mat.MetallicFactor < 0 || mat.MetallicFactor > 1 {
			r.addWarning(
				CodePBROutOfRange,
				fmt.Sprintf("material[%d] %q metallic factor %.3f out of [0,1]", i, mat.Name, mat.MetallicFactor),
			)
		}
		if mat.RoughnessFactor < 0 || mat.RoughnessFactor > 1 {
			r.addWarning(
				CodePBROutOfRange,
				fmt.Sprintf("material[%d] %q roughness factor %.3f out of [0,1]", i, mat.Name, mat.RoughnessFactor),
			)
		}
	}
}

func checkTextureRefBounds(r *Result, a *ir.Asset) {
	textureCount := len(a.Textures)
	for i, mat := range a.Materials {
		if mat == nil {
			continue
		}
		refs := []*ir.TextureRef{
			mat.BaseColorTexture, mat.MetallicTexture, mat.RoughnessTexture,
			mat.NormalTexture, mat.OcclusionTexture, mat.EmissiveTexture,
		}
		if mat.Clearcoat != nil {
			refs = append(refs, mat.Clearcoat.Texture, mat.Clearcoat.RoughnessTexture, mat.Clearcoat.NormalTexture)
		}
		if mat.Sheen != nil {
			refs = append(refs, mat.Sheen.ColorTexture, mat.Sheen.RoughnessTexture)
		}
		if mat.Transmission != nil {
			refs = append(refs, mat.Transmission.Texture)
		}
		if mat.Volume != nil {
			refs = append(refs, mat.Volume.ThicknessTexture)
		}
		if mat.Specular != nil {
			refs = append(refs, mat.Specular.Texture, mat.Specular.ColorTexture)
		}
		if mat.Anisotropy != nil {
			refs = append(refs, mat.Anisotropy.Texture)
		}
		if mat.Iridescence != nil {
			refs = append(refs, mat.Iridescence.Texture, mat.Iridescence.ThicknessTexture)
		}
		if mat.DiffuseTransmission != nil {
			refs = append(refs, mat.DiffuseTransmission.Texture, mat.DiffuseTransmission.ColorTexture)
		}
		if mat.SpecularGlossiness != nil {
			refs = append(refs, mat.SpecularGlossiness.DiffuseTexture, mat.SpecularGlossiness.SpecularGlossinessTexture)
		}
		for _, ref := range refs {
			if ref != nil && (ref.TextureIndex < 0 || ref.TextureIndex >= textureCount) {
				r.addError(CodeTextureRefBounds,
					fmt.Sprintf("material[%d] %q texture index %d out of range [0,%d)", i, mat.Name, ref.TextureIndex, textureCount))
			}
		}
	}

	for i, texture := range a.Textures {
		if texture == nil {
			continue
		}
		if texture.ImageIndex != ir.NoIndex && (texture.ImageIndex < 0 || texture.ImageIndex >= len(a.Images)) {
			r.addError(CodeTextureRefBounds,
				fmt.Sprintf("texture[%d] %q image index %d out of range [0,%d)", i, texture.Name, texture.ImageIndex, len(a.Images)))
		}
	}
}

func checkAnimationNaN(r *Result, a *ir.Asset) {
	for ai, anim := range a.Animations {
		if anim == nil {
			continue
		}
		for ci := range anim.Channels {
			channel := &anim.Channels[ci]
			for ti, t := range channel.Times {
				if math.IsNaN(float64(t)) || math.IsInf(float64(t), 0) {
					r.addError(CodeAnimationNaN, fmt.Sprintf("animation[%d].channel[%d] keyframe time[%d] is NaN/Inf", ai, ci, ti))
					break
				}
			}
		}
	}
}

func checkMaterialExtRanges(r *Result, a *ir.Asset) {
	for i, mat := range a.Materials {
		if mat == nil {
			continue
		}
		if mat.Clearcoat != nil {
			if mat.Clearcoat.Factor < 0 || mat.Clearcoat.Factor > 1 {
				r.addWarning(CodePBROutOfRange, fmt.Sprintf("material[%d] clearcoat factor %.3f out of [0,1]", i, mat.Clearcoat.Factor))
			}
			if mat.Clearcoat.RoughnessFactor < 0 || mat.Clearcoat.RoughnessFactor > 1 {
				r.addWarning(
					CodePBROutOfRange,
					fmt.Sprintf("material[%d] clearcoat roughness factor %.3f out of [0,1]", i, mat.Clearcoat.RoughnessFactor),
				)
			}
		}
		if mat.Sheen != nil && (mat.Sheen.RoughnessFactor < 0 || mat.Sheen.RoughnessFactor > 1) {
			r.addWarning(
				CodePBROutOfRange,
				fmt.Sprintf("material[%d] sheen roughness factor %.3f out of [0,1]", i, mat.Sheen.RoughnessFactor),
			)
		}
		if mat.Transmission != nil && (mat.Transmission.Factor < 0 || mat.Transmission.Factor > 1) {
			r.addWarning(CodePBROutOfRange, fmt.Sprintf("material[%d] transmission factor %.3f out of [0,1]", i, mat.Transmission.Factor))
		}
		if mat.Iridescence != nil && (mat.Iridescence.Factor < 0 || mat.Iridescence.Factor > 1) {
			r.addWarning(CodePBROutOfRange, fmt.Sprintf("material[%d] iridescence factor %.3f out of [0,1]", i, mat.Iridescence.Factor))
		}
	}
}
