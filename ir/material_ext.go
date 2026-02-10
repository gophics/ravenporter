package ir

// Node/Material extensions - Typed representations for standard PBR Next properties.

type MaterialClearcoat struct {
	Factor           float32
	RoughnessFactor  float32
	Texture          *TextureRef
	RoughnessTexture *TextureRef
	NormalTexture    *TextureRef
}

type MaterialSheen struct {
	ColorFactor      [3]float32
	ColorTexture     *TextureRef
	RoughnessFactor  float32
	RoughnessTexture *TextureRef
}

type MaterialTransmission struct {
	Factor  float32
	Texture *TextureRef
}

type MaterialVolume struct {
	ThicknessFactor     float32
	ThicknessTexture    *TextureRef
	AttenuationDistance float32
	AttenuationColor    [3]float32
}

type MaterialIOR struct {
	IOR float32
}

type MaterialSpecular struct {
	Factor       float32
	Texture      *TextureRef
	ColorFactor  [3]float32
	ColorTexture *TextureRef
}

type MaterialAnisotropy struct {
	Strength float32
	Rotation float32
	Texture  *TextureRef
}

type MaterialIridescence struct {
	Factor           float32
	Texture          *TextureRef
	IOR              float32
	ThicknessMinimum float32
	ThicknessMaximum float32
	ThicknessTexture *TextureRef
}

type MaterialDispersion struct {
	Dispersion float32
}

type MaterialDiffuseTransmission struct {
	Factor       float32
	Texture      *TextureRef
	ColorFactor  [3]float32
	ColorTexture *TextureRef
}

type MaterialSpecularGlossiness struct {
	DiffuseFactor             [4]float32
	DiffuseTexture            *TextureRef
	SpecularFactor            [3]float32
	GlossinessFactor          float32
	SpecularGlossinessTexture *TextureRef
}

type MaterialEmissiveStrength struct {
	EmissiveStrength float32
}
