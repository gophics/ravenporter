package fbx

import (
	"strings"

	"github.com/gophics/ravenporter/ir"
)

func convertMaterial(node *fbxNode) *ir.Material {
	mat := &ir.Material{
		Name:            extractName(node),
		BaseColorFactor: defaultBaseColor,
		MetallicFactor:  0,
		RoughnessFactor: defaultRoughness,
		AlphaMode:       ir.AlphaOpaque,
	}

	p70 := findNode(node.children, nodeProperties70)
	if p70 == nil {
		return mat
	}

	for _, p := range p70.children {
		if p.name != nodeP || len(p.properties) <= propMinIndex+2 {
			continue
		}
		name := p.properties[0].strVal
		r := float32(p.properties[propMinIndex].floatVal)
		g := float32(p.properties[propMinIndex+1].floatVal)
		b := float32(p.properties[propMinIndex+2].floatVal)

		switch name {
		case fbxPropDiffuseColor:
			mat.BaseColorFactor = [4]float32{r, g, b, defaultBaseColor[3]}
		case fbxPropEmissiveColor:
			mat.EmissiveFactor = [3]float32{r, g, b}
		case fbxPropSpecularColor:
			if mat.Properties == nil {
				mat.Properties = make(map[string]any)
			}
			mat.Properties[fbxPropSpecular] = [3]float32{r, g, b}
		case fbxPropAmbientColor:
			if mat.Properties == nil {
				mat.Properties = make(map[string]any)
			}
			mat.Properties[fbxPropAmbient] = [3]float32{r, g, b}
		}
	}

	return mat
}

func convertTexture(node *fbxNode) (*ir.ImageAsset, *ir.Texture) {
	var path string
	for _, child := range node.children {
		switch child.name {
		case texRelFilename:
			if len(child.properties) > 0 && child.properties[0].strVal != "" {
				path = child.properties[0].strVal
			}
		case texFileName:
			if path == "" && len(child.properties) > 0 {
				path = child.properties[0].strVal
			}
		}
	}
	if path == "" {
		return nil, nil
	}
	return &ir.ImageAsset{
			Name:       extractName(node),
			SourcePath: path,
		}, &ir.Texture{
			Name: extractName(node),
		}
}

func convertFBXCamera(node *fbxNode) *ir.Camera {
	cam := &ir.Camera{
		Name: extractName(node),
		Perspective: &ir.PerspectiveCamera{
			FOV:  float32(defaultCamFOV * degToRad),
			Near: defaultNear,
			Far:  defaultFar,
		},
	}
	p70 := findNode(node.children, nodeProperties70)
	if p70 == nil {
		return cam
	}
	for _, p := range p70.children {
		if p.name != nodeP || len(p.properties) <= propMinIndex {
			continue
		}
		name := p.properties[0].strVal
		val := float32(p.properties[propMinIndex].floatVal)
		switch name {
		case propFOV, propFOVX:
			cam.Perspective.FOV = val * float32(degToRad)
		case propNearPlane:
			cam.Perspective.Near = val
		case propFarPlane:
			cam.Perspective.Far = val
		}
	}
	return cam
}

func convertFBXLight(node *fbxNode) *ir.Light {
	light := &ir.Light{
		Name:      extractName(node),
		Color:     defaultLightColor,
		Intensity: defaultIntensity,
		Point:     &ir.PointLight{},
	}
	p70 := findNode(node.children, nodeProperties70)
	if p70 == nil {
		return light
	}
	var lightType int
	var innerAngle, outerAngle float32
	for _, p := range p70.children {
		if p.name != nodeP || len(p.properties) <= propMinIndex {
			continue
		}
		name := p.properties[0].strVal
		switch name {
		case propLightColor:
			if len(p.properties) > propMinIndex+2 {
				light.Color = [3]float32{
					float32(p.properties[propMinIndex].floatVal),
					float32(p.properties[propMinIndex+1].floatVal),
					float32(p.properties[propMinIndex+2].floatVal),
				}
			}
		case propIntensity:
			light.Intensity = float32(p.properties[propMinIndex].floatVal) / fbxIntensityScale
		case propLightType:
			lightType = int(p.properties[propMinIndex].intVal)
		case propInnerAngle:
			innerAngle = float32(p.properties[propMinIndex].floatVal) * float32(degToRad)
		case propOuterAngle:
			outerAngle = float32(p.properties[propMinIndex].floatVal) * float32(degToRad)
		}
	}
	switch lightType {
	case 0:
		light.Point = &ir.PointLight{}
	case 1:
		light.Point = nil
		light.Directional = &ir.DirectionalLight{}
	case 2: //nolint:mnd // FBX LightType: Spot
		light.Point = nil
		light.Spot = &ir.SpotLight{
			InnerConeAngle: innerAngle,
			OuterConeAngle: outerAngle,
		}
	}
	return light
}

func applyTextureConnection(mat *ir.Material, propName string, texIdx int) {
	const texturePropertyCap = 2

	switch propName {
	case fbxPropDiffuseColor, "":
		mat.BaseColorTexture = &ir.TextureRef{TextureIndex: texIdx, Tiling: defaultTiling}
	case fbxPropEmissiveColor:
		mat.EmissiveTexture = &ir.TextureRef{TextureIndex: texIdx, Tiling: defaultTiling}
	case fbxPropNormalMap:
		mat.NormalTexture = &ir.TextureRef{TextureIndex: texIdx, Tiling: defaultTiling}
		mat.NormalScale = defaultNormalScale
	case fbxPropAmbientColor:
		if mat.Properties == nil {
			mat.Properties = make(map[string]any, texturePropertyCap)
		}
		mat.Properties[fbxPropAmbientTexture] = texIdx
	case fbxPropSpecularColor:
		if mat.Properties == nil {
			mat.Properties = make(map[string]any, texturePropertyCap)
		}
		mat.Properties[fbxPropSpecularTexture] = texIdx
	}
}

func parseConnections(node *fbxNode) []connection {
	if node == nil {
		return nil
	}
	var conns []connection
	for _, child := range node.children {
		if len(child.properties) < vecStride {
			continue
		}
		typ := child.properties[0].strVal
		c := connection{
			childID:  child.properties[1].intVal,
			parentID: child.properties[2].intVal,
		}
		switch typ {
		case connOO:
			conns = append(conns, c)
		case connOP:
			if len(child.properties) > vecStride {
				c.propName = child.properties[vecStride].strVal
			}
			conns = append(conns, c)
		}
	}
	return conns
}

func resolveConnections(
	asset *ir.Asset, conns []connection,
	geoMap, matMap, modelMap, texMap, camMap, lightMap map[int64]int,
	videoMap map[int64][]byte,
	lodGroupMap map[int64]int,
) {
	for _, c := range conns {
		modelIdx, isModel := modelMap[c.parentID]
		if !isModel {
			continue
		}

		if geoIdx, ok := geoMap[c.childID]; ok {
			asset.Nodes[modelIdx].MeshIndex = geoIdx
		}
		if matIdx, ok := matMap[c.childID]; ok {
			assignMaterial(asset, modelIdx, matIdx)
		}
		if camIdx, ok := camMap[c.childID]; ok {
			asset.Nodes[modelIdx].CameraIndex = camIdx
		}
		if lightIdx, ok := lightMap[c.childID]; ok {
			asset.Nodes[modelIdx].LightIndex = lightIdx
		}
	}

	for _, c := range conns {
		matIdx, isMat := matMap[c.parentID]
		texIdx, isTex := texMap[c.childID]
		if !isMat || !isTex || matIdx >= len(asset.Materials) {
			continue
		}
		applyTextureConnection(asset.Materials[matIdx], c.propName, texIdx)
	}

	parentSet := make(map[int]bool, len(asset.Nodes))
	for _, c := range conns {
		parentIdx, parentIsModel := modelMap[c.parentID]
		childIdx, childIsModel := modelMap[c.childID]
		if parentIsModel && childIsModel {
			asset.Nodes[parentIdx].Children = append(asset.Nodes[parentIdx].Children, childIdx)
			parentSet[childIdx] = true

			if lodIdx, isLOD := lodGroupMap[c.parentID]; isLOD {
				asset.Nodes[childIdx].LODGroupIndex = lodIdx
				asset.LODGroups[lodIdx].Levels = append(asset.LODGroups[lodIdx].Levels, ir.LODLevel{
					Threshold: 0.0,
					NodeIndex: childIdx,
				})
			}
		}
	}

	for i := range asset.Nodes {
		if !parentSet[i] {
			asset.RootNodes = append(asset.RootNodes, i)
		}
	}

	for _, c := range conns {
		texIdx, isTex := texMap[c.parentID]
		data, hasData := videoMap[c.childID]
		if !isTex || !hasData || texIdx >= len(asset.Textures) {
			continue
		}
		imageIdx := asset.Textures[texIdx].ImageIndex
		if imageIdx < 0 || imageIdx >= len(asset.Images) {
			continue
		}
		asset.Images[imageIdx].Compressed = append([]byte(nil), data...)
		asset.Images[imageIdx].SourcePath = ""
	}
}

func assignMaterial(asset *ir.Asset, nodeIdx, matIdx int) {
	meshIdx := asset.Nodes[nodeIdx].MeshIndex
	if meshIdx < 0 || meshIdx >= len(asset.Meshes) {
		return
	}
	mesh := asset.Meshes[meshIdx]
	for i := range mesh.Primitives {
		if mesh.Primitives[i].MaterialIndex == ir.NoIndex {
			mesh.Primitives[i].MaterialIndex = matIdx
		}
	}
}

func appendCollisionMeshes(asset *ir.Asset) {
	for i := range asset.Nodes {
		node := &asset.Nodes[i]
		if !node.IsCollision {
			continue
		}
		collisionType, ok := collisionTypeFromName(node.Name)
		if !ok {
			continue
		}
		asset.CollisionMeshes = append(asset.CollisionMeshes, &ir.CollisionMesh{
			Type:      collisionType,
			MeshIndex: node.MeshIndex,
			NodeIndex: i,
		})
	}
}

func collisionTypeFromName(name string) (ir.CollisionType, bool) {
	nameUpper := strings.ToUpper(name)
	switch {
	case strings.HasPrefix(nameUpper, "UBX_"):
		return ir.CollisionTypeBox, true
	case strings.HasPrefix(nameUpper, "USP_"):
		return ir.CollisionTypeSphere, true
	case strings.HasPrefix(nameUpper, "UCP_"):
		return ir.CollisionTypeCapsule, true
	case strings.HasPrefix(nameUpper, "UCX_"):
		return ir.CollisionTypeConvexHull, true
	default:
		return ir.CollisionTypeMesh, false
	}
}
