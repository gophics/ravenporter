package fbx

import (
	"math"
	"strconv"

	"github.com/gophics/ravenporter/ir"
)

const (
	nodeObjects        = "Objects"
	nodeConnections    = "Connections"
	nodeGlobalSettings = "GlobalSettings"
	nodeProperties70   = "Properties70"
	nodeP              = "P"

	objGeometry      = "Geometry"
	objModel         = "Model"
	objMaterial      = "Material"
	objTexture       = "Texture"
	objAnimStack     = "AnimationStack"
	objAnimLayer     = "AnimationLayer"
	objAnimCurveNode = "AnimationCurveNode"
	objAnimCurve     = "AnimationCurve"
	objVideo         = "Video"

	connOO = "OO"
	connOP = "OP"

	propUpAxis        = "UpAxis"
	propUnitScale     = "UnitScaleFactor"
	propLclTranslate  = "Lcl Translation"
	propLclRotation   = "Lcl Rotation"
	propLclScaling    = "Lcl Scaling"
	propPreRotation   = "PreRotation"
	propGeomTranslate = "GeometricTranslation"
	propGeomRotation  = "GeometricRotation"
	propGeomScaling   = "GeometricScaling"

	layerNormal    = "LayerElementNormal"
	layerUV        = "LayerElementUV"
	layerTangent   = "LayerElementTangent"
	layerBinormal  = "LayerElementBinormal"
	layerSmoothing = "LayerElementSmoothing"
	leSmoothData   = "Smoothing"

	refIndexToDirect = "IndexToDirect"
	mapByPolygonVtx  = "ByPolygonVertex"
	mapByVertex      = "ByVertex"

	geoVertices    = "Vertices"
	geoPolyIndices = "PolygonVertexIndex"

	texRelFilename = "RelativeFilename"
	texFileName    = "FileName"

	objDeformer          = "Deformer"
	deformerCluster      = "Cluster"
	geoSubtypeShape      = "Shape"
	clusterIndexes       = "Indexes"
	clusterWeights       = "Weights"
	clusterTransformLink = "TransformLink"

	leNormals   = "Normals"
	leTangents  = "Tangents"
	leBinormals = "Binormals"
	leUV        = "UV"
	leNormalIdx = "NormalIndex"
	leUVIdx     = "UVIndex"
	leRefInfo   = "ReferenceInformationType"
	leMapInfo   = "MappingInformationType"

	animKeyTime  = "KeyTime"
	animKeyValue = "KeyValueFloat"

	animTargetT = "T"
	animTargetR = "R"
	animTargetS = "S"

	propMinIndex = 4
	vecStride    = 3
	degToRad     = math.Pi / 180

	fbxPropDiffuseColor    = "DiffuseColor"
	fbxPropEmissiveColor   = "EmissiveColor"
	fbxPropSpecularColor   = "SpecularColor"
	fbxPropAmbientColor    = "AmbientColor"
	fbxPropNormalMap       = "NormalMap"
	fbxPropSpecular        = "specular"
	fbxPropAmbient         = "ambient"
	fbxPropSpecularTexture = "specularTexture"
	fbxPropAmbientTexture  = "ambientTexture"

	fbxKTimeScale = 46186158000.0 // FBX KTime ticks per second

	defaultSkinName = "FBX_Skin"
	deformerSkin    = "Skin"
	defaultAnimName = "default"
	animLongT       = "Translation"
	animLongR       = "Rotation"
	animLongS       = "Scaling"

	objCamera          = "Camera"
	objLight           = "Light"
	layerColor         = "LayerElementColor"
	leColors           = "Colors"
	leColorIdx         = "ColorIndex"
	propFOV            = "FieldOfView"
	propFOVX           = "FieldOfViewX"
	propNearPlane      = "NearPlane"
	propFarPlane       = "FarPlane"
	propLightColor     = "Color"
	propIntensity      = "Intensity"
	propLightType      = "LightType"
	propInnerAngle     = "InnerAngle"
	propOuterAngle     = "OuterAngle"
	colorVecStride     = 4
	defaultCamFOV      = 39.6
	defaultNear        = 0.1
	defaultFar         = 1000.0
	defaultRoughness   = 1.0
	defaultNormalScale = 1.0
	defaultIntensity   = 1.0
	defaultUnit        = 1.0
	fbxIntensityScale  = 100.0
)

var (
	defaultBaseColor  = [4]float32{0.8, 0.8, 0.8, 1.0} //nolint:gochecknoglobals // shared default
	defaultLightColor = [3]float32{1, 1, 1}            //nolint:gochecknoglobals // shared default
	defaultTiling     = [2]float32{1, 1}               //nolint:gochecknoglobals // shared default
)

type connection struct {
	childID  int64
	parentID int64
	propName string
}

type fbxAnimCurveNode struct {
	id     int64
	target string
}

type fbxAnimCurve struct {
	id     int64
	times  []float32
	values []float32
	interp ir.Interpolation
}

type fbxCluster struct {
	id      int64
	idxs    []int32
	weights []float64
	ibm     [16]float32
}

type fbxShape struct {
	id        int64
	name      string
	positions [][3]float32
}

func convertFBX(nodes []fbxNode, version uint32) *ir.Asset {
	asset := ir.NewAsset(ir.FormatFBX)
	asset.UpAxis = ir.YUp
	asset.Unit = 1.0
	asset.Metadata.SourceVersion = strconv.FormatUint(uint64(version), 10)

	readGlobalSettings(nodes, asset)

	objects := findNode(nodes, nodeObjects)
	conns := findNode(nodes, nodeConnections)

	if objects == nil {
		return asset
	}

	maps, curveNodes, curves := parseObjects(objects, asset)

	connections := parseConnections(conns)
	resolveConnections(
		asset, connections, maps.geo, maps.mat, maps.model,
		maps.tex, maps.cam, maps.light, maps.video, maps.lodGroups,
	)
	resolveAnimations(asset, connections, curveNodes, curves, maps.model, maps.animStacks, maps.animLayers)
	resolveSkins(asset, connections, maps.clusters, maps.model)
	resolveMorphTargets(asset, connections, maps.shapes, maps.geo)
	appendCollisionMeshes(asset)

	return asset
}

type objectMaps struct {
	geo, mat, model, tex map[int64]int
	clusters             []fbxCluster
	shapes               []fbxShape
	cam                  map[int64]int
	light                map[int64]int
	video                map[int64][]byte
	animStacks           map[int64]int
	animLayers           map[int64]struct{}
	lodGroups            map[int64]int
}

//nolint:funlen // dispatch switch
func parseObjects(objects *fbxNode, asset *ir.Asset) (objectMaps, []fbxAnimCurveNode, []fbxAnimCurve) {
	nObj := len(objects.children)
	m := objectMaps{
		geo:        make(map[int64]int, nObj/4), //nolint:mnd // capacity hint
		mat:        make(map[int64]int, nObj/8), //nolint:mnd // capacity hint
		model:      make(map[int64]int, nObj/4), //nolint:mnd // capacity hint
		tex:        make(map[int64]int, nObj/8), //nolint:mnd // capacity hint
		cam:        make(map[int64]int),
		light:      make(map[int64]int),
		video:      make(map[int64][]byte),
		animStacks: make(map[int64]int),
		animLayers: make(map[int64]struct{}),
		lodGroups:  make(map[int64]int),
	}
	var curveNodes []fbxAnimCurveNode
	var curves []fbxAnimCurve

	for _, child := range objects.children {
		if len(child.properties) < 2 { //nolint:mnd // FBX objects have ID + name
			continue
		}
		id := child.properties[0].intVal

		switch child.name {
		case objGeometry:
			subType := extractSubType(&child)
			if subType == geoSubtypeShape {
				sh := extractShapePositions(&child, id)
				m.shapes = append(m.shapes, sh)
				continue
			}
			mesh := convertGeometry(&child)
			if mesh != nil {
				m.geo[id] = len(asset.Meshes)
				asset.Meshes = append(asset.Meshes, mesh)
			}
		case objModel:
			node := convertModel(&child)
			m.model[id] = len(asset.Nodes)
			asset.Nodes = append(asset.Nodes, node)

			if extractSubType(&child) == "LodGroup" {
				m.lodGroups[id] = len(asset.LODGroups)
				asset.LODGroups = append(asset.LODGroups, &ir.LODGroup{Name: node.Name})
			}
		case objMaterial:
			mat := convertMaterial(&child)
			m.mat[id] = len(asset.Materials)
			asset.Materials = append(asset.Materials, mat)
		case objTexture:
			img, tex := convertTexture(&child)
			if tex != nil {
				tex.ImageIndex = len(asset.Images)
				asset.Images = append(asset.Images, img)
				m.tex[id] = len(asset.Textures)
				asset.Textures = append(asset.Textures, tex)
			}
		case objAnimStack:
			anim := &ir.Animation{Name: extractName(&child)}
			m.animStacks[id] = len(asset.Animations)
			asset.Animations = append(asset.Animations, anim)
		case objAnimLayer:
			m.animLayers[id] = struct{}{}
		case objAnimCurveNode:
			cn := fbxAnimCurveNode{id: id, target: extractAnimTarget(&child)}
			curveNodes = append(curveNodes, cn)
		case objAnimCurve:
			ac := extractAnimCurve(&child, id)
			if ac != nil {
				curves = append(curves, *ac)
			}
		case objDeformer:
			subType := extractSubType(&child)
			if subType == deformerCluster {
				cl := extractCluster(&child, id)
				m.clusters = append(m.clusters, cl)
			}
		case objCamera:
			cam := convertFBXCamera(&child)
			m.cam[id] = len(asset.Cameras)
			asset.Cameras = append(asset.Cameras, cam)
		case objLight:
			light := convertFBXLight(&child)
			m.light[id] = len(asset.Lights)
			asset.Lights = append(asset.Lights, light)
		case objVideo:
			if data := extractVideoContent(&child); len(data) > 0 {
				m.video[id] = data
			}
		}
	}

	return m, curveNodes, curves
}

func readGlobalSettings(nodes []fbxNode, asset *ir.Asset) {
	gs := findNode(nodes, nodeGlobalSettings)
	if gs == nil {
		return
	}
	p70 := findNode(gs.children, nodeProperties70)
	if p70 == nil {
		return
	}
	for _, p := range p70.children {
		if p.name != nodeP || len(p.properties) == 0 {
			continue
		}
		name := p.properties[0].strVal
		switch name {
		case propUpAxis:
			if len(p.properties) > propMinIndex && p.properties[propMinIndex].intVal == 2 {
				asset.UpAxis = ir.ZUp
			}
		case propUnitScale:
			if len(p.properties) > propMinIndex {
				asset.Unit = p.properties[propMinIndex].floatVal
			}
		}
	}
}
