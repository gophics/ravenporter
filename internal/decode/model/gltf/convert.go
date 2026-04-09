package gltf

import (
	"math"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
	"github.com/valyala/fastjson"
)

const (
	defaultAlphaCutoff = 0.5
	defaultMetallic    = 1.0
	defaultRoughness   = 1.0
	defaultNormalScale = 1.0
	defaultOccStrength = 1.0
	defaultIntensity   = 1.0
	defaultOuterCone   = math.Pi / 4
	defaultUnit        = 1.0
	defaultMipLevels   = 1

	alphaModeBlend = "BLEND"
	alphaModeMask  = "MASK"

	lightDirectional = "directional"
	lightPoint       = "point"
	lightSpot        = "spot"
)

var (
	defaultBaseColor = [4]float32{1, 1, 1, 1} //nolint:gochecknoglobals // glTF default
	defaultWhite3    = [3]float32{1, 1, 1}    //nolint:gochecknoglobals // glTF default
)

func (d *doc) convertMaterials() []*ir.Material {
	arr := d.root.GetArray(keyMaterials)
	if len(arr) == 0 {
		return nil
	}
	bulk := make([]ir.Material, len(arr))
	out := make([]*ir.Material, len(arr))
	for i, m := range arr {
		convertMaterial(m, &bulk[i])
		out[i] = &bulk[i]
	}
	return out
}

func convertMaterial(m *fastjson.Value, mat *ir.Material) {
	mat.Name = string(m.GetStringBytes(keyName))
	mat.AlphaCutoff = getFloat32(m, keyAlphaCutoff, defaultAlphaCutoff)
	mat.DoubleSided = m.GetBool(keyDoubleSided)

	switch decutil.Bstr(m.GetStringBytes(keyAlphaMode)) {
	case alphaModeMask:
		mat.AlphaMode = ir.AlphaMask
	case alphaModeBlend:
		mat.AlphaMode = ir.AlphaBlend
	default:
		mat.AlphaMode = ir.AlphaOpaque
	}

	if pbr := m.Get(keyPBRMetallicRoughness); pbr != nil {
		bc := pbr.GetArray(keyBaseColorFactor)
		mat.BaseColorFactor = getFloat4(bc, defaultBaseColor)
		mat.MetallicFactor = getFloat32(pbr, keyMetallicFactor, defaultMetallic)
		mat.RoughnessFactor = getFloat32(pbr, keyRoughnessFactor, defaultRoughness)
		mat.BaseColorTexture = convertTexRef(pbr.Get(keyBaseColorTexture))
		mrTex := convertTexRef(pbr.Get(keyMetRoughTexture))
		mat.MetallicTexture = mrTex
		mat.RoughnessTexture = mrTex
	}

	if nt := m.Get(keyNormalTexture); nt != nil {
		mat.NormalTexture = convertTexRef(nt)
		mat.NormalScale = getFloat32(nt, keyScale, defaultNormalScale)
	}

	if ot := m.Get(keyOcclusionTexture); ot != nil {
		mat.OcclusionTexture = convertTexRef(ot)
		mat.OcclusionStrength = getFloat32(ot, keyStrength, defaultOccStrength)
	}

	ef := m.GetArray(keyEmissiveFactor)
	mat.EmissiveFactor = getFloat3(ef, [3]float32{})
	mat.EmissiveTexture = convertTexRef(m.Get(keyEmissiveTexture))

	if ext := m.Get(keyExtensions, keyKHRMaterialsUnlit); ext != nil {
		mat.MetallicFactor = 0
		mat.RoughnessFactor = defaultRoughness
		mat.Unlit = true
	}

	parseKHRClearcoat(m, mat)
	parseKHRSheen(m, mat)
	parseKHRTransmission(m, mat)
	parseKHRVolume(m, mat)
	parseKHRIOR(m, mat)
	parseKHRSpecular(m, mat)
	parseKHRAnisotropy(m, mat)
	parseKHRIridescence(m, mat)
	parseKHRPBRSpecGloss(m, mat)
	parseKHRDispersion(m, mat)
	parseKHRDiffuseTransmission(m, mat)
	parseKHREmissiveStrength(m, mat)
}

func parseKHREmissiveStrength(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHREmissiveStrength)
	if ext == nil {
		return
	}
	mat.EmissiveStrength = &ir.MaterialEmissiveStrength{
		EmissiveStrength: getFloat32(ext, keyEmissiveStrength, 1.0),
	}
}

func parseKHRClearcoat(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRClearcoat)
	if ext == nil {
		return
	}
	cc := &ir.MaterialClearcoat{
		Factor:          getFloat32(ext, keyClearcoatFactor, 0),
		RoughnessFactor: getFloat32(ext, keyClearcoatRoughnessFactor, 0),
	}
	cc.Texture = convertTexRef(ext.Get(keyClearcoatTexture))
	cc.RoughnessTexture = convertTexRef(ext.Get(keyClearcoatRoughnessTexture))
	cc.NormalTexture = convertTexRef(ext.Get(keyClearcoatNormalTexture))
	mat.Clearcoat = cc
}

func parseKHRSheen(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRSheen)
	if ext == nil {
		return
	}
	sheen := &ir.MaterialSheen{
		ColorFactor:     getFloat3(ext.GetArray(keySheenColorFactor), [3]float32{}),
		RoughnessFactor: getFloat32(ext, keySheenRoughnessFactor, 0),
	}
	sheen.ColorTexture = convertTexRef(ext.Get(keySheenColorTexture))
	sheen.RoughnessTexture = convertTexRef(ext.Get(keySheenRoughnessTexture))
	mat.Sheen = sheen
}

func parseKHRTransmission(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRTransmission)
	if ext == nil {
		return
	}
	tm := &ir.MaterialTransmission{
		Factor: getFloat32(ext, keyTransmissionFactor, 0),
	}
	tm.Texture = convertTexRef(ext.Get(keyTransmissionTexture))
	mat.Transmission = tm
}

func parseKHRVolume(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRVolume)
	if ext == nil {
		return
	}
	vol := &ir.MaterialVolume{
		ThicknessFactor:     getFloat32(ext, keyThicknessFactor, 0),
		AttenuationDistance: getFloat32(ext, keyAttenuationDistance, 0),
		AttenuationColor:    getFloat3(ext.GetArray(keyAttenuationColor), defaultWhite3),
	}
	vol.ThicknessTexture = convertTexRef(ext.Get(keyThicknessTexture))
	mat.Volume = vol
}

const defaultIOR = 1.5

const (
	defaultIridescenceIor      = 1.3
	defaultIridescenceThickMin = 100
	defaultIridescenceThickMax = 400
)

func parseKHRIOR(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRIOR)
	if ext == nil {
		return
	}
	mat.IOR = &ir.MaterialIOR{
		IOR: getFloat32(ext, keyIOR, defaultIOR),
	}
}

func parseKHRSpecular(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRSpecular)
	if ext == nil {
		return
	}
	spec := &ir.MaterialSpecular{
		Factor:      getFloat32(ext, keySpecularFactor, 1),
		ColorFactor: getFloat3(ext.GetArray(keySpecularColorFactor), defaultWhite3),
	}
	spec.Texture = convertTexRef(ext.Get(keySpecularTexture))
	spec.ColorTexture = convertTexRef(ext.Get(keySpecularColorTexture))
	mat.Specular = spec
}

func parseKHRAnisotropy(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRAnisotropy)
	if ext == nil {
		return
	}
	aniso := &ir.MaterialAnisotropy{
		Strength: getFloat32(ext, keyAnisotropyStrength, 0),
		Rotation: getFloat32(ext, keyAnisotropyRotation, 0),
	}
	aniso.Texture = convertTexRef(ext.Get(keyAnisotropyTexture))
	mat.Anisotropy = aniso
}

func parseKHRIridescence(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRIridescence)
	if ext == nil {
		return
	}
	irid := &ir.MaterialIridescence{
		Factor:           getFloat32(ext, keyIridescenceFactor, 0),
		IOR:              getFloat32(ext, keyIridescenceIor, defaultIridescenceIor),
		ThicknessMinimum: getFloat32(ext, keyIridescenceThicknessMinimum, defaultIridescenceThickMin),
		ThicknessMaximum: getFloat32(ext, keyIridescenceThicknessMaximum, defaultIridescenceThickMax),
	}
	irid.Texture = convertTexRef(ext.Get(keyIridescenceTexture))
	irid.ThicknessTexture = convertTexRef(ext.Get(keyIridescenceThicknessTexture))
	mat.Iridescence = irid
}

func (d *doc) parseKHRMaterialVariants(asset *ir.Asset) {
	ext := d.root.Get(keyExtensions, keyKHRMatVariants)
	if ext == nil {
		return
	}
	arr := ext.GetArray(keyVariants)
	if len(arr) == 0 {
		return
	}
	names := make([]string, 0, len(arr))
	for _, v := range arr {
		names = append(names, string(v.GetStringBytes(keyName)))
	}
	if asset.Metadata.ExtraProperties == nil {
		asset.Metadata.ExtraProperties = make(map[string]string)
	}
	asset.Metadata.ExtraProperties["materialVariants"] = strings.Join(names, ",")
}

func parseKHRPBRSpecGloss(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRPBRSpecGloss)
	if ext == nil {
		return
	}
	sg := &ir.MaterialSpecularGlossiness{
		DiffuseFactor:    getFloat4(ext.GetArray(keyDiffuseFactor), [4]float32{1, 1, 1, 1}),
		SpecularFactor:   getFloat3(ext.GetArray(keySpecGlossSpecularFactor), defaultWhite3),
		GlossinessFactor: getFloat32(ext, keyGlossinessFactor, 1),
	}
	sg.DiffuseTexture = convertTexRef(ext.Get(keyDiffuseTexture))
	sg.SpecularGlossinessTexture = convertTexRef(ext.Get(keySpecularGlossinessTexture))
	mat.SpecularGlossiness = sg
}

func parseKHRDispersion(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRDispersion)
	if ext == nil {
		return
	}
	mat.Dispersion = &ir.MaterialDispersion{
		Dispersion: getFloat32(ext, keyDispersion, 0),
	}
}

func parseKHRDiffuseTransmission(m *fastjson.Value, mat *ir.Material) {
	ext := m.Get(keyExtensions, keyKHRDiffuseTransmission)
	if ext == nil {
		return
	}
	dt := &ir.MaterialDiffuseTransmission{
		Factor:      getFloat32(ext, keyDiffuseTransmissionFactor, 0),
		ColorFactor: getFloat3(ext.GetArray(keyDiffuseTransmissionColorFactor), defaultWhite3),
	}
	dt.Texture = convertTexRef(ext.Get(keyDiffuseTransmissionTexture))
	dt.ColorTexture = convertTexRef(ext.Get(keyDiffuseTransmissionColorTexture))
	mat.DiffuseTransmission = dt
}

func convertTexRef(v *fastjson.Value) *ir.TextureRef {
	if v == nil {
		return nil
	}
	ref := &ir.TextureRef{
		TextureIndex: v.GetInt(keyIndex),
		UVSet:        v.GetInt(keyTexCoord),
		Tiling:       [2]float32{1, 1},
	}
	if ext := v.Get(keyExtensions, keyKHRTextureTransform); ext != nil {
		ref.Offset = getFloat2(ext.GetArray(keyOffset), [2]float32{})
		ref.Tiling = getFloat2(ext.GetArray(keyScale), [2]float32{1, 1})
		ref.Rotation = getFloat32(ext, keyRotation, 0)
		if texCoord := ext.Get(keyTexCoord); texCoord != nil {
			ref.UVSet = texCoord.GetInt()
		}
	}
	return ref
}

func (d *doc) convertCameras() []*ir.Camera {
	arr := d.root.GetArray(keyCameras)
	if len(arr) == 0 {
		return nil
	}
	bulk := make([]ir.Camera, len(arr))
	out := make([]*ir.Camera, len(arr))
	for i, c := range arr {
		convertCamera(c, &bulk[i])
		out[i] = &bulk[i]
	}
	return out
}

func convertCamera(c *fastjson.Value, cam *ir.Camera) {
	cam.Name = string(c.GetStringBytes(keyName))

	if p := c.Get(keyPerspective); p != nil {
		cam.Perspective = &ir.PerspectiveCamera{
			FOV:  getFloat32(p, keyYfov, 0),
			Near: getFloat32(p, keyZnear, 0),
			Far:  getFloat32(p, keyZfar, 0),
		}
		if ar := p.Get(keyAspectRatio); ar != nil {
			cam.Perspective.Aspect = float32(ar.GetFloat64())
		}
	}

	if o := c.Get(keyOrthographic); o != nil {
		cam.Orthographic = &ir.OrthographicCamera{
			XMag: getFloat32(o, keyXmag, 0),
			YMag: getFloat32(o, keyYmag, 0),
			Near: getFloat32(o, keyZnear, 0),
			Far:  getFloat32(o, keyZfar, 0),
		}
	}
}

func (d *doc) convertLights() []*ir.Light {
	ext := d.root.Get(keyExtensions, keyKHRLightsPunctual)
	if ext == nil {
		return nil
	}
	arr := ext.GetArray(keyLights)
	if len(arr) == 0 {
		return nil
	}
	bulk := make([]ir.Light, len(arr))
	out := make([]*ir.Light, len(arr))
	for i, l := range arr {
		convertLight(l, &bulk[i])
		out[i] = &bulk[i]
	}
	return out
}

func convertLight(l *fastjson.Value, light *ir.Light) {
	light.Name = string(l.GetStringBytes(keyName))
	light.Color = getFloat3(l.GetArray(keyColor), defaultWhite3)
	light.Intensity = getFloat32(l, keyIntensity, defaultIntensity)

	switch decutil.Bstr(l.GetStringBytes(keyType)) {
	case lightDirectional:
		light.Directional = &ir.DirectionalLight{}
	case lightPoint:
		light.Point = &ir.PointLight{
			Range: getFloat32(l, keyRange, 0),
		}
	case lightSpot:
		spot := l.Get(keySpot)
		light.Spot = &ir.SpotLight{
			Range:          getFloat32(l, keyRange, 0),
			InnerConeAngle: getFloat32(spot, keyInnerConeAngle, 0),
			OuterConeAngle: getFloat32(spot, keyOuterConeAngle, defaultOuterCone),
		}
	}
}

func resolveLightIndex(n *fastjson.Value) int {
	ext := n.Get(keyExtensions, keyKHRLightsPunctual)
	if ext == nil {
		return ir.NoIndex
	}
	lightVal := ext.Get(keyLight)
	if lightVal == nil {
		return ir.NoIndex
	}
	return lightVal.GetInt()
}

func (d *doc) convertSkins() []*ir.Skeleton {
	arr := d.root.GetArray(keySkins)
	if len(arr) == 0 {
		return nil
	}
	bulk := make([]ir.Skeleton, len(arr))
	out := make([]*ir.Skeleton, len(arr))
	for i, s := range arr {
		d.convertSkin(s, &bulk[i])
		out[i] = &bulk[i]
	}
	return out
}

func (d *doc) convertSkin(s *fastjson.Value, skel *ir.Skeleton) {
	joints := getIntSlice(s, keyJoints)

	skel.Name = string(s.GetStringBytes(keyName))
	skel.Joints = joints
	skel.RootIdx = 0

	if skelVal := s.Get(keySkeleton); skelVal != nil {
		skelIdx := skelVal.GetInt()
		for i, j := range joints {
			if j == skelIdx {
				skel.RootIdx = i
				break
			}
		}
	}

	if ibmVal := s.Get(keyInverseBindMatrices); ibmVal != nil {
		a := d.getAccessor(ibmVal.GetInt())
		skel.InverseBindMatrices = d.bufs.readMat4s(a)
	}
}

func markJointNodes(nodes []ir.Node, skeletons []*ir.Skeleton) {
	for _, s := range skeletons {
		for _, j := range s.Joints {
			if j >= 0 && j < len(nodes) {
				nodes[j].IsJoint = true
			}
		}
	}
}

//nolint:gocritic // unnamedResult: returns are assigned in body
func (d *doc) convertNodes(asset *ir.Asset) ([]ir.Node, []int) {
	arr := d.root.GetArray(keyNodes)
	nodes := make([]ir.Node, len(arr))
	for i, n := range arr {
		nodes[i] = convertNode(n)
		d.expandInstancedNode(n, i, &nodes)

		if ext := n.Get(keyExtensions, keyMSFTLOD); ext != nil {
			ids := getIntSlice(ext, keyIDs)
			if len(ids) > 0 {
				levelNodeIDs := make([]int, 0, len(ids)+1)
				levelNodeIDs = append(levelNodeIDs, i)
				for _, id := range ids {
					if id >= 0 && id < len(arr) {
						levelNodeIDs = append(levelNodeIDs, id)
					}
				}
				if len(levelNodeIDs) == 0 {
					continue
				}

				thresholds := lodThresholds(n, len(levelNodeIDs))
				lodIdx := len(asset.LODGroups)
				asset.LODGroups = append(asset.LODGroups, &ir.LODGroup{Name: nodes[i].Name})
				for levelIdx, nodeIdx := range levelNodeIDs {
					level := ir.LODLevel{NodeIndex: nodeIdx}
					if levelIdx < len(thresholds) {
						level.Threshold = thresholds[levelIdx]
					}
					asset.LODGroups[lodIdx].Levels = append(asset.LODGroups[lodIdx].Levels, level)
				}
				// The nodes themselves must point back to the LOD group.
				// Since we haven't finished building the node slice yet, assign the
				// LODGroupIndex in a second pass.
			}
		}
	}

	// Second pass: assign LODGroupIndex to children of LOD groups
	for lodIdx, lg := range asset.LODGroups {
		for _, level := range lg.Levels {
			if level.NodeIndex >= 0 && level.NodeIndex < len(nodes) {
				nodes[level.NodeIndex].LODGroupIndex = lodIdx
			}
		}
	}

	return nodes, d.defaultRoots(nodes)
}

func (d *doc) expandInstancedNode(raw *fastjson.Value, nodeIndex int, nodes *[]ir.Node) {
	node := (*nodes)[nodeIndex]
	if node.MeshIndex == ir.NoIndex {
		return
	}

	translations, rotations, scales, count := d.readInstancing(raw)
	if count == 0 {
		return
	}

	instanceChildren := make([]int, 0, count+len(node.Children))
	for i := range count {
		child := ir.Node{
			Name:         instanceNodeName(node.Name, i, count),
			Transform:    instanceTransform(translations, rotations, scales, i),
			Visible:      node.Visible,
			Mobility:     node.Mobility,
			MeshIndex:    node.MeshIndex,
			SkinIndex:    node.SkinIndex,
			IsCollision:  node.IsCollision,
			MorphWeights: append([]float32(nil), node.MorphWeights...),
		}
		instanceChildren = append(instanceChildren, len(*nodes))
		*nodes = append(*nodes, child)
	}

	node.MeshIndex = ir.NoIndex
	node.SkinIndex = ir.NoIndex
	node.MorphWeights = nil
	node.Children = append(instanceChildren, node.Children...)
	(*nodes)[nodeIndex] = node
}

func (d *doc) readInstancing(node *fastjson.Value) (
	translations [][3]float32,
	rotations [][4]float32,
	scales [][3]float32,
	count int,
) {
	ext := node.Get(keyExtensions, keyEXTMeshGPUInst)
	if ext == nil {
		return nil, nil, nil, 0
	}
	attrs := ext.Get(keyAttributesInst)
	if attrs == nil {
		return nil, nil, nil, 0
	}

	translations = d.readInstancedVec3s(attrs, attrInstTranslation)
	rotations = d.readInstancedVec4s(attrs, attrInstRotation)
	scales = d.readInstancedVec3s(attrs, attrInstScale)

	count = instancingCount(len(translations), len(rotations), len(scales))
	return translations, rotations, scales, count
}

func (d *doc) readInstancedVec3s(attrs *fastjson.Value, key string) [][3]float32 {
	val := attrs.Get(key)
	if val == nil {
		return nil
	}
	return d.bufs.readVec3s(d.getAccessor(val.GetInt()))
}

func (d *doc) readInstancedVec4s(attrs *fastjson.Value, key string) [][4]float32 {
	val := attrs.Get(key)
	if val == nil {
		return nil
	}
	return d.bufs.readVec4s(d.getAccessor(val.GetInt()))
}

func instancingCount(lengths ...int) int {
	count := 0
	for _, length := range lengths {
		if length <= 0 {
			continue
		}
		if count == 0 || length < count {
			count = length
		}
	}
	return count
}

func instanceTransform(translations [][3]float32, rotations [][4]float32, scales [][3]float32, index int) ir.Transform {
	var transform ir.Transform
	if index < len(translations) {
		transform.Translation = translations[index]
	}
	if index < len(rotations) {
		transform.Rotation = rotations[index]
	}
	if index < len(scales) {
		transform.Scale = scales[index]
	}
	return transform
}

func instanceNodeName(name string, index, count int) string {
	if count == 1 || name == "" {
		return name
	}
	return name + "_instance_" + strconv.Itoa(index)
}

func convertNode(n *fastjson.Value) ir.Node {
	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        string(n.GetStringBytes(keyName)),
		MeshIndex:   getOptionalInt(n, keyMesh),
		SkinIndex:   getOptionalInt(n, keySkin),
		CameraIndex: getOptionalInt(n, keyCamera),
		LightIndex:  resolveLightIndex(n),
		Children:    getIntSlice(n, keyChildren),
	}

	node.Transform = extractTransform(n)
	node.MorphWeights = getFloat32Slice(n, keyWeights)

	return node
}

func extractTransform(n *fastjson.Value) ir.Transform {
	mat := n.GetArray(keyMatrix)
	if len(mat) == elemMat4 {
		var m [16]float32
		for i, v := range mat {
			m[i] = float32(v.GetFloat64())
		}
		if m != mathx.Ident4() && m != [16]float32{} {
			return ir.Transform{Matrix: m}
		}
	}

	t := getFloat3(n.GetArray(keyTranslation), [3]float32{})
	r := getFloat4(n.GetArray(keyRotation), mathx.IdentityQuat)
	s := getFloat3(n.GetArray(keyScale), mathx.IdentityScale)

	return ir.Transform{
		Translation: t,
		Rotation:    r,
		Scale:       s,
	}
}

func (d *doc) defaultRoots(nodes []ir.Node) []int {
	sceneVal := d.root.Get(keyScene)
	if sceneVal != nil {
		sceneIdx := sceneVal.GetInt()
		scenes := d.root.GetArray(keyScenes)
		if sceneIdx >= 0 && sceneIdx < len(scenes) {
			return getIntSlice(scenes[sceneIdx], keyNodes)
		}
	}

	isChild := make([]bool, len(nodes))
	for i := range nodes {
		for _, idx := range nodes[i].Children {
			if idx >= 0 && idx < len(nodes) {
				isChild[idx] = true
			}
		}
	}

	roots := make([]int, 0, len(nodes))
	for i := range nodes {
		if !isChild[i] {
			roots = append(roots, i)
		}
	}
	return roots
}

func getOptionalInt(v *fastjson.Value, key string) int {
	f := v.Get(key)
	if f == nil {
		return ir.NoIndex
	}
	return f.GetInt()
}

func getFloat32(v *fastjson.Value, key string, def float32) float32 {
	f := v.Get(key)
	if f == nil {
		return def
	}
	return float32(f.GetFloat64())
}

func getFloat3(arr []*fastjson.Value, def [3]float32) [3]float32 {
	if len(arr) < elemVec3 {
		return def
	}
	return [3]float32{
		float32(arr[0].GetFloat64()),
		float32(arr[1].GetFloat64()),
		float32(arr[2].GetFloat64()),
	}
}

func getFloat2(arr []*fastjson.Value, def [2]float32) [2]float32 {
	if len(arr) < elemVec2 {
		return def
	}
	return [2]float32{
		float32(arr[0].GetFloat64()),
		float32(arr[1].GetFloat64()),
	}
}

func getFloat4(arr []*fastjson.Value, def [4]float32) [4]float32 {
	if len(arr) < elemVec4 {
		return def
	}
	return [4]float32{
		float32(arr[0].GetFloat64()),
		float32(arr[1].GetFloat64()),
		float32(arr[2].GetFloat64()),
		float32(arr[3].GetFloat64()),
	}
}

func getIntSlice(v *fastjson.Value, key string) []int {
	arr := v.GetArray(key)
	if len(arr) == 0 {
		return nil
	}
	out := make([]int, len(arr))
	for i, a := range arr {
		out[i] = a.GetInt()
	}
	return out
}

func getFloat32Slice(v *fastjson.Value, key string) []float32 {
	arr := v.GetArray(key)
	if len(arr) == 0 {
		return nil
	}
	return float32Array(arr)
}

func lodThresholds(n *fastjson.Value, levelCount int) []float32 {
	if levelCount <= 0 {
		return nil
	}
	extras := n.Get(keyExtras, keyMSFTScreenCoverage)
	if extras == nil {
		return nil
	}
	arr := extras.GetArray()
	if len(arr) == 0 {
		return nil
	}
	thresholds := float32Array(arr)
	if len(thresholds) > levelCount {
		thresholds = thresholds[:levelCount]
	}
	return thresholds
}

func float32Array(arr []*fastjson.Value) []float32 {
	if len(arr) == 0 {
		return nil
	}
	out := make([]float32, len(arr))
	for i, a := range arr {
		out[i] = float32(a.GetFloat64())
	}
	return out
}
