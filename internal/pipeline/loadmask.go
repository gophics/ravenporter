package pipeline

import "github.com/gophics/ravenporter/ir"

// LoadMask selects which content domains remain in the imported asset.
type LoadMask uint32

// LoadMask flags keep the selected content domains in the returned asset.
const (
	LoadMeshes LoadMask = 1 << iota
	LoadMaterials
	LoadTextures
	LoadAnimations
	LoadSkeletons
	LoadCameras
	LoadLights
	LoadAudio
	LoadFonts
	LoadImages

	LoadAll = LoadMeshes |
		LoadMaterials |
		LoadTextures |
		LoadAnimations |
		LoadSkeletons |
		LoadCameras |
		LoadLights |
		LoadAudio |
		LoadFonts |
		LoadImages
)

func (m LoadMask) includes(flag LoadMask) bool {
	return m&flag != 0
}

func pruneAsset(asset *ir.Asset, mask LoadMask) {
	if asset == nil {
		return
	}

	if !mask.includes(LoadMeshes) {
		pruneMeshes(asset)
	}
	if !mask.includes(LoadMaterials) {
		pruneMaterials(asset)
	}
	if !mask.includes(LoadTextures) {
		pruneTextures(asset)
	}
	if !mask.includes(LoadAnimations) {
		asset.Animations = nil
	}
	if !mask.includes(LoadSkeletons) {
		pruneSkeletons(asset)
	}
	if !mask.includes(LoadCameras) {
		pruneCameras(asset)
	}
	if !mask.includes(LoadLights) {
		pruneLights(asset)
	}
	if !mask.includes(LoadAudio) {
		asset.AudioClips = nil
	}
	if !mask.includes(LoadFonts) {
		asset.Fonts = nil
	}
	if !mask.includes(LoadImages) {
		pruneImages(asset)
	}
}

func pruneMeshes(asset *ir.Asset) {
	asset.Meshes = nil
	for i := range asset.Nodes {
		asset.Nodes[i].MeshIndex = ir.NoIndex
	}
	for i := range asset.CollisionMeshes {
		if asset.CollisionMeshes[i] == nil {
			continue
		}
		asset.CollisionMeshes[i].MeshIndex = ir.NoIndex
	}
}

func pruneMaterials(asset *ir.Asset) {
	asset.Materials = nil
	for _, mesh := range asset.Meshes {
		if mesh == nil {
			continue
		}
		for i := range mesh.Primitives {
			mesh.Primitives[i].MaterialIndex = ir.NoIndex
		}
	}
}

func pruneTextures(asset *ir.Asset) {
	asset.Textures = nil
	for _, mat := range asset.Materials {
		clearMaterialTextures(mat)
	}
	for _, light := range asset.Lights {
		if light == nil {
			continue
		}
		light.IESProfile = nil
	}
}

func pruneSkeletons(asset *ir.Asset) {
	asset.Skeletons = nil
	for i := range asset.Nodes {
		asset.Nodes[i].SkinIndex = ir.NoIndex
	}
}

func pruneCameras(asset *ir.Asset) {
	asset.Cameras = nil
	for i := range asset.Nodes {
		asset.Nodes[i].CameraIndex = ir.NoIndex
	}
}

func pruneLights(asset *ir.Asset) {
	asset.Lights = nil
	for i := range asset.Nodes {
		asset.Nodes[i].LightIndex = ir.NoIndex
	}
}

func pruneImages(asset *ir.Asset) {
	asset.Images = nil
	for _, font := range asset.Fonts {
		if font == nil || font.Bitmap == nil {
			continue
		}
		font.Bitmap.AtlasIndex = ir.NoIndex
		font.Bitmap.AtlasPath = ""
	}
}

func clearMaterialTextures(mat *ir.Material) {
	if mat == nil {
		return
	}
	mat.BaseColorTexture = nil
	mat.MetallicTexture = nil
	mat.RoughnessTexture = nil
	mat.NormalTexture = nil
	mat.OcclusionTexture = nil
	mat.EmissiveTexture = nil

	if mat.Clearcoat != nil {
		mat.Clearcoat.Texture = nil
		mat.Clearcoat.RoughnessTexture = nil
		mat.Clearcoat.NormalTexture = nil
	}
	if mat.Sheen != nil {
		mat.Sheen.ColorTexture = nil
		mat.Sheen.RoughnessTexture = nil
	}
	if mat.Transmission != nil {
		mat.Transmission.Texture = nil
	}
	if mat.Volume != nil {
		mat.Volume.ThicknessTexture = nil
	}
	if mat.Specular != nil {
		mat.Specular.Texture = nil
		mat.Specular.ColorTexture = nil
	}
	if mat.Anisotropy != nil {
		mat.Anisotropy.Texture = nil
	}
	if mat.Iridescence != nil {
		mat.Iridescence.Texture = nil
		mat.Iridescence.ThicknessTexture = nil
	}
	if mat.DiffuseTransmission != nil {
		mat.DiffuseTransmission.Texture = nil
		mat.DiffuseTransmission.ColorTexture = nil
	}
	if mat.SpecularGlossiness != nil {
		mat.SpecularGlossiness.DiffuseTexture = nil
		mat.SpecularGlossiness.SpecularGlossinessTexture = nil
	}
}
