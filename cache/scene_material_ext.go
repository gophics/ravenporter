package cache

import "github.com/gophics/ravenporter/ir"

func writeTextureRef(enc *encoder, ref *ir.TextureRef) {
	enc.bool(ref != nil)
	if ref == nil {
		return
	}
	enc.int(ref.TextureIndex)
	enc.int(ref.UVSet)
	writeVec2(enc, ref.Offset)
	writeVec2(enc, ref.Tiling)
	enc.f32(ref.Rotation)
	enc.u8(ref.Channel)
}

func readTextureRef(dec *decoder) *ir.TextureRef {
	if !dec.bool() {
		return nil
	}
	return &ir.TextureRef{
		TextureIndex: int(dec.i32()),
		UVSet:        int(dec.i32()),
		Offset:       readVec2(dec),
		Tiling:       readVec2(dec),
		Rotation:     dec.f32(),
		Channel:      dec.u8(),
	}
}

func writeClearcoat(enc *encoder, value *ir.MaterialClearcoat) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.Factor)
	enc.f32(value.RoughnessFactor)
	writeTextureRef(enc, value.Texture)
	writeTextureRef(enc, value.RoughnessTexture)
	writeTextureRef(enc, value.NormalTexture)
}

func readClearcoat(dec *decoder) *ir.MaterialClearcoat {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialClearcoat{
		Factor:           dec.f32(),
		RoughnessFactor:  dec.f32(),
		Texture:          readTextureRef(dec),
		RoughnessTexture: readTextureRef(dec),
		NormalTexture:    readTextureRef(dec),
	}
}

func writeSheen(enc *encoder, value *ir.MaterialSheen) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	writeVec3(enc, value.ColorFactor)
	writeTextureRef(enc, value.ColorTexture)
	enc.f32(value.RoughnessFactor)
	writeTextureRef(enc, value.RoughnessTexture)
}

func readSheen(dec *decoder) *ir.MaterialSheen {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialSheen{
		ColorFactor:      readVec3(dec),
		ColorTexture:     readTextureRef(dec),
		RoughnessFactor:  dec.f32(),
		RoughnessTexture: readTextureRef(dec),
	}
}

func writeTransmission(enc *encoder, value *ir.MaterialTransmission) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.Factor)
	writeTextureRef(enc, value.Texture)
}

func readTransmission(dec *decoder) *ir.MaterialTransmission {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialTransmission{
		Factor:  dec.f32(),
		Texture: readTextureRef(dec),
	}
}

func writeVolume(enc *encoder, value *ir.MaterialVolume) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.ThicknessFactor)
	writeTextureRef(enc, value.ThicknessTexture)
	enc.f32(value.AttenuationDistance)
	writeVec3(enc, value.AttenuationColor)
}

func readVolume(dec *decoder) *ir.MaterialVolume {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialVolume{
		ThicknessFactor:     dec.f32(),
		ThicknessTexture:    readTextureRef(dec),
		AttenuationDistance: dec.f32(),
		AttenuationColor:    readVec3(dec),
	}
}

func writeIOR(enc *encoder, value *ir.MaterialIOR) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.IOR)
}

func readIOR(dec *decoder) *ir.MaterialIOR {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialIOR{IOR: dec.f32()}
}

func writeSpecular(enc *encoder, value *ir.MaterialSpecular) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.Factor)
	writeTextureRef(enc, value.Texture)
	writeVec3(enc, value.ColorFactor)
	writeTextureRef(enc, value.ColorTexture)
}

func readSpecular(dec *decoder) *ir.MaterialSpecular {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialSpecular{
		Factor:       dec.f32(),
		Texture:      readTextureRef(dec),
		ColorFactor:  readVec3(dec),
		ColorTexture: readTextureRef(dec),
	}
}

func writeAnisotropy(enc *encoder, value *ir.MaterialAnisotropy) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.Strength)
	enc.f32(value.Rotation)
	writeTextureRef(enc, value.Texture)
}

func readAnisotropy(dec *decoder) *ir.MaterialAnisotropy {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialAnisotropy{
		Strength: dec.f32(),
		Rotation: dec.f32(),
		Texture:  readTextureRef(dec),
	}
}

func writeIridescence(enc *encoder, value *ir.MaterialIridescence) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.Factor)
	writeTextureRef(enc, value.Texture)
	enc.f32(value.IOR)
	enc.f32(value.ThicknessMinimum)
	enc.f32(value.ThicknessMaximum)
	writeTextureRef(enc, value.ThicknessTexture)
}

func readIridescence(dec *decoder) *ir.MaterialIridescence {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialIridescence{
		Factor:           dec.f32(),
		Texture:          readTextureRef(dec),
		IOR:              dec.f32(),
		ThicknessMinimum: dec.f32(),
		ThicknessMaximum: dec.f32(),
		ThicknessTexture: readTextureRef(dec),
	}
}

func writeDispersion(enc *encoder, value *ir.MaterialDispersion) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.Dispersion)
}

func readDispersion(dec *decoder) *ir.MaterialDispersion {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialDispersion{Dispersion: dec.f32()}
}

func writeDiffuseTransmission(enc *encoder, value *ir.MaterialDiffuseTransmission) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.Factor)
	writeTextureRef(enc, value.Texture)
	writeVec3(enc, value.ColorFactor)
	writeTextureRef(enc, value.ColorTexture)
}

func readDiffuseTransmission(dec *decoder) *ir.MaterialDiffuseTransmission {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialDiffuseTransmission{
		Factor:       dec.f32(),
		Texture:      readTextureRef(dec),
		ColorFactor:  readVec3(dec),
		ColorTexture: readTextureRef(dec),
	}
}

func writeSpecularGlossiness(enc *encoder, value *ir.MaterialSpecularGlossiness) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	writeVec4(enc, value.DiffuseFactor)
	writeTextureRef(enc, value.DiffuseTexture)
	writeVec3(enc, value.SpecularFactor)
	enc.f32(value.GlossinessFactor)
	writeTextureRef(enc, value.SpecularGlossinessTexture)
}

func readSpecularGlossiness(dec *decoder) *ir.MaterialSpecularGlossiness {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialSpecularGlossiness{
		DiffuseFactor:             readVec4(dec),
		DiffuseTexture:            readTextureRef(dec),
		SpecularFactor:            readVec3(dec),
		GlossinessFactor:          dec.f32(),
		SpecularGlossinessTexture: readTextureRef(dec),
	}
}

func writeEmissiveStrength(enc *encoder, value *ir.MaterialEmissiveStrength) {
	enc.bool(value != nil)
	if value == nil {
		return
	}
	enc.f32(value.EmissiveStrength)
}

func readEmissiveStrength(dec *decoder) *ir.MaterialEmissiveStrength {
	if !dec.bool() {
		return nil
	}
	return &ir.MaterialEmissiveStrength{EmissiveStrength: dec.f32()}
}
