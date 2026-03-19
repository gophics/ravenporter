package fbx

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	asciiNodeObjects       = "Objects"
	asciiNodeGeometry      = "Geometry"
	asciiNodeModel         = "Model"
	asciiNodeTexture       = "Texture"
	asciiNodeAnimStack     = "AnimationStack"
	asciiNodeAnimCurveNode = "AnimationCurveNode"
	asciiNodeAnimCurve     = "AnimationCurve"
	asciiNodeDeformer      = "Deformer"
	asciiNodeAttribute     = "NodeAttribute"
	asciiPropVertex        = "Vertices"
	asciiPropIndex         = "PolygonVertexIndex"
	asciiPropNormals       = "Normals"
	asciiPropUV            = "UV"
	asciiMaterial          = "Material"
	asciiArrayPrefix       = "a:"
	nameSepLen             = 2 // len("::")
	defaultMeshName        = "FBX_Mesh"
	uvsPerVertex           = 2
	asciiRelFilename       = "RelativeFilename:"
	asciiFileName          = "FileName:"
	connPartsOP            = 4 // C: "OP",child,parent,"prop"
)

var (
	bOpen                 = []byte{'{'}
	bClose                = []byte{'}'}
	bComma                = []byte{','}
	bNameSep              = []byte("::")
	bArrayPrefix          = []byte(asciiArrayPrefix)
	bRelFilename          = []byte(asciiRelFilename)
	bFileName             = []byte(asciiFileName)
	bPropVertex           = []byte(asciiPropVertex + ":")
	bPropIndex            = []byte(asciiPropIndex + ":")
	bPropNormals          = []byte(asciiPropNormals + ":")
	bPropUV               = []byte(asciiPropUV + ":")
	bUVIndex              = []byte("UVIndex")
	bGeometry             = []byte(asciiNodeGeometry + ":")
	bModel                = []byte(asciiNodeModel + ":")
	bTexture              = []byte(asciiNodeTexture + ":")
	bDeformer             = []byte(asciiNodeDeformer + ":")
	bAnimCurveNode        = []byte(asciiNodeAnimCurveNode + ":")
	bAnimCurve            = []byte(asciiNodeAnimCurve + ":")
	bAnimStack            = []byte(asciiNodeAnimStack + ":")
	bNodeAttribute        = []byte(asciiNodeAttribute + ":")
	bMaterial             = []byte(asciiMaterial + ":")
	bCluster              = []byte("\"Cluster\"")
	bShape                = []byte("\"Shape\"")
	bCamera               = []byte("\"" + attrCamera + "\"")
	bLight                = []byte("\"" + attrLight + "\"")
	bConnections          = []byte("Connections:")
	bConnPrefix           = []byte("C:")
	bClusterIndexes       = []byte(clusterIndexes + ":")
	bClusterWeights       = []byte(clusterWeights + ":")
	bClusterTransformLink = []byte(clusterTransformLink + ":")
	bTransform            = []byte("Transform:")
	bKeyTime              = []byte(animKeyTime + ":")
	bKeyValue             = []byte(animKeyValue + ":")
	bVertices             = []byte(geoVertices + ":")
	bQuote                = []byte("\"")
	bPropLine             = []byte("P:")
	bPropFOV              = []byte("\"" + propFOV + "\"")
	bPropFOVX             = []byte("\"" + propFOVX + "\"")
	bPropNear             = []byte("\"" + propNearPlane + "\"")
	bPropFar              = []byte("\"" + propFarPlane + "\"")
	bPropLightColor       = []byte("\"" + propLightColor + "\"")
	bPropIntensity        = []byte("\"" + propIntensity + "\"")
	bPropLightType        = []byte("\"" + propLightType + "\"")
	bPropInnerAngle       = []byte("\"" + propInnerAngle + "\"")
	bPropOuterAngle       = []byte("\"" + propOuterAngle + "\"")
)

type asciiConnection struct {
	child    int64
	parent   int64
	propName string
}

type asciiTexture struct {
	id   int64
	name string
	path string
}

type asciiCluster struct {
	id      int64
	idxs    []int32
	weights []float64
	ibm     [16]float32
}

type asciiAnimCurve struct {
	id       int64
	keyTimes []float64
	keyVals  []float64
}

type asciiModelInfo struct {
	id   int64
	name string
}

type asciiMeshData struct {
	name        string
	positions   [][3]float32
	normals     [][3]float32
	uvs         [][2]float32
	indices     []uint32
	polyIndices []int32 // raw FBX polygon indices with negative end-of-face markers
}

type asciiShape struct {
	id     int64
	name   string
	deltas [][3]float32
}

type asciiParseResult struct {
	meshes           []*ir.Mesh
	models           []asciiModelInfo
	matNames         []string
	matIDs           []int64
	geoIDs           []int64
	animStacks       []string
	curveNodeIDs     []int64
	curveNodeTargets []string
	curveIDs         []int64
	curves           []asciiAnimCurve
	clusters         []asciiCluster
	deformerIDs      []int64
	deformerTypes    []string
	shapes           []asciiShape
	nodeAttrIDs      []int64
	nodeAttrTypes    []string
	cameras          []*ir.Camera
	lights           []*ir.Light
	textures         []asciiTexture
}

func decodeASCIIFBX(sysCtx context.Context, data []byte) (*ir.Asset, error) {
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	s := decutil.LineScanner{Data: data}

	conns, err := parseASCIIConnections(sysCtx, &s)
	if err != nil {
		return nil, err
	}

	s.Pos = 0
	result, err := parseASCIIMeshes(sysCtx, &s)
	if err != nil {
		return nil, err
	}

	asset := ir.NewAsset(ir.FormatFBX)
	asset.UpAxis = ir.YUp
	asset.Unit = defaultUnit
	asset.Metadata.SourceVersion = extractASCIIVersion(data)

	asset.Meshes = result.meshes
	asset.Nodes = buildASCIIHierarchy(result.meshes, result.geoIDs, result.models, conns)

	asset.Cameras = result.cameras
	asset.Lights = result.lights
	wireASCIICamerasLights(asset, result, conns)

	childSet := make(map[int]bool, len(asset.Nodes))
	for i := range asset.Nodes {
		for _, c := range asset.Nodes[i].Children {
			childSet[c] = true
		}
	}
	for i := range asset.Nodes {
		if !childSet[i] {
			asset.RootNodes = append(asset.RootNodes, i)
		}
	}

	matIDMap := make(map[int64]int, len(result.matNames))
	for i, name := range result.matNames {
		if i < len(result.matIDs) {
			matIDMap[result.matIDs[i]] = len(asset.Materials)
		}
		asset.Materials = append(asset.Materials, &ir.Material{
			Name:            name,
			BaseColorFactor: defaultBaseColor,
			AlphaMode:       ir.AlphaOpaque,
			RoughnessFactor: defaultRoughness,
		})
	}

	texIDMap := make(map[int64]int, len(result.textures))
	for _, tex := range result.textures {
		imageIndex := len(asset.Images)
		asset.Images = append(asset.Images, &ir.ImageAsset{
			Name:       tex.name,
			SourcePath: tex.path,
		})
		texIDMap[tex.id] = len(asset.Textures)
		asset.Textures = append(asset.Textures, &ir.Texture{
			Name:       tex.name,
			ImageIndex: imageIndex,
		})
	}
	wireASCIITextures(asset, conns, matIDMap, texIDMap)
	assignASCIIMaterials(asset, conns, result.models, matIDMap)

	if len(result.animStacks) > 0 {
		for _, name := range result.animStacks {
			asset.Animations = append(asset.Animations, &ir.Animation{Name: name})
		}
	}
	resolveASCIIAnimations(asset, result, conns, result.models)

	resolveASCIIMorphTargets(asset, result)

	resolveASCIISkins(asset, result, conns)

	if len(asset.Meshes) == 0 {
		return nil, decutil.DecodeErr(ir.FormatFBX, "ascii", errors.New("no geometry found"))
	}

	return asset, nil
}

func parseASCIIMeshes(sysCtx context.Context, s *decutil.LineScanner) (asciiParseResult, error) { //nolint:funlen // dispatch loop
	var res asciiParseResult
	var geometries []asciiMeshData
	var depth int

	for line := s.Next(); line != nil; line = s.Next() {
		if err := sysCtx.Err(); err != nil {
			return res, err
		}

		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)
		depth += openCount - closeCount

		if bytes.HasPrefix(line, bGeometry) && openCount > 0 {
			geoID := extractASCIIIDB(line)
			name := extractASCIIName(line)

			if bytes.Contains(line, bShape) {
				shape := parseASCIIShape(s, &depth, geoID, name)
				res.shapes = append(res.shapes, shape)
				continue
			}

			geo := parseASCIIGeometry(s, &depth)
			geo.name = name
			if len(geo.positions) > 0 {
				geometries = append(geometries, geo)
				res.geoIDs = append(res.geoIDs, geoID)
			}
			continue
		}

		if bytes.HasPrefix(line, bDeformer) && bytes.Contains(line, bCluster) && openCount > 0 {
			clID := extractASCIIIDB(line)
			cl := parseASCIIDeformer(s, &depth, clID)
			res.clusters = append(res.clusters, cl)
			res.deformerIDs = append(res.deformerIDs, clID)
			res.deformerTypes = append(res.deformerTypes, deformerCluster)
			continue
		}

		if bytes.HasPrefix(line, bTexture) && openCount > 0 {
			texID := extractASCIIIDB(line)
			name := extractASCIIName(line)
			tex := parseASCIITexture(s, &depth, texID, name)
			res.textures = append(res.textures, tex)
			continue
		}

		if bytes.HasPrefix(line, bAnimCurve) && !bytes.Contains(line, bAnimCurveNode) && openCount > 0 {
			curveID := extractASCIIIDB(line)
			curve := parseASCIIAnimCurve(s, &depth, curveID)
			res.curves = append(res.curves, curve)
			continue
		}

		if bytes.Contains(line, bNodeAttribute) && openCount > 0 {
			attrID := extractASCIIIDB(line)
			name := extractASCIIName(line)
			if bytes.Contains(line, bCamera) {
				cam := parseASCIICamera(s, &depth, name)
				res.nodeAttrIDs = append(res.nodeAttrIDs, attrID)
				res.nodeAttrTypes = append(res.nodeAttrTypes, attrCamera)
				res.cameras = append(res.cameras, cam)
			} else if bytes.Contains(line, bLight) {
				light := parseASCIILight(s, &depth, name)
				res.nodeAttrIDs = append(res.nodeAttrIDs, attrID)
				res.nodeAttrTypes = append(res.nodeAttrTypes, attrLight)
				res.lights = append(res.lights, light)
			}
			continue
		}

		classifyASCIIObjectLine(line, openCount, &res)
	}

	for i := range geometries {
		res.meshes = append(res.meshes, buildASCIIMesh(geometries[i]))
	}
	return res, nil
}

func classifyASCIIObjectLine(line []byte, openCount int, res *asciiParseResult) {
	if openCount == 0 {
		return
	}

	if bytes.Contains(line, bModel) {
		modelID := extractASCIIIDB(line)
		name := extractASCIIName(line)
		if name != defaultMeshName {
			res.models = append(res.models, asciiModelInfo{id: modelID, name: name})
		} else {
			res.models = append(res.models, asciiModelInfo{id: modelID, name: asciiNodeModel})
		}
	}

	if bytes.Contains(line, bMaterial) && bytes.Contains(line, bQuote) {
		mat := extractASCIIName(line)
		matID := extractASCIIIDB(line)
		if mat != defaultMeshName {
			res.matNames = appendUnique(res.matNames, mat)
			res.matIDs = append(res.matIDs, matID)
		}
	}

	if bytes.Contains(line, bAnimStack) {
		name := extractASCIIName(line)
		if name != defaultMeshName {
			res.animStacks = append(res.animStacks, name)
		}
	}

	if bytes.Contains(line, bAnimCurveNode) {
		cnID := extractASCIIIDB(line)
		cnTarget := extractASCIIName(line)
		res.curveNodeIDs = append(res.curveNodeIDs, cnID)
		res.curveNodeTargets = append(res.curveNodeTargets, cnTarget)
	}

	isAnimCurve := bytes.Contains(line, bAnimCurve)
	if isAnimCurve && !bytes.Contains(line, bAnimCurveNode) {
		curveID := extractASCIIIDB(line)
		res.curveIDs = append(res.curveIDs, curveID)
	}

	if bytes.Contains(line, bDeformer) {
		defID := extractASCIIIDB(line)
		subType := deformerSkin
		if bytes.Contains(line, bCluster) {
			subType = deformerCluster
		}
		res.deformerIDs = append(res.deformerIDs, defID)
		res.deformerTypes = append(res.deformerTypes, subType)
	}
}

func parseASCIIConnections(sysCtx context.Context, s *decutil.LineScanner) ([]asciiConnection, error) {
	var conns []asciiConnection
	var inConnections bool

	for line := s.Next(); line != nil; line = s.Next() {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		if bytes.HasPrefix(line, bConnections) {
			inConnections = true
			continue
		}
		if !inConnections {
			continue
		}
		if len(line) == 1 && line[0] == '}' {
			break
		}
		if !bytes.HasPrefix(line, bConnPrefix) {
			continue
		}
		c1 := bytes.IndexByte(line, ',')
		if c1 < 0 {
			continue
		}
		c2 := bytes.IndexByte(line[c1+1:], ',')
		if c2 < 0 {
			continue
		}
		c2 += c1 + 1
		child, err1 := strconv.ParseInt(decutil.Bstr(bytes.TrimSpace(line[c1+1:c2])), 10, 64)
		var parentEnd int
		c3 := bytes.IndexByte(line[c2+1:], ',')
		if c3 >= 0 {
			parentEnd = c2 + 1 + c3
		} else {
			parentEnd = len(line)
		}
		parent, err2 := strconv.ParseInt(decutil.Bstr(bytes.TrimSpace(line[c2+1:parentEnd])), 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		c := asciiConnection{child: child, parent: parent}
		if c3 >= 0 {
			c.propName = string(bytes.Trim(bytes.TrimSpace(line[parentEnd+1:]), "\""))
		}
		conns = append(conns, c)
	}
	return conns, nil
}

func extractASCIIIDB(line []byte) int64 {
	colonIdx := bytes.IndexByte(line, ':')
	if colonIdx < 0 {
		return 0
	}
	rest := line[colonIdx+1:]
	commaIdx := bytes.IndexByte(rest, ',')
	if commaIdx >= 0 {
		rest = rest[:commaIdx]
	}
	id, _ := strconv.ParseInt(decutil.Bstr(bytes.TrimSpace(rest)), 10, 64) //nolint:errcheck // best-effort
	return id
}
