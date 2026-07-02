package usda

import (
	"archive/zip"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

type crateDecodeContext struct {
	archive     []*zip.File
	maxFileSize int64
	reporter    detect.DecodeReporter
}

//nolint:funlen,gocyclo // crate conversion handles all spec types inline
func decodeCrateToSceneWithContext(data []byte, ctx crateDecodeContext) (*ir.Asset, error) {
	cr, err := parseCrate(data)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatUSD, "crate parse failed", err)
	}

	asset := ir.NewAsset(ir.FormatUSD)
	asset.UpAxis = ir.YUp
	asset.Unit = 1.0
	asset.Metadata.SourceVersion = crateVersionStr(cr.version)

	pathToNode := make(map[int32]int)
	matPathToIdx := make(map[int32]int)
	meshPathToIdx := make(map[int32]int)
	var shaders []deferredShader
	var gsubsets []deferredGeomSubset
	var bshapes []deferredBlendShape
	var assetInherits []inheritArc

	for _, spec := range cr.specs {
		if !isCrateRootSpec(cr, spec) {
			continue
		}
		fields := cr.specFields(spec)
		if axis := crateFieldStr(cr, fields, tokUpAxis); axis == "Z" {
			asset.UpAxis = ir.ZUp
		}
		if f, ok := cr.findFieldValue(fields, tokUnit); ok {
			if v := cr.readFloat64(f.valueRep); v > 0 {
				asset.Unit = v
			}
		}
		if name := crateFieldStr(cr, fields, tokDefPrim); name != "" {
			asset.Name = name
		}
		if f, ok := cr.findFieldValue(fields, tokVariantSets); ok {
			tokens := cr.readTokenArray(f.valueRep)
			if len(tokens) > 0 {
				if asset.Metadata.ExtraProperties == nil {
					asset.Metadata.ExtraProperties = make(map[string]string)
				}
				asset.Metadata.ExtraProperties["variantSets"] = strings.Join(tokens, ",")
			}
		}
		if err := resolveCrateExternalRefs(cr, ctx, asset, fields, tokSubLayers, "sublayer"); err != nil {
			return nil, err
		}
	}

	for _, spec := range cr.specs {
		if spec.specType != specTypePrim {
			continue
		}
		fields := cr.specFields(spec)
		typeName := crateFieldStr(cr, fields, tokType)
		name := cr.pathName(spec.pathIdx)

		nodeIdxBefore := len(asset.Nodes)

		switch typeName {
		case tokMesh:
			meshPathToIdx[spec.pathIdx] = len(asset.Meshes)
			convertCrateMesh(cr, fields, name, asset)
		case tokCamera:
			convertCrateCamera(cr, fields, name, asset)
		case tokXform:
			convertCrateXform(cr, fields, name, asset)
		case tokDistLight:
			convertCrateLight(cr, fields, name, asset, tokDistLight)
		case tokSphLight:
			convertCrateLight(cr, fields, name, asset, tokSphLight)
		case tokDiskLight:
			convertCrateLight(cr, fields, name, asset, tokDiskLight)
		case tokRectLight:
			convertCrateLight(cr, fields, name, asset, tokRectLight)
		case tokCylLight:
			convertCrateLight(cr, fields, name, asset, tokCylLight)
		case tokScope:
			convertCrateScope(name, asset)
		case tokSkeleton:
			convertCrateSkeleton(cr, fields, name, asset)
		case tokMaterial:
			matPathToIdx[spec.pathIdx] = len(asset.Materials)
			convertCrateMaterial(cr, fields, name, asset)
		case tokShader:
			shaders = append(shaders, deferredShader{pathIdx: spec.pathIdx, fields: fields})
		case tokBasisCurves:
			convertCrateBasisCurves(cr, fields, name, asset)
		case tokPointsPrim:
			convertCratePoints(cr, fields, name, asset)
		case tokNurbsCurves:
			convertCrateBasisCurves(cr, fields, name, asset)
		case tokCube, tokSphere, tokCylinder, tokCone, tokCapsule:
			convertCrateProceduralPrim(cr, fields, name, typeName, asset)
		case tokGeomSubset:
			gsubsets = append(gsubsets, deferredGeomSubset{pathIdx: spec.pathIdx, fields: fields})
		case tokSkelAnim:
			convertCrateSkelAnim(cr, fields, name, asset)
		case tokBlendShape:
			bshapes = append(bshapes, deferredBlendShape{pathIdx: spec.pathIdx, fields: fields})
		}

		if len(asset.Nodes) > nodeIdxBefore {
			pathToNode[spec.pathIdx] = nodeIdxBefore
		}
		if err := resolveCrateExternalRefs(cr, ctx, asset, fields, tokReferences, "reference"); err != nil {
			return nil, err
		}
		if err := resolveCrateExternalRefs(cr, ctx, asset, fields, tokPayload, "payload"); err != nil {
			return nil, err
		}
		if nodeIdx, ok := pathToNode[spec.pathIdx]; ok {
			recordCrateInheritArcs(cr, fields, nodeIdx, &assetInherits)
		}
	}

	wireDeferredShaders(cr, asset, matPathToIdx, shaders)
	wireDeferredGeomSubsets(cr, asset, meshPathToIdx, gsubsets)
	wireDeferredBlendShapes(cr, asset, meshPathToIdx, bshapes)
	resolveInheritedNodes(asset, assetInherits)

	asset.RootNodes = asset.RootNodes[:0]

	// Deterministic parent wiring order.
	var sortedPaths []int
	for p := range pathToNode {
		sortedPaths = append(sortedPaths, int(p))
	}
	slices.Sort(sortedPaths)

	for _, p := range sortedPaths {
		if p < math.MinInt32 || p > math.MaxInt32 {
			continue
		}
		pathIdx := int32(p)
		nodeIdx := pathToNode[pathIdx]
		if pathIdx < 0 || int(pathIdx) >= len(cr.paths) {
			asset.RootNodes = append(asset.RootNodes, nodeIdx)
			continue
		}
		parentPath := cr.paths[pathIdx].parentIdx
		if parentNodeIdx, ok := pathToNode[parentPath]; ok {
			asset.Nodes[parentNodeIdx].Children = append(asset.Nodes[parentNodeIdx].Children, nodeIdx)
		} else {
			asset.RootNodes = append(asset.RootNodes, nodeIdx)
		}
	}

	return asset, nil
}

func isCrateRootSpec(cr *crateReader, spec crateSpec) bool {
	if spec.specType == specTypePseudoRoot {
		return true
	}
	if spec.pathIdx < 0 || int(spec.pathIdx) >= len(cr.paths) {
		return false
	}
	return cr.parentPathIdx(spec.pathIdx) < 0 && cr.pathName(spec.pathIdx) == ""
}

func resolveCrateExternalRefs(
	cr *crateReader,
	ctx crateDecodeContext,
	asset *ir.Asset,
	fields []crateField,
	fieldName, relation string,
) error {
	f, ok := cr.findFieldValue(fields, fieldName)
	if !ok {
		return nil
	}
	for _, refPath := range crateFieldStrings(cr, f) {
		refPath = extractAssetPath(refPath)
		if refPath == "" {
			continue
		}
		if ctx.reporter != nil {
			ctx.reporter.AddDependency("scene", refPath, relation, usdaReportedBy)
		}
		if ctx.archive == nil {
			continue
		}
		refScene, err := loadCrateRefScene(ctx, refPath)
		if err != nil {
			return err
		}
		if refScene != nil {
			mergeRefScene(asset, refScene)
		}
	}
	return nil
}

func recordCrateInheritArcs(cr *crateReader, fields []crateField, nodeIdx int, inherits *[]inheritArc) {
	for _, fieldName := range []string{tokInherits, tokSpecializes} {
		f, ok := cr.findFieldValue(fields, fieldName)
		if !ok {
			continue
		}
		for _, basePath := range crateFieldStrings(cr, f) {
			basePath = strings.Trim(basePath, "<>")
			basePath = strings.TrimPrefix(basePath, "/")
			if basePath == "" {
				continue
			}
			*inherits = append(*inherits, inheritArc{nodeIdx: nodeIdx, basePath: basePath})
		}
	}
}

func crateFieldStrings(cr *crateReader, field crateField) []string {
	if values := cr.readStringArray(field.valueRep); len(values) > 0 {
		return compactStrings(values)
	}
	if values := cr.readTokenArray(field.valueRep); len(values) > 0 {
		return compactStrings(values)
	}
	if value := cr.readInlineString(field.valueRep); value != "" {
		return []string{value}
	}
	if value := cr.readInlineToken(field.valueRep); value != "" {
		return []string{value}
	}
	return nil
}

func compactStrings(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func loadCrateRefScene(ctx crateDecodeContext, refPath string) (*ir.Asset, error) {
	for _, file := range ctx.archive {
		if !strings.HasSuffix(file.Name, "/"+refPath) && file.Name != refPath {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, err := decutil.ReadAllLimit(rc, ctx.maxFileSize)
		rc.Close() //nolint:errcheck,gosec
		if err != nil {
			return nil, err
		}
		if hasUSDCMagic(data) {
			return decodeCrateToSceneWithContext(data, ctx)
		}
		parser := &usdaParser{
			ls:          decutil.LineScanner{Data: data},
			asset:       ir.NewAsset(ir.FormatUSD),
			archive:     ctx.archive,
			maxFileSize: ctx.maxFileSize,
			reporter:    ctx.reporter,
		}
		parser.asset.UpAxis = ir.YUp
		parser.asset.Unit = 1.0
		parser.asset.Metadata.SourceVersion = usdaSourceVersion
		parser.parse()
		if parser.err != nil {
			return nil, parser.err
		}
		return parser.asset, nil
	}
	return nil, nil
}

//nolint:funlen,gocyclo // mesh reads all attributes inline
func convertCrateMesh(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	var prim ir.Primitive
	prim.Mode = ir.Triangles
	prim.MaterialIndex = ir.NoIndex
	var morphTargets []ir.MorphTarget

	if f, ok := cr.findFieldValue(fields, tokPoints); ok {
		prim.Data.Positions = cr.readVec3fArray(f.valueRep)
	} else if f, ok := cr.findFieldValue(fields, tokTimeSamples); ok {
		times, frames := cr.readTimeSampledVec3(f.valueRep)
		if len(frames) > 0 {
			prim.Data.Positions = frames[0]
			for fi := 1; fi < len(frames); fi++ {
				mt := ir.MorphTarget{Name: "frame_" + strconv.Itoa(int(times[fi]))}
				mt.Positions = make([][3]float32, len(frames[fi]))
				for vi := range frames[fi] {
					if vi >= len(frames[0]) {
						continue
					}
					mt.Positions[vi] = [3]float32{
						frames[fi][vi][0] - frames[0][vi][0],
						frames[fi][vi][1] - frames[0][vi][1],
						frames[fi][vi][2] - frames[0][vi][2],
					}
				}
				morphTargets = append(morphTargets, mt)
			}
		}
	}
	if f, ok := cr.findFieldValue(fields, tokFaceIdx); ok {
		prim.Data.Indices = cr.readUint32Array(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokNormals); ok {
		prim.Data.Normals = cr.readVec3fArray(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokST); ok {
		prim.Data.TexCoord0 = cr.readVec2fArray(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokFaceCounts); ok {
		counts := cr.readIntArray(f.valueRep)
		if counts != nil && len(prim.Data.Indices) > 0 {
			prim.Data.Indices = triangulateCrateIndices(prim.Data.Indices, counts)
		}
	}
	if f, ok := cr.findFieldValue(fields, tokDisplayColor); ok {
		v3 := cr.readVec3fArray(f.valueRep)
		if len(v3) > 0 {
			colors := make([][4]float32, len(v3))
			for i, c := range v3 {
				colors[i] = [4]float32{c[0], c[1], c[2], 1}
			}
			prim.Data.Colors0 = colors
		}
	}
	if f, ok := cr.findFieldValue(fields, tokDisplayOpacity); ok {
		opacities := cr.readFloatArray(f.valueRep)
		if prim.Data.Colors0 == nil {
			prim.Data.Colors0 = make([][4]float32, len(opacities))
			for i := range prim.Data.Colors0 {
				prim.Data.Colors0[i] = [4]float32{1, 1, 1, opacities[i]}
			}
		} else {
			for i := range min(len(opacities), len(prim.Data.Colors0)) {
				prim.Data.Colors0[i][3] = opacities[i]
			}
		}
	}

	if len(prim.Data.Positions) == 0 {
		return
	}

	leftHanded := false
	if orient := crateFieldStr(cr, fields, tokOrientation); orient == usdaOrientLeft {
		leftHanded = true
	}
	doubleSided := false
	if f, ok := cr.findFieldValue(fields, tokDoubleSided); ok {
		doubleSided = cr.readInlineBool(f.valueRep)
	}

	if f, ok := cr.findFieldValue(fields, tokJointIndices); ok {
		prim.Data.Joints0 = cr.readJointIndices(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokJointWeights); ok {
		prim.Data.Weights0 = cr.readJointWeights(f.valueRep)
	}

	matName := crateFieldStr(cr, fields, tokMatBinding)
	if matName != "" {
		if idx := strings.LastIndex(matName, "/"); idx >= 0 {
			matName = matName[idx+1:]
		}
		for i, m := range asset.Materials {
			if m.Name == matName {
				prim.MaterialIndex = i
				break
			}
		}
	}
	if doubleSided && prim.MaterialIndex != ir.NoIndex {
		asset.Materials[prim.MaterialIndex].DoubleSided = true
	}

	if leftHanded && len(prim.Data.Indices) > 0 {
		flipWindingOrder(prim.Data.Indices)
	}

	prim.Data.VertexCount = len(prim.Data.Positions)
	prim.MorphTargets = morphTargets
	mesh := &ir.Mesh{
		Name:       name,
		Primitives: []ir.Primitive{prim},
	}
	asset.Meshes = append(asset.Meshes, mesh)

	skinIdx := ir.NoIndex
	skelName := crateFieldStr(cr, fields, tokSkelBinding)
	if skelName != "" {
		if si := strings.LastIndex(skelName, "/"); si >= 0 {
			skelName = skelName[si+1:]
		}
		for i, s := range asset.Skeletons {
			if s.Name == skelName {
				skinIdx = i
				break
			}
		}
	}
	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   len(asset.Meshes) - 1,
		SkinIndex:   skinIdx,
		CameraIndex: ir.NoIndex,
		LightIndex:  ir.NoIndex,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}

func convertCrateCamera(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	focalLen := float32(defaultFocalLen)
	hAperture := float32(defaultHAperture)
	vAperture := float32(defaultVAperture)
	var near, far float32

	if f, ok := cr.findFieldValue(fields, tokFocalLen); ok {
		focalLen = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokHAperture); ok {
		hAperture = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokVAperture); ok {
		vAperture = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokClipRange); ok {
		v := cr.readVec2fArray(f.valueRep)
		if len(v) > 0 {
			near, far = v[0][0], v[0][1]
		}
	}

	cam := &ir.Camera{Name: name}

	projection := crateFieldStr(cr, fields, tokProjection)
	if projection == usdaCamOrtho {
		cam.Orthographic = &ir.OrthographicCamera{
			XMag: hAperture,
			YMag: vAperture,
			Near: near,
			Far:  far,
		}
	} else {
		fov := float32(fovDivisor * math.Atan(float64(hAperture)/(fovDivisor*float64(focalLen))))
		aspect := hAperture / vAperture
		cam.Perspective = &ir.PerspectiveCamera{
			FOV:    fov,
			Aspect: aspect,
			Near:   near,
			Far:    far,
		}
	}
	camIdx := len(asset.Cameras)
	asset.Cameras = append(asset.Cameras, cam)

	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   ir.NoIndex,
		SkinIndex:   ir.NoIndex,
		CameraIndex: camIdx,
		LightIndex:  ir.NoIndex,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}

func convertCrateXform(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	transform := ir.IdentityTransform()

	if f, ok := cr.findFieldValue(fields, tokTranslate); ok {
		v := cr.readVec3dArray(f.valueRep)
		if v == nil {
			v = cr.readVec3fArray(f.valueRep)
		}
		if len(v) > 0 {
			transform.Translation = v[0]
		}
	}
	if f, ok := cr.findFieldValue(fields, tokRotateXYZ); ok {
		v := cr.readVec3fArray(f.valueRep)
		if v == nil {
			v = cr.readVec3dArray(f.valueRep)
		}
		if len(v) > 0 {
			xr := float64(v[0][0]) * mathx.DegToRad
			yr := float64(v[0][1]) * mathx.DegToRad
			zr := float64(v[0][2]) * mathx.DegToRad
			transform.Rotation = mathx.EulerToQuat(xr, yr, zr)
		}
	}
	if f, ok := cr.findFieldValue(fields, tokXformScale); ok {
		v := cr.readVec3dArray(f.valueRep)
		if v == nil {
			v = cr.readVec3fArray(f.valueRep)
		}
		if len(v) > 0 {
			transform.Scale = v[0]
		}
	}
	if f, ok := cr.findFieldValue(fields, tokTransform); ok {
		transform.Matrix = cr.readMatrix4d(f.valueRep)
	}

	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   ir.NoIndex,
		SkinIndex:   ir.NoIndex,
		CameraIndex: ir.NoIndex,
		LightIndex:  ir.NoIndex,
		Transform:   transform,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}

func convertCrateLight(cr *crateReader, fields []crateField, name string, asset *ir.Asset, lightType string) {
	light := &ir.Light{
		Name:      name,
		Color:     [3]float32{1, 1, 1},
		Intensity: defaultLightIntens,
	}

	if f, ok := cr.findFieldValue(fields, tokIntensity); ok {
		light.Intensity = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokColor); ok {
		v := cr.readVec3fArray(f.valueRep)
		if len(v) > 0 {
			light.Color = v[0]
		}
	}

	switch lightType {
	case tokDistLight:
		light.Directional = &ir.DirectionalLight{}
	case tokDiskLight:
		var coneAngle float32
		if f, ok := cr.findFieldValue(fields, tokConeAngle); ok {
			coneAngle = cr.readInlineFloat(f.valueRep)
		}
		light.Spot = &ir.SpotLight{
			OuterConeAngle: float32(float64(coneAngle) * mathx.DegToRad),
			InnerConeAngle: float32(float64(coneAngle) * mathx.DegToRad),
		}
	default:
		light.Point = &ir.PointLight{}
	}

	lightIdx := len(asset.Lights)
	asset.Lights = append(asset.Lights, light)

	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   ir.NoIndex,
		SkinIndex:   ir.NoIndex,
		CameraIndex: ir.NoIndex,
		LightIndex:  lightIdx,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}

func convertCrateScope(name string, asset *ir.Asset) {
	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   ir.NoIndex,
		SkinIndex:   ir.NoIndex,
		CameraIndex: ir.NoIndex,
		LightIndex:  ir.NoIndex,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}

func crateFieldStr(cr *crateReader, fields []crateField, name string) string {
	f, ok := cr.findFieldValue(fields, name)
	if !ok {
		return ""
	}
	return cr.readInlineToken(f.valueRep)
}

func crateVersionStr(v [3]uint8) string {
	buf := [5]byte{v[0] + '0', '.', v[1] + '0', '.', v[2] + '0'}
	return string(buf[:])
}

func triangulateCrateIndices(indices []uint32, faceCounts []int32) []uint32 {
	out := make([]uint32, 0, len(indices))
	offset := 0
	for _, fc := range faceCounts {
		c := int(fc)
		if offset+c > len(indices) {
			break
		}
		for j := 2; j < c; j++ {
			out = append(out, indices[offset], indices[offset+j-1], indices[offset+j])
		}
		offset += c
	}
	return out
}

func convertCrateSkeleton(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	skel := &ir.Skeleton{Name: name}

	if f, ok := cr.findFieldValue(fields, tokJoints); ok {
		tokens := cr.readTokenArray(f.valueRep)
		pathToNode := make(map[string]int, len(tokens))
		for _, j := range tokens {
			jname := j
			if idx := strings.LastIndex(j, "/"); idx >= 0 {
				jname = j[idx+1:]
			}
			jnode := ir.Node{LODGroupIndex: ir.NoIndex,
				Name:        jname,
				IsJoint:     true,
				MeshIndex:   ir.NoIndex,
				SkinIndex:   ir.NoIndex,
				CameraIndex: ir.NoIndex,
				LightIndex:  ir.NoIndex,
			}
			asset.Nodes = append(asset.Nodes, jnode)
			nodeIdx := len(asset.Nodes) - 1
			skel.Joints = append(skel.Joints, nodeIdx)
			pathToNode[j] = nodeIdx
		}
		for _, j := range tokens {
			if idx := strings.LastIndex(j, "/"); idx > 0 {
				parentPath := j[:idx]
				if parentIdx, ok := pathToNode[parentPath]; ok {
					childIdx := pathToNode[j]
					asset.Nodes[parentIdx].Children = append(asset.Nodes[parentIdx].Children, childIdx)
				}
			}
		}
	}
	if f, ok := cr.findFieldValue(fields, tokBindXforms); ok {
		skel.InverseBindMatrices = cr.readMatrix4dArray(f.valueRep)
	}

	asset.Skeletons = append(asset.Skeletons, skel)
}

func convertCrateSkelAnim(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	var joints []string
	if f, ok := cr.findFieldValue(fields, tokJoints); ok {
		joints = cr.readTokenArray(f.valueRep)
	}
	if len(joints) == 0 {
		return
	}
	var ad skelAnimData
	if f, ok := cr.findFieldValue(fields, tokAnimTranslations); ok {
		ad.transFrames = [][][3]float32{cr.readVec3fArray(f.valueRep)}
		ad.transTimes = []float32{0}
	}
	if f, ok := cr.findFieldValue(fields, tokAnimRotations); ok {
		ad.rotFrames = [][][4]float32{cr.readQuatfArray(f.valueRep)}
		ad.rotTimes = []float32{0}
	}
	if f, ok := cr.findFieldValue(fields, tokAnimScales); ok {
		ad.scaleFrames = [][][3]float32{cr.readVec3fArray(f.valueRep)}
		ad.scaleTimes = []float32{0}
	}
	anim := buildSkelAnim(name, joints, &ad, asset)
	if anim != nil {
		asset.Animations = append(asset.Animations, anim)
	}
}

func convertCrateMaterial(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	mat := &ir.Material{
		Name:            name,
		BaseColorFactor: [4]float32{1, 1, 1, 1},
		RoughnessFactor: 0.5,
	}

	if f, ok := cr.findFieldValue(fields, tokDiffuseColor); ok {
		v := cr.readVec3fArray(f.valueRep)
		if len(v) > 0 {
			mat.BaseColorFactor = [4]float32{v[0][0], v[0][1], v[0][2], 1}
		}
	}
	if f, ok := cr.findFieldValue(fields, tokMetallic); ok {
		mat.MetallicFactor = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokRoughness); ok {
		mat.RoughnessFactor = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokOpacity); ok {
		mat.BaseColorFactor[3] = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokEmissiveColor); ok {
		v := cr.readVec3fArray(f.valueRep)
		if len(v) > 0 {
			mat.EmissiveFactor = v[0]
		}
	}
	if f, ok := cr.findFieldValue(fields, tokOpacityThr); ok {
		mat.AlphaCutoff = cr.readInlineFloat(f.valueRep)
		if mat.AlphaCutoff > 0 {
			mat.AlphaMode = ir.AlphaMask
		}
	}
	if f, ok := cr.findFieldValue(fields, tokClearcoat); ok {
		if mat.Properties == nil {
			mat.Properties = make(map[string]any)
		}
		mat.Properties[propClearcoat] = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokClearcoatRough); ok {
		if mat.Properties == nil {
			mat.Properties = make(map[string]any)
		}
		mat.Properties[propClearcoatR] = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokIOR); ok {
		if mat.Properties == nil {
			mat.Properties = make(map[string]any)
		}
		mat.Properties[propIOR] = cr.readInlineFloat(f.valueRep)
	}

	asset.Materials = append(asset.Materials, mat)
}

type deferredShader struct {
	pathIdx int32
	fields  []crateField
}

type deferredGeomSubset struct {
	pathIdx int32
	fields  []crateField
}

type deferredBlendShape struct {
	pathIdx int32
	fields  []crateField
}

func wireDeferredShaders(cr *crateReader, asset *ir.Asset, matPathToIdx map[int32]int, shaders []deferredShader) {
	for i := range shaders {
		ds := &shaders[i]
		infoID := crateFieldStr(cr, ds.fields, tokInfoID)
		if infoID != tokUVTexture {
			continue
		}

		parentPath := cr.parentPathIdx(ds.pathIdx)
		matIdx, ok := matPathToIdx[parentPath]
		if !ok {
			grandparent := cr.parentPathIdx(parentPath)
			matIdx, ok = matPathToIdx[grandparent]
			if !ok {
				continue
			}
		}
		if matIdx < 0 || matIdx >= len(asset.Materials) {
			continue
		}

		filePath := crateFieldStr(cr, ds.fields, tokInputsFile)
		filePath = strings.Trim(filePath, "@")
		if filePath == "" {
			continue
		}

		tex := &ir.Texture{
			Name:  cr.pathName(ds.pathIdx),
			WrapS: mapWrapMode(crateFieldStr(cr, ds.fields, tokInputsWrapS)),
			WrapT: mapWrapMode(crateFieldStr(cr, ds.fields, tokInputsWrapT)),
		}
		tex.ImageIndex = len(asset.Images)
		asset.Images = append(asset.Images, &ir.ImageAsset{
			Name:       cr.pathName(ds.pathIdx),
			SourcePath: filePath,
		})
		texIdx := len(asset.Textures)
		asset.Textures = append(asset.Textures, tex)
		ref := &ir.TextureRef{TextureIndex: texIdx, Tiling: [2]float32{1, 1}}

		mat := asset.Materials[matIdx]
		shaderName := cr.pathName(ds.pathIdx)
		switch {
		case strings.Contains(shaderName, chanMetallic):
			mat.MetallicTexture = ref
		case strings.Contains(shaderName, chanRoughness):
			mat.RoughnessTexture = ref
		case strings.Contains(shaderName, chanNormal):
			mat.NormalTexture = ref
		case strings.Contains(shaderName, chanEmissive):
			mat.EmissiveTexture = ref
		case strings.Contains(shaderName, chanOcclusion):
			mat.OcclusionTexture = ref
		default:
			if mat.BaseColorTexture == nil {
				mat.BaseColorTexture = ref
			}
		}
	}
}

func wireDeferredGeomSubsets(
	cr *crateReader, asset *ir.Asset,
	meshPathToIdx map[int32]int, gsubsets []deferredGeomSubset,
) {
	type subsetGroup struct {
		meshIdx int
		subsets []geomSubset
	}
	groups := make(map[int32]*subsetGroup)

	for i := range gsubsets {
		gs := &gsubsets[i]
		parentPath := cr.parentPathIdx(gs.pathIdx)
		meshIdx, ok := meshPathToIdx[parentPath]
		if !ok {
			continue
		}
		elemType := crateFieldStr(cr, gs.fields, tokElementType)
		if elemType != "" && elemType != usdaElementFace {
			continue
		}
		family := crateFieldStr(cr, gs.fields, tokFamilyName)
		if family != "" && family != tokMaterialBind {
			continue
		}
		var faceIndices []int
		if f, found := cr.findFieldValue(gs.fields, tokSubsetIndices); found {
			raw := cr.readIntArray(f.valueRep)
			faceIndices = make([]int, len(raw))
			for j, v := range raw {
				faceIndices[j] = int(v)
			}
		}
		if len(faceIndices) == 0 {
			continue
		}
		var matName string
		if s := crateFieldStr(cr, gs.fields, tokMatBinding); s != "" {
			if idx := strings.LastIndex(s, "/"); idx >= 0 {
				matName = s[idx+1:]
			} else {
				matName = s
			}
		}
		g, ok := groups[parentPath]
		if !ok {
			g = &subsetGroup{meshIdx: meshIdx}
			groups[parentPath] = g
		}
		g.subsets = append(g.subsets, geomSubset{faceIndices: faceIndices, matName: matName})
	}

	for _, g := range groups {
		if g.meshIdx < 0 || g.meshIdx >= len(asset.Meshes) {
			continue
		}
		mesh := asset.Meshes[g.meshIdx]
		if len(mesh.Primitives) == 0 {
			continue
		}
		subs := splitBySubsets(mesh.Primitives[0], g.subsets, asset.Materials)
		mesh.Primitives = subs
	}
}

func wireDeferredBlendShapes(
	cr *crateReader, asset *ir.Asset,
	meshPathToIdx map[int32]int, bshapes []deferredBlendShape,
) {
	type bsGroup struct {
		meshIdx int
		targets []ir.MorphTarget
	}
	groups := make(map[int32]*bsGroup)

	for i := range bshapes {
		bs := &bshapes[i]
		parentPath := cr.parentPathIdx(bs.pathIdx)
		meshIdx, ok := meshPathToIdx[parentPath]
		if !ok {
			continue
		}
		mt := ir.MorphTarget{Name: cr.pathName(bs.pathIdx)}
		if f, found := cr.findFieldValue(bs.fields, tokBlendOffsets); found {
			mt.Positions = cr.readVec3fArray(f.valueRep)
		}
		if f, found := cr.findFieldValue(bs.fields, tokBlendPointIdx); found {
			raw := cr.readIntArray(f.valueRep)
			mt.Indices = make([]uint32, len(raw))
			for j, v := range raw {
				mt.Indices[j] = uint32(v)
			}
		}
		if f, found := cr.findFieldValue(bs.fields, tokBlendNormalOff); found {
			mt.Normals = cr.readVec3fArray(f.valueRep)
		}
		if len(mt.Positions) == 0 {
			continue
		}
		g, ok := groups[parentPath]
		if !ok {
			g = &bsGroup{meshIdx: meshIdx}
			groups[parentPath] = g
		}
		g.targets = append(g.targets, mt)
	}

	for _, g := range groups {
		if g.meshIdx < 0 || g.meshIdx >= len(asset.Meshes) {
			continue
		}
		mesh := asset.Meshes[g.meshIdx]
		for pi := range mesh.Primitives {
			mesh.Primitives[pi].MorphTargets = g.targets
		}
	}
}

func convertCrateBasisCurves(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	var prim ir.Primitive
	prim.Mode = ir.LineStrip
	prim.MaterialIndex = ir.NoIndex

	if f, ok := cr.findFieldValue(fields, tokPoints); ok {
		prim.Data.Positions = cr.readVec3fArray(f.valueRep)
	}
	if len(prim.Data.Positions) == 0 {
		return
	}
	if f, ok := cr.findFieldValue(fields, tokCurveVertCounts); ok {
		counts := cr.readIntArray(f.valueRep)
		if len(counts) > 0 {
			prim.Mode = ir.Lines
			prim.Data.Indices = curvesToLineIndicesCrate(counts)
		}
	}
	prim.Data.VertexCount = len(prim.Data.Positions)

	mesh := &ir.Mesh{Name: name, Primitives: []ir.Primitive{prim}}
	asset.Meshes = append(asset.Meshes, mesh)
	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:      name,
		MeshIndex: len(asset.Meshes) - 1,
		SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}

func convertCratePoints(cr *crateReader, fields []crateField, name string, asset *ir.Asset) {
	var prim ir.Primitive
	prim.Mode = ir.Points
	prim.MaterialIndex = ir.NoIndex

	if f, ok := cr.findFieldValue(fields, tokPoints); ok {
		prim.Data.Positions = cr.readVec3fArray(f.valueRep)
	}
	if len(prim.Data.Positions) == 0 {
		return
	}
	prim.Data.VertexCount = len(prim.Data.Positions)

	mesh := &ir.Mesh{Name: name, Primitives: []ir.Primitive{prim}}
	asset.Meshes = append(asset.Meshes, mesh)
	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:      name,
		MeshIndex: len(asset.Meshes) - 1,
		SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}

func curvesToLineIndicesCrate(counts []int32) []uint32 {
	var indices []uint32
	offset := uint32(0)
	for _, c := range counts {
		n := int(c)
		for i := 0; i < n-1; i++ {
			indices = append(indices, offset+uint32(i), offset+uint32(i+1))
		}
		offset += uint32(c)
	}
	return indices
}

func convertCrateProceduralPrim(cr *crateReader, fields []crateField, name, typeName string, asset *ir.Asset) {
	size := float32(cubeHalfDiv)
	radius := float32(1)
	height := float32(cubeHalfDiv)

	if f, ok := cr.findFieldValue(fields, tokSize); ok {
		size = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokRadius); ok {
		radius = cr.readInlineFloat(f.valueRep)
	}
	if f, ok := cr.findFieldValue(fields, tokHeight); ok {
		height = cr.readInlineFloat(f.valueRep)
	}

	var prim ir.Primitive
	prim.Mode = ir.Triangles
	prim.MaterialIndex = ir.NoIndex

	switch typeName {
	case tokCube:
		prim.Data = genCubeMesh(size)
	case tokSphere:
		prim.Data = genSphereMesh(radius, proceduralSegs, proceduralRings)
	case tokCylinder:
		prim.Data = genCylinderMesh(radius, height, proceduralSegs)
	case tokCone:
		prim.Data = genConeMesh(radius, height, proceduralSegs)
	case tokCapsule:
		prim.Data = genCapsuleMesh(radius, height, proceduralSegs, proceduralHemiRings)
	}

	if len(prim.Data.Positions) == 0 {
		return
	}
	prim.Data.VertexCount = len(prim.Data.Positions)
	mesh := &ir.Mesh{
		Name:       name,
		Primitives: []ir.Primitive{prim},
	}
	asset.Meshes = append(asset.Meshes, mesh)
	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:      name,
		MeshIndex: len(asset.Meshes) - 1,
		SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
	}
	asset.Nodes = append(asset.Nodes, node)
	asset.RootNodes = append(asset.RootNodes, len(asset.Nodes)-1)
}
