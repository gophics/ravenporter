package ir

// Material holds PBR metallic-roughness properties.
type Material struct {
	Name string

	// PBR Metallic-Roughness
	BaseColorFactor   [4]float32 // RGBA linear, default {1,1,1,1}
	BaseColorTexture  *TextureRef
	MetallicFactor    float32
	RoughnessFactor   float32
	MetallicTexture   *TextureRef // Channel: 'B' (glTF packs in blue channel)
	RoughnessTexture  *TextureRef // Channel: 'G' (glTF packs in green channel)
	NormalTexture     *TextureRef
	NormalScale       float32
	OcclusionTexture  *TextureRef
	OcclusionStrength float32
	EmissiveFactor    [3]float32 // RGB linear
	EmissiveTexture   *TextureRef

	// Alpha.
	AlphaMode   AlphaMode
	AlphaCutoff float32

	// Rendering.
	DoubleSided bool
	Unlit       bool

	// PBR Next Typed Extensions
	Clearcoat           *MaterialClearcoat
	Sheen               *MaterialSheen
	Transmission        *MaterialTransmission
	Volume              *MaterialVolume
	IOR                 *MaterialIOR
	Specular            *MaterialSpecular
	Anisotropy          *MaterialAnisotropy
	Iridescence         *MaterialIridescence
	Dispersion          *MaterialDispersion
	DiffuseTransmission *MaterialDiffuseTransmission
	SpecularGlossiness  *MaterialSpecularGlossiness
	EmissiveStrength    *MaterialEmissiveStrength

	// Format-specific extended properties.
	Properties map[string]any
}

// TextureRef references a texture in Scene.Textures with UV mapping.
type TextureRef struct {
	TextureIndex int
	UVSet        int
	Offset       [2]float32
	Tiling       [2]float32 // UV tiling/scale (default: {1,1})
	Rotation     float32
	Channel      uint8 // Source channel: 'R', 'G', 'B', 'A', or 0 = all/RGBA
}

// Texture holds texture metadata and optionally embedded data.
// Exactly one of Embedded or ExternalPath is populated.
type Texture struct {
	Name       string
	ImageIndex int
	MipLevels  int
	WrapS      TextureWrap
	WrapT      TextureWrap
	MinFilter  TextureFilter
	MagFilter  TextureFilter
	Metadata   map[string]string
}
