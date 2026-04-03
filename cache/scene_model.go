package cache

import "github.com/gophics/ravenporter/ir"

const (
	minOptionalEntryBytes    = 1
	minPrimitiveBytes        = 20
	minMorphTargetBytes      = 16
	minAnimationChannelBytes = 24
	minSkeletonMatrixBytes   = 64
)

func writeMeshSlice(enc *encoder, meshes []*ir.Mesh) {
	enc.count(len(meshes))
	for _, mesh := range meshes {
		enc.bool(mesh != nil)
		if mesh == nil {
			continue
		}
		enc.string(mesh.Name)
		enc.float32s(mesh.MorphWeights)
		writeBounds(enc, mesh.BoundingBox)
		enc.count(len(mesh.Primitives))
		for i := range mesh.Primitives {
			writePrimitive(enc, mesh.Primitives[i])
		}
	}
}

func readMeshSlice(dec *decoder) []*ir.Mesh {
	count := dec.count(minOptionalEntryBytes)
	meshes := make([]*ir.Mesh, count)
	for i := range meshes {
		if !dec.bool() {
			continue
		}
		mesh := &ir.Mesh{
			Name:         dec.string(),
			MorphWeights: dec.float32s(),
			BoundingBox:  readBounds(dec),
		}
		primCount := dec.count(minPrimitiveBytes)
		mesh.Primitives = make([]ir.Primitive, primCount)
		for j := range mesh.Primitives {
			mesh.Primitives[j] = readPrimitive(dec)
		}
		meshes[i] = mesh
	}
	return meshes
}

func writePrimitive(enc *encoder, primitive ir.Primitive) {
	enc.int(int(primitive.Mode))
	enc.int(primitive.MaterialIndex)
	writeMeshData(enc, primitive.Data)
	enc.count(len(primitive.MorphTargets))
	for i := range primitive.MorphTargets {
		writeMorphTarget(enc, primitive.MorphTargets[i])
	}
}

func readPrimitive(dec *decoder) ir.Primitive {
	primitive := ir.Primitive{
		Mode:          ir.PrimitiveMode(dec.i32()),
		MaterialIndex: int(dec.i32()),
		Data:          readMeshData(dec),
	}
	count := dec.count(minMorphTargetBytes)
	primitive.MorphTargets = make([]ir.MorphTarget, count)
	for i := range primitive.MorphTargets {
		primitive.MorphTargets[i] = readMorphTarget(dec)
	}
	return primitive
}

func writeMeshData(enc *encoder, data ir.MeshData) {
	enc.int(data.VertexCount)
	writeVec3Slice(enc, data.Positions)
	enc.uint32s(data.Indices)
	writeVec3Slice(enc, data.Normals)
	writeVec4Slice(enc, data.Tangents)
	writeVec2Slice(enc, data.TexCoord0)
	writeVec2Slice(enc, data.TexCoord1)
	writeVec2Slice(enc, data.TexCoord2)
	writeVec2Slice(enc, data.TexCoord3)
	writeColorSlice(enc, data.Colors0)
	writeJointSlice(enc, data.Joints0)
	writeJointSlice(enc, data.Joints1)
	writeWeightSlice(enc, data.Weights0)
	writeWeightSlice(enc, data.Weights1)
	enc.ints(data.SmoothGroups)
}

func readMeshData(dec *decoder) ir.MeshData {
	return ir.MeshData{
		VertexCount:  int(dec.i32()),
		Positions:    readVec3Slice(dec),
		Indices:      dec.uint32s(),
		Normals:      readVec3Slice(dec),
		Tangents:     readVec4Slice(dec),
		TexCoord0:    readVec2Slice(dec),
		TexCoord1:    readVec2Slice(dec),
		TexCoord2:    readVec2Slice(dec),
		TexCoord3:    readVec2Slice(dec),
		Colors0:      readColorSlice(dec),
		Joints0:      readJointSlice(dec),
		Joints1:      readJointSlice(dec),
		Weights0:     readWeightSlice(dec),
		Weights1:     readWeightSlice(dec),
		SmoothGroups: dec.ints(),
	}
}

func writeMorphTarget(enc *encoder, target ir.MorphTarget) {
	enc.string(target.Name)
	enc.uint32s(target.Indices)
	writeVec3Slice(enc, target.Positions)
	writeVec3Slice(enc, target.Normals)
	writeVec3Slice(enc, target.Tangents)
}

func readMorphTarget(dec *decoder) ir.MorphTarget {
	return ir.MorphTarget{
		Name:      dec.string(),
		Indices:   dec.uint32s(),
		Positions: readVec3Slice(dec),
		Normals:   readVec3Slice(dec),
		Tangents:  readVec3Slice(dec),
	}
}

func writeMaterialSlice(enc *encoder, materials []*ir.Material) {
	enc.count(len(materials))
	for _, material := range materials {
		enc.bool(material != nil)
		if material == nil {
			continue
		}
		writeMaterial(enc, material)
	}
}

func readMaterialSlice(dec *decoder) ([]*ir.Material, error) {
	count := dec.count(minOptionalEntryBytes)
	materials := make([]*ir.Material, count)
	for i := range materials {
		if !dec.bool() {
			continue
		}
		material, err := readMaterial(dec)
		if err != nil {
			return nil, err
		}
		materials[i] = material
	}
	return materials, nil
}

func writeMaterial(enc *encoder, material *ir.Material) {
	enc.string(material.Name)
	writeVec4(enc, material.BaseColorFactor)
	writeTextureRef(enc, material.BaseColorTexture)
	enc.f32(material.MetallicFactor)
	enc.f32(material.RoughnessFactor)
	writeTextureRef(enc, material.MetallicTexture)
	writeTextureRef(enc, material.RoughnessTexture)
	writeTextureRef(enc, material.NormalTexture)
	enc.f32(material.NormalScale)
	writeTextureRef(enc, material.OcclusionTexture)
	enc.f32(material.OcclusionStrength)
	writeVec3(enc, material.EmissiveFactor)
	writeTextureRef(enc, material.EmissiveTexture)
	enc.int(int(material.AlphaMode))
	enc.f32(material.AlphaCutoff)
	enc.bool(material.DoubleSided)
	enc.bool(material.Unlit)
	writeClearcoat(enc, material.Clearcoat)
	writeSheen(enc, material.Sheen)
	writeTransmission(enc, material.Transmission)
	writeVolume(enc, material.Volume)
	writeIOR(enc, material.IOR)
	writeSpecular(enc, material.Specular)
	writeAnisotropy(enc, material.Anisotropy)
	writeIridescence(enc, material.Iridescence)
	writeDispersion(enc, material.Dispersion)
	writeDiffuseTransmission(enc, material.DiffuseTransmission)
	writeSpecularGlossiness(enc, material.SpecularGlossiness)
	writeEmissiveStrength(enc, material.EmissiveStrength)
	writeMaterialProperties(enc, material.Properties)
}

func readMaterial(dec *decoder) (*ir.Material, error) {
	material := &ir.Material{
		Name:                dec.string(),
		BaseColorFactor:     readVec4(dec),
		BaseColorTexture:    readTextureRef(dec),
		MetallicFactor:      dec.f32(),
		RoughnessFactor:     dec.f32(),
		MetallicTexture:     readTextureRef(dec),
		RoughnessTexture:    readTextureRef(dec),
		NormalTexture:       readTextureRef(dec),
		NormalScale:         dec.f32(),
		OcclusionTexture:    readTextureRef(dec),
		OcclusionStrength:   dec.f32(),
		EmissiveFactor:      readVec3(dec),
		EmissiveTexture:     readTextureRef(dec),
		AlphaMode:           ir.AlphaMode(dec.i32()),
		AlphaCutoff:         dec.f32(),
		DoubleSided:         dec.bool(),
		Unlit:               dec.bool(),
		Clearcoat:           readClearcoat(dec),
		Sheen:               readSheen(dec),
		Transmission:        readTransmission(dec),
		Volume:              readVolume(dec),
		IOR:                 readIOR(dec),
		Specular:            readSpecular(dec),
		Anisotropy:          readAnisotropy(dec),
		Iridescence:         readIridescence(dec),
		Dispersion:          readDispersion(dec),
		DiffuseTransmission: readDiffuseTransmission(dec),
		SpecularGlossiness:  readSpecularGlossiness(dec),
		EmissiveStrength:    readEmissiveStrength(dec),
	}
	properties, err := readMaterialProperties(dec)
	if err != nil {
		return nil, err
	}
	material.Properties = properties
	return material, nil
}

func writeTextureSlice(enc *encoder, textures []*ir.Texture) {
	enc.count(len(textures))
	for _, texture := range textures {
		enc.bool(texture != nil)
		if texture == nil {
			continue
		}
		enc.string(texture.Name)
		enc.int(texture.ImageIndex)
		enc.int(texture.MipLevels)
		enc.int(int(texture.WrapS))
		enc.int(int(texture.WrapT))
		enc.int(int(texture.MinFilter))
		enc.int(int(texture.MagFilter))
		writeStringMap(enc, texture.Metadata)
	}
}

func readTextureSlice(dec *decoder) []*ir.Texture {
	count := dec.count(minOptionalEntryBytes)
	textures := make([]*ir.Texture, count)
	for i := range textures {
		if !dec.bool() {
			continue
		}
		texture := &ir.Texture{
			Name:       dec.string(),
			ImageIndex: int(dec.i32()),
		}
		texture.MipLevels = int(dec.i32())
		texture.WrapS = ir.TextureWrap(dec.i32())
		texture.WrapT = ir.TextureWrap(dec.i32())
		texture.MinFilter = ir.TextureFilter(dec.i32())
		texture.MagFilter = ir.TextureFilter(dec.i32())
		texture.Metadata = readStringMap(dec)
		textures[i] = texture
	}
	return textures
}

func writeAnimationSlice(enc *encoder, animations []*ir.Animation) {
	enc.count(len(animations))
	for _, animation := range animations {
		enc.bool(animation != nil)
		if animation == nil {
			continue
		}
		enc.string(animation.Name)
		enc.f64(animation.Duration)
		enc.count(len(animation.Channels))
		for i := range animation.Channels {
			writeAnimationChannel(enc, animation.Channels[i])
		}
	}
}

func readAnimationSlice(dec *decoder) []*ir.Animation {
	count := dec.count(minOptionalEntryBytes)
	animations := make([]*ir.Animation, count)
	for i := range animations {
		if !dec.bool() {
			continue
		}
		animation := &ir.Animation{
			Name:     dec.string(),
			Duration: dec.f64(),
		}
		channelCount := dec.count(minAnimationChannelBytes)
		animation.Channels = make([]ir.AnimationChannel, channelCount)
		for j := range animation.Channels {
			animation.Channels[j] = readAnimationChannel(dec)
		}
		animations[i] = animation
	}
	return animations
}

func writeAnimationChannel(enc *encoder, channel ir.AnimationChannel) {
	enc.int(channel.NodeIndex)
	enc.int(int(channel.Target))
	enc.int(int(channel.Interpolation))
	enc.float32s(channel.Times)
	enc.string(channel.Pointer)
	enc.int(channel.MaterialIndex)
	enc.float32s(channel.Values)
	writeVec3Slice(enc, channel.Translations)
	writeVec4Slice(enc, channel.Rotations)
	writeVec3Slice(enc, channel.Scales)
	enc.float32s(channel.Weights)
}

func readAnimationChannel(dec *decoder) ir.AnimationChannel {
	return ir.AnimationChannel{
		NodeIndex:     int(dec.i32()),
		Target:        ir.ChannelTarget(dec.i32()),
		Interpolation: ir.Interpolation(dec.i32()),
		Times:         dec.float32s(),
		Pointer:       dec.string(),
		MaterialIndex: int(dec.i32()),
		Values:        dec.float32s(),
		Translations:  readVec3Slice(dec),
		Rotations:     readVec4Slice(dec),
		Scales:        readVec3Slice(dec),
		Weights:       dec.float32s(),
	}
}

func writeSkeletonSlice(enc *encoder, skeletons []*ir.Skeleton) {
	enc.count(len(skeletons))
	for _, skeleton := range skeletons {
		enc.bool(skeleton != nil)
		if skeleton == nil {
			continue
		}
		enc.string(skeleton.Name)
		enc.ints(skeleton.Joints)
		enc.int(skeleton.RootIdx)
		enc.count(len(skeleton.InverseBindMatrices))
		for _, matrix := range skeleton.InverseBindMatrices {
			writeMat4(enc, matrix)
		}
	}
}

func readSkeletonSlice(dec *decoder) []*ir.Skeleton {
	count := dec.count(minOptionalEntryBytes)
	skeletons := make([]*ir.Skeleton, count)
	for i := range skeletons {
		if !dec.bool() {
			continue
		}
		skeleton := &ir.Skeleton{
			Name:    dec.string(),
			Joints:  dec.ints(),
			RootIdx: int(dec.i32()),
		}
		matrixCount := dec.count(minSkeletonMatrixBytes)
		skeleton.InverseBindMatrices = make([][16]float32, matrixCount)
		for j := range skeleton.InverseBindMatrices {
			skeleton.InverseBindMatrices[j] = readMat4(dec)
		}
		skeletons[i] = skeleton
	}
	return skeletons
}

func writeCameraSlice(enc *encoder, cameras []*ir.Camera) {
	enc.count(len(cameras))
	for _, camera := range cameras {
		enc.bool(camera != nil)
		if camera == nil {
			continue
		}
		enc.string(camera.Name)
		enc.bool(camera.Perspective != nil)
		if camera.Perspective != nil {
			enc.f32(camera.Perspective.FOV)
			enc.f32(camera.Perspective.Aspect)
			enc.f32(camera.Perspective.Near)
			enc.f32(camera.Perspective.Far)
			enc.f32(camera.Perspective.FocalLength)
			enc.f32(camera.Perspective.FocusDistance)
			enc.f32(camera.Perspective.FStop)
			enc.f32(camera.Perspective.SensorWidth)
			enc.f32(camera.Perspective.SensorHeight)
		}
		enc.bool(camera.Orthographic != nil)
		if camera.Orthographic != nil {
			enc.f32(camera.Orthographic.XMag)
			enc.f32(camera.Orthographic.YMag)
			enc.f32(camera.Orthographic.Near)
			enc.f32(camera.Orthographic.Far)
		}
	}
}

func readCameraSlice(dec *decoder) []*ir.Camera {
	count := dec.count(minOptionalEntryBytes)
	cameras := make([]*ir.Camera, count)
	for i := range cameras {
		if !dec.bool() {
			continue
		}
		camera := &ir.Camera{Name: dec.string()}
		if dec.bool() {
			camera.Perspective = &ir.PerspectiveCamera{
				FOV:           dec.f32(),
				Aspect:        dec.f32(),
				Near:          dec.f32(),
				Far:           dec.f32(),
				FocalLength:   dec.f32(),
				FocusDistance: dec.f32(),
				FStop:         dec.f32(),
				SensorWidth:   dec.f32(),
				SensorHeight:  dec.f32(),
			}
		}
		if dec.bool() {
			camera.Orthographic = &ir.OrthographicCamera{
				XMag: dec.f32(),
				YMag: dec.f32(),
				Near: dec.f32(),
				Far:  dec.f32(),
			}
		}
		cameras[i] = camera
	}
	return cameras
}

func writeLightSlice(enc *encoder, lights []*ir.Light) {
	enc.count(len(lights))
	for _, light := range lights {
		enc.bool(light != nil)
		if light == nil {
			continue
		}
		enc.string(light.Name)
		writeVec3(enc, light.Color)
		enc.f32(light.Temperature)
		enc.f32(light.Intensity)
		writeTextureRef(enc, light.IESProfile)
		enc.bool(light.Directional != nil)
		enc.bool(light.Point != nil)
		if light.Point != nil {
			enc.f32(light.Point.Range)
			enc.f32(light.Point.SourceRadius)
			enc.f32(light.Point.SourceLength)
		}
		enc.bool(light.Spot != nil)
		if light.Spot != nil {
			enc.f32(light.Spot.Range)
			enc.f32(light.Spot.InnerConeAngle)
			enc.f32(light.Spot.OuterConeAngle)
			enc.f32(light.Spot.SourceRadius)
		}
	}
}

func readLightSlice(dec *decoder) []*ir.Light {
	count := dec.count(minOptionalEntryBytes)
	lights := make([]*ir.Light, count)
	for i := range lights {
		if !dec.bool() {
			continue
		}
		light := &ir.Light{
			Name:        dec.string(),
			Color:       readVec3(dec),
			Temperature: dec.f32(),
			Intensity:   dec.f32(),
			IESProfile:  readTextureRef(dec),
		}
		if dec.bool() {
			light.Directional = &ir.DirectionalLight{}
		}
		if dec.bool() {
			light.Point = &ir.PointLight{
				Range:        dec.f32(),
				SourceRadius: dec.f32(),
				SourceLength: dec.f32(),
			}
		}
		if dec.bool() {
			light.Spot = &ir.SpotLight{
				Range:          dec.f32(),
				InnerConeAngle: dec.f32(),
				OuterConeAngle: dec.f32(),
				SourceRadius:   dec.f32(),
			}
		}
		lights[i] = light
	}
	return lights
}
