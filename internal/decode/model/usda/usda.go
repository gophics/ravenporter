package usda

import (
	"archive/zip"
	"bytes"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatUSD, Decoder: &Decoder{}}}
}

type Decoder struct{}

const usdaReportedBy = "decode:model/usda"

const (
	usdaFormatName = "USDA"
	extUSDA        = ".usda"
	extUSD         = ".usd"
	usdzExtension  = ".usdz"
	usdzMagic0     = 0x50
	usdzMagic1     = 0x4B
	usdzMinSize    = 4
	usdaProbeSize  = 64
	usdaMinProbe   = 4
	vec3MinFields  = 3
	vec2MinFields  = 2
	usdaFloatBits  = 32
	usdaUint32Base = 10
	usdaUint32Bits = 32

	usdaSourceVersion = "1.0"

	usdaMagicPrefix   = "#usda"
	usdaDefMesh       = "def Mesh"
	usdaDefXform      = "def Xform"
	usdaKeyUpAxis     = "upAxis"
	usdaKeyMetersUnit = "metersPerUnit"
	usdaUpZ           = `"Z"`
	usdaPropPoints    = "point3f[] points"
	usdaPropIndices   = "int[] faceVertexIndices"
	usdaPropNormals   = "normal3f[] normals"
	usdaPropTexCoord  = "texCoord2f[] primvars:st"
	usdaPropFaceCnt   = "int[] faceVertexCounts"
	usdaUnnamed       = "unnamed"

	usdaXformTranslate = "double3 xformOp:translate"
	usdaXformRotateXYZ = "float3 xformOp:rotateXYZ"
	usdaXformScale     = "double3 xformOp:scale"
	usdaXformMatrix    = "xformOp:transform"

	usdaDefCamera    = "def Camera"
	usdaCamFocalLen  = "float focalLength"
	usdaCamHAperture = "float horizontalAperture"
	usdaCamVAperture = "float verticalAperture"
	usdaCamClipRange = "float2 clippingRange"
	defaultFocalLen  = 50.0
	defaultHAperture = 36.0
	defaultVAperture = 24.0
	fovDivisor       = 2.0

	usdaDefDistLight   = "def DistantLight"
	usdaDefSphereLight = "def SphereLight"
	usdaDefDiskLight   = "def DiskLight"
	usdaDefRectLight   = "def RectLight"
	usdaDefCylLight    = "def CylinderLight"
	usdaLightColor     = "color3f inputs:color"
	usdaLightIntensity = "float inputs:intensity"
	usdaLightConeAngle = "float inputs:shaping:cone:angle"
	defaultLightIntens = 1.0

	usdaDefScope = "def Scope"

	usdaPropDisplayColor   = "color3f[] primvars:displayColor"
	usdaPropDisplayOpacity = "float[] primvars:displayOpacity"
	usdaMatBinding         = "rel material:binding"
	usdaDoubleSided        = "uniform bool doubleSided"
	usdaOrientation        = "uniform token orientation"
	usdaOrientLeft         = "leftHanded"
	usdaTokTrue            = "true"

	usdaDefMaterial      = "def Material"
	usdaDefShader        = "def Shader"
	usdaShaderSurface    = "UsdPreviewSurface"
	usdaShaderDiffuse    = "color3f inputs:diffuseColor"
	usdaShaderMetallic   = "float inputs:metallic"
	usdaShaderRough      = "float inputs:roughness"
	usdaShaderOpacity    = "float inputs:opacity"
	usdaShaderEmissive   = "color3f inputs:emissiveColor"
	usdaShaderNormal     = "normal3f inputs:normal"
	usdaShaderTexFile    = "asset inputs:file"
	usdaShaderConnect    = ".connect"
	usdaShaderUVTex      = "UsdUVTexture"
	usdaShaderClearcoat  = "float inputs:clearcoat"
	usdaShaderClearcoatR = "float inputs:clearcoatRoughness"
	usdaShaderIOR        = "float inputs:ior"
	usdaShaderOpacityThr = "float inputs:opacityThreshold"
	usdaTexWrapS         = "token inputs:wrapS"
	usdaTexWrapT         = "token inputs:wrapT"

	usdaDefGeomSubset = "def GeomSubset"
	usdaSubsetIndices = "int[] indices"
	usdaSubsetElement = "uniform token elementType"
	usdaSubsetFamily  = "uniform token familyName"
	usdaElementFace   = "face"

	usdaCamProjection = "token projection"
	usdaCamOrtho      = "orthographic"

	usdaDefSkeleton  = "def Skeleton"
	usdaDefSkelAnim  = "def SkelAnimation"
	usdaSkelJoints   = "uniform token[] joints"
	usdaSkelBind     = "matrix4d[] bindTransforms"
	usdaSkelBinding  = "rel skel:skeleton"
	usdaSkelJointIdx = "int[] primvars:skel:jointIndices"
	usdaSkelJointWt  = "float[] primvars:skel:jointWeights"

	usdaAnimTranslations = "float3[] translations"
	usdaAnimRotations    = "quatf[] rotations"
	usdaAnimScales       = "half3[] scales"
	usdaTimeSamples      = ".timeSamples"

	usdaDefBlendShape    = "def BlendShape"
	usdaBlendOffsets     = "vector3f[] offsets"
	usdaBlendPointIdx    = "int[] pointIndices"
	usdaBlendNormalOff   = "vector3f[] normalOffsets"
	usdaSkelBlendShapes  = "uniform token[] skel:blendShapes"
	usdaSkelBlendTargets = "rel skel:blendShapeTargets"

	usdaDefCube     = "def Cube"
	usdaDefSphere   = "def Sphere"
	usdaDefCylinder = "def Cylinder"
	usdaDefCone     = "def Cone"
	usdaDefCapsule  = "def Capsule"
	usdaPrimSize    = "double size"
	usdaPrimRadius  = "double radius"
	usdaPrimHeight  = "double height"
	usdaPrimAxis    = "token axis"

	usdaDefBasisCurves  = "def BasisCurves"
	usdaDefPoints       = "def Points"
	usdaDefNurbsCurves  = "def NurbsCurves"
	usdaCurvePoints     = "point3f[] points"
	usdaCurveVertCounts = "int[] curveVertexCounts"

	usdaVariantSets = "variantSets"

	usdaDefaultPrim = "defaultPrim"

	usdaRefPrefix = "references"
	usdaPayload   = "payload"

	usdaSublayers   = "subLayers"
	usdaInherits    = "inherits"
	usdaSpecializes = "specializes"

	propClearcoat  = "clearcoat"
	propClearcoatR = "clearcoatRoughness"
	propIOR        = "ior"

	chanDiffuse   = "diffuseColor"
	chanMetallic  = "metallic"
	chanRoughness = "roughness"
	chanNormal    = "normal"
	chanEmissive  = "emissiveColor"
	chanOcclusion = "occlusion"

	wrapClamp  = "clamp"
	wrapMirror = "mirror"

	probeWindow = 128
)

var (
	usdaMagicPrefixBytes = []byte(usdaMagicPrefix)
	usdcMagicBytes       = []byte(usdcMagic)
	zipLocalHeaderBytes  = []byte("PK\x03\x04")
)

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeRead(r, probeWindow, func(buf []byte) bool {
		if bytes.HasPrefix(buf, usdaMagicPrefixBytes) || hasUSDCMagic(buf) {
			return true
		}
		if bytes.HasPrefix(buf, zipLocalHeaderBytes) {
			return decutil.BytesContainsASCIIFold(buf, usdcExtension) ||
				decutil.BytesContainsASCIIFold(buf, extUSDA) ||
				decutil.BytesContainsASCIIFold(buf, extUSD)
		}
		return false
	})
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	data, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatUSD, "failed to read", err)
	}

	if hasUSDCMagic(data) {
		return decodeCrateToScene(data)
	}

	if len(data) >= usdzMinSize && data[0] == usdzMagic0 && data[1] == usdzMagic1 {
		return decodeUSDZ(data, opts.Reporter, opts.MaxFileSize)
	}

	parser := &usdaParser{
		ls:          decutil.LineScanner{Data: data},
		asset:       ir.NewAsset(ir.FormatUSD),
		maxFileSize: opts.MaxFileSize,
		reporter:    opts.Reporter,
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

func (d *Decoder) Extensions() []string {
	return []string{extUSDA, extUSD, usdcExtension, usdzExtension}
}
func (d *Decoder) FormatName() string { return usdaFormatName }

func decodeUSDZ(data []byte, reporter detect.DecodeReporter, maxFileSize int64) (*ir.Asset, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatUSD, "invalid usdz archive", err)
	}

	var asset *ir.Asset
	for _, f := range zr.File {
		if !hasUSDSceneExtension(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		inner, err := decutil.ReadAllLimit(rc, maxFileSize)
		rc.Close() //nolint:errcheck,gosec // best-effort
		if err != nil {
			return nil, decutil.DecodeErr(ir.FormatUSD, "failed to read usdz scene entry", err)
		}
		if hasUSDCMagic(inner) {
			var crateErr error
			asset, crateErr = decodeCrateToScene(inner)
			if crateErr != nil {
				return nil, crateErr
			}
		} else {
			parser := &usdaParser{
				ls:          decutil.LineScanner{Data: inner},
				asset:       ir.NewAsset(ir.FormatUSD),
				archive:     zr.File,
				maxFileSize: maxFileSize,
				reporter:    reporter,
			}
			parser.asset.UpAxis = ir.YUp
			parser.asset.Unit = 1.0
			parser.asset.Metadata.SourceVersion = usdaSourceVersion
			parser.parse()
			if parser.err != nil {
				return nil, parser.err
			}
			asset = parser.asset
		}
		break
	}
	if asset == nil {
		return nil, decutil.DecodeErr(ir.FormatUSD, "no usdc/usda file in usdz archive", nil)
	}

	for _, f := range zr.File {
		fmt := imgutil.ImageFormatFromPath(f.Name)
		if fmt == "" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		raw, err := decutil.ReadAllLimit(rc, maxFileSize)
		rc.Close() //nolint:errcheck,gosec // best-effort
		if err != nil {
			return nil, decutil.DecodeErr(ir.FormatUSD, "failed to read usdz image entry", err)
		}
		if len(raw) == 0 {
			continue
		}
		asset.Images = append(asset.Images, &ir.ImageAsset{
			Name:         f.Name,
			Format:       fmt,
			Compressed:   raw,
			SourceFormat: ir.FormatUSD,
		})
	}

	return asset, nil
}

func hasUSDSceneExtension(name string) bool {
	return decutil.HasASCIISuffixFold(name, usdcExtension) ||
		decutil.HasASCIISuffixFold(name, extUSDA) ||
		decutil.HasASCIISuffixFold(name, extUSD)
}

func hasUSDCMagic(data []byte) bool {
	return len(data) >= usdcMagicLen && bytes.Equal(data[:usdcMagicLen], usdcMagicBytes)
}

type usdaParser struct {
	ls          decutil.LineScanner
	asset       *ir.Asset
	depth       int
	archive     []*zip.File
	maxFileSize int64
	inherits    []inheritArc
	err         error
	reporter    detect.DecodeReporter
}

type inheritArc struct {
	nodeIdx  int
	basePath string
}

func (p *usdaParser) parse() {
	for b := p.ls.Next(); p.err == nil && b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		if strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, usdaDefMesh):
			p.parseMeshPrim(line)
		case strings.HasPrefix(line, usdaDefXform):
			p.parseXformPrim(line)
		case strings.HasPrefix(line, usdaDefCamera):
			p.parseCameraPrim(line)
		case strings.HasPrefix(line, usdaDefDistLight),
			strings.HasPrefix(line, usdaDefSphereLight),
			strings.HasPrefix(line, usdaDefDiskLight),
			strings.HasPrefix(line, usdaDefRectLight),
			strings.HasPrefix(line, usdaDefCylLight):
			p.parseLightPrim(line)
		case strings.HasPrefix(line, usdaDefScope):
			p.parseScopePrim(line)
		case strings.HasPrefix(line, usdaDefMaterial):
			p.parseMaterialPrim(line)
		case strings.HasPrefix(line, usdaDefSkeleton):
			p.parseSkeletonPrim(line)
		case strings.HasPrefix(line, usdaDefSkelAnim):
			p.parseSkelAnimPrim(line)
		case strings.HasPrefix(line, usdaDefCube),
			strings.HasPrefix(line, usdaDefSphere),
			strings.HasPrefix(line, usdaDefCylinder),
			strings.HasPrefix(line, usdaDefCone),
			strings.HasPrefix(line, usdaDefCapsule):
			p.parseProceduralPrim(line)
		case strings.HasPrefix(line, usdaDefBasisCurves):
			p.parseBasisCurvesPrim(line)
		case strings.HasPrefix(line, usdaDefPoints):
			p.parsePointsPrim(line)
		case strings.HasPrefix(line, usdaDefNurbsCurves):
			p.parseBasisCurvesPrim(line)
		case strings.Contains(line, usdaKeyUpAxis):
			p.parseUpAxis(line)
		case strings.Contains(line, usdaKeyMetersUnit):
			p.parseUnit(line)
		case strings.Contains(line, usdaDefaultPrim):
			p.parseDefaultPrim(line)
		case strings.Contains(line, usdaRefPrefix),
			strings.Contains(line, usdaPayload):
			p.resolveReference(line)
		case strings.Contains(line, usdaSublayers):
			p.parseSublayers(line)
		case strings.Contains(line, usdaInherits),
			strings.Contains(line, usdaSpecializes):
			p.parseInheritArc(line)
		case strings.Contains(line, usdaVariantSets):
			p.parseVariantSets()
		case line == "{":
			p.depth++
		case line == "}":
			p.depth--
		}
	}
	p.resolveInherits()
}

func (p *usdaParser) resolveReference(line string) {
	start := strings.IndexByte(line, '@')
	if start < 0 {
		return
	}
	end := strings.IndexByte(line[start+1:], '@')
	if end < 0 {
		return
	}
	refPath := line[start+1 : start+1+end]
	refPath = strings.TrimPrefix(refPath, "./")
	if refPath == "" {
		return
	}
	relation := "reference"
	if strings.Contains(line, usdaPayload) {
		relation = "payload"
	}
	if p.reporter != nil {
		p.reporter.AddDependency("scene", refPath, relation, usdaReportedBy)
	}
	if p.archive == nil {
		return
	}
	refScene, err := p.loadRefScene(refPath)
	if err != nil {
		p.err = err
		return
	}
	if refScene != nil {
		mergeRefScene(p.asset, refScene)
	}
}

func (p *usdaParser) loadRefScene(refPath string) (*ir.Asset, error) {
	refData, err := p.readArchiveFile(refPath)
	if err != nil || len(refData) == 0 {
		return nil, err
	}
	if hasUSDCMagic(refData) {
		refScene, err := decodeCrateToScene(refData)
		if err != nil {
			return nil, err
		}
		return refScene, nil
	}
	refParser := &usdaParser{
		ls:          decutil.LineScanner{Data: refData},
		asset:       ir.NewAsset(ir.FormatUSD),
		archive:     p.archive,
		maxFileSize: p.maxFileSize,
		reporter:    p.reporter,
	}
	refParser.asset.UpAxis = ir.YUp
	refParser.asset.Unit = 1.0
	refParser.parse()
	if refParser.err != nil {
		return nil, refParser.err
	}
	return refParser.asset, nil
}

func (p *usdaParser) readArchiveFile(name string) ([]byte, error) {
	for _, f := range p.archive {
		if !strings.HasSuffix(f.Name, "/"+name) && f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := decutil.ReadAllLimit(rc, p.maxFileSize)
		rc.Close() //nolint:errcheck,gosec // best-effort
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, nil
}

func mergeRefScene(dst, src *ir.Asset) {
	nodeOff := len(dst.Nodes)
	meshOff := len(dst.Meshes)
	matOff := len(dst.Materials)
	texOff := len(dst.Textures)
	imgOff := len(dst.Images)
	skelOff := len(dst.Skeletons)
	camOff := len(dst.Cameras)
	lightOff := len(dst.Lights)

	dst.Meshes = append(dst.Meshes, src.Meshes...)
	dst.Materials = append(dst.Materials, src.Materials...)
	dst.Textures = append(dst.Textures, src.Textures...)
	dst.Images = append(dst.Images, src.Images...)
	dst.Skeletons = append(dst.Skeletons, src.Skeletons...)
	dst.Cameras = append(dst.Cameras, src.Cameras...)
	dst.Lights = append(dst.Lights, src.Lights...)

	for i := range src.Nodes {
		n := src.Nodes[i]
		if n.MeshIndex != ir.NoIndex {
			n.MeshIndex += meshOff
		}
		if n.SkinIndex != ir.NoIndex {
			n.SkinIndex += skelOff
		}
		if n.CameraIndex != ir.NoIndex {
			n.CameraIndex += camOff
		}
		if n.LightIndex != ir.NoIndex {
			n.LightIndex += lightOff
		}
		for j := range n.Children {
			n.Children[j] += nodeOff
		}
		dst.Nodes = append(dst.Nodes, n)
	}
	for _, ri := range src.RootNodes {
		dst.RootNodes = append(dst.RootNodes, ri+nodeOff)
	}
	for _, tex := range dst.Textures[texOff:] {
		if tex != nil && tex.ImageIndex != ir.NoIndex {
			tex.ImageIndex += imgOff
		}
	}
	for _, m := range dst.Materials[matOff:] {
		offsetTexRef(m.BaseColorTexture, texOff)
		offsetTexRef(m.MetallicTexture, texOff)
		offsetTexRef(m.RoughnessTexture, texOff)
		offsetTexRef(m.NormalTexture, texOff)
		offsetTexRef(m.EmissiveTexture, texOff)
		offsetTexRef(m.OcclusionTexture, texOff)
	}
	for i := range src.Meshes {
		for j := range src.Meshes[i].Primitives {
			if src.Meshes[i].Primitives[j].MaterialIndex != ir.NoIndex {
				dst.Meshes[meshOff+i].Primitives[j].MaterialIndex += matOff
			}
		}
	}
}

func offsetTexRef(ref *ir.TextureRef, off int) {
	if ref != nil {
		ref.TextureIndex += off
	}
}

func (p *usdaParser) parseSublayers(line string) {
	for {
		atStart := strings.IndexByte(line, '@')
		if atStart < 0 {
			break
		}
		atEnd := strings.IndexByte(line[atStart+1:], '@')
		if atEnd < 0 {
			break
		}
		refPath := strings.TrimPrefix(line[atStart+1:atStart+1+atEnd], "./")
		line = line[atStart+1+atEnd+1:]
		if refPath == "" {
			continue
		}
		if p.reporter != nil {
			p.reporter.AddDependency("scene", refPath, "sublayer", usdaReportedBy)
		}
		if p.archive == nil {
			continue
		}
		refScene, err := p.loadRefScene(refPath)
		if err != nil {
			p.err = err
			return
		}
		if refScene != nil {
			mergeRefScene(p.asset, refScene)
		}
	}
}

func (p *usdaParser) parseInheritArc(line string) {
	start := strings.IndexByte(line, '<')
	if start < 0 {
		return
	}
	end := strings.IndexByte(line[start:], '>')
	if end < 0 {
		return
	}
	basePath := line[start+1 : start+end]
	basePath = strings.TrimPrefix(basePath, "/")

	if len(p.asset.Nodes) == 0 {
		return
	}
	nodeIdx := len(p.asset.Nodes) - 1
	p.inherits = append(p.inherits, inheritArc{nodeIdx: nodeIdx, basePath: basePath})
}

func (p *usdaParser) resolveInherits() {
	if len(p.inherits) == 0 {
		return
	}
	nameMap := make(map[string]int, len(p.asset.Nodes))
	for i := range p.asset.Nodes {
		nameMap[p.asset.Nodes[i].Name] = i
	}

	for _, arc := range p.inherits {
		baseName := arc.basePath
		if slashIdx := strings.LastIndexByte(arc.basePath, '/'); slashIdx >= 0 {
			baseName = arc.basePath[slashIdx+1:]
		}
		baseIdx, ok := nameMap[baseName]
		if !ok {
			continue
		}
		if arc.nodeIdx >= len(p.asset.Nodes) {
			continue
		}
		dst := &p.asset.Nodes[arc.nodeIdx]
		src := &p.asset.Nodes[baseIdx]

		if dst.MeshIndex == ir.NoIndex && src.MeshIndex != ir.NoIndex {
			dst.MeshIndex = src.MeshIndex
		}
		if dst.SkinIndex == ir.NoIndex && src.SkinIndex != ir.NoIndex {
			dst.SkinIndex = src.SkinIndex
		}
		if dst.CameraIndex == ir.NoIndex && src.CameraIndex != ir.NoIndex {
			dst.CameraIndex = src.CameraIndex
		}
		if dst.LightIndex == ir.NoIndex && src.LightIndex != ir.NoIndex {
			dst.LightIndex = src.LightIndex
		}
	}
}

func (p *usdaParser) parseMeshPrim(defLine string) { //nolint:funlen,gocyclo // mesh parsing needs all attributes inline
	name := extractQuotedName(defLine)
	mesh := &ir.Mesh{Name: name}
	var prim ir.Primitive
	prim.Mode = ir.Triangles
	prim.MaterialIndex = ir.NoIndex
	var faceCounts []int
	var meshMatName string
	var skelName string
	var doubleSided bool
	var leftHanded bool
	var subsets []geomSubset
	var morphTargets []ir.MorphTarget

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)

		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				if faceCounts != nil && len(prim.Data.Indices) > 0 {
					prim.Data.Indices = triangulateFaceCounts(prim.Data.Indices, faceCounts)
				}
				if leftHanded {
					flipWindingOrder(prim.Data.Indices)
				}
				prim.Data.VertexCount = len(prim.Data.Positions)
				if meshMatName != "" {
					for i, m := range p.asset.Materials {
						if m.Name == meshMatName {
							prim.MaterialIndex = i
							break
						}
					}
				}
				if doubleSided && prim.MaterialIndex != ir.NoIndex {
					p.asset.Materials[prim.MaterialIndex].DoubleSided = true
				}
				if len(subsets) > 0 {
					subs := splitBySubsets(prim, subsets, p.asset.Materials)
					for si := range subs {
						subs[si].MorphTargets = morphTargets
					}
					mesh.Primitives = append(mesh.Primitives, subs...)
				} else {
					prim.MorphTargets = morphTargets
					mesh.Primitives = append(mesh.Primitives, prim)
				}
				p.asset.Meshes = append(p.asset.Meshes, mesh)
				skinIdx := ir.NoIndex
				if skelName != "" {
					for i, s := range p.asset.Skeletons {
						if s.Name == skelName {
							skinIdx = i
							break
						}
					}
				}
				node := ir.Node{LODGroupIndex: ir.NoIndex,
					Name:      name,
					MeshIndex: len(p.asset.Meshes) - 1,
					SkinIndex: skinIdx, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex,
				}
				p.asset.Nodes = append(p.asset.Nodes, node)
				p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
				return
			}
		case strings.Contains(line, usdaPropPoints):
			if strings.Contains(line, usdaTimeSamples) {
				times, frames := p.parseTimeSampledVec3()
				if len(frames) > 0 {
					prim.Data.Positions = frames[0]
					for fi := 1; fi < len(frames); fi++ {
						mt := ir.MorphTarget{Name: "frame_" + strconv.Itoa(int(times[fi]))}
						mt.Positions = make([][3]float32, len(frames[fi]))
						for vi := range frames[fi] {
							if vi < len(frames[0]) {
								mt.Positions[vi] = [3]float32{
									frames[fi][vi][0] - frames[0][vi][0],
									frames[fi][vi][1] - frames[0][vi][1],
									frames[fi][vi][2] - frames[0][vi][2],
								}
							}
						}
						morphTargets = append(morphTargets, mt)
					}
				}
			} else {
				prim.Data.Positions = parseVec3Array(p.collectArray(line))
			}
		case strings.Contains(line, usdaPropIndices):
			prim.Data.Indices = parseUint32Array(p.collectArray(line))
		case strings.Contains(line, usdaPropNormals):
			prim.Data.Normals = parseVec3Array(p.collectArray(line))
		case strings.Contains(line, usdaPropTexCoord):
			prim.Data.TexCoord0 = parseVec2Array(p.collectArray(line))
		case strings.Contains(line, usdaPropFaceCnt):
			faceCounts = parseIntArray(p.collectArray(line))
		case strings.Contains(line, usdaPropDisplayColor):
			prim.Data.Colors0 = parseColor3Array(p.collectArray(line))
		case strings.Contains(line, usdaPropDisplayOpacity):
			opacities := parseFloatArray(p.collectArray(line))
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
		case strings.Contains(line, usdaOrientation):
			if strings.Contains(line, usdaOrientLeft) {
				leftHanded = true
			}
		case strings.Contains(line, usdaMatBinding):
			meshMatName = extractMatBindingName(line)
		case strings.Contains(line, usdaDoubleSided):
			if strings.Contains(line, usdaTokTrue) {
				doubleSided = true
			}
		case strings.HasPrefix(line, usdaDefGeomSubset):
			subsets = append(subsets, p.parseGeomSubset())
		case strings.Contains(line, usdaSkelJointIdx):
			prim.Data.Joints0 = parseJointIndices(p.collectArray(line))
		case strings.Contains(line, usdaSkelJointWt):
			prim.Data.Weights0 = parseJointWeights(p.collectArray(line))
		case strings.Contains(line, usdaSkelBinding):
			skelName = extractMatBindingName(line)
		case strings.HasPrefix(line, usdaDefBlendShape):
			morphTargets = append(morphTargets, p.parseBlendShape(line))
		}
	}
}

func triangulateFaceCounts(indices []uint32, faceCounts []int) []uint32 {
	out := make([]uint32, 0, len(indices))
	offset := 0
	for _, fc := range faceCounts {
		if offset+fc > len(indices) {
			break
		}
		for j := 2; j < fc; j++ {
			out = append(out, indices[offset], indices[offset+j-1], indices[offset+j])
		}
		offset += fc
	}
	return out
}

func (p *usdaParser) parseXformPrim(defLine string) { //nolint:funlen // xform dispatches all child prim types
	name := extractQuotedName(defLine)
	depth := 1
	var transform ir.Transform
	transform.Scale = mathx.IdentityScale
	transform.Rotation = mathx.IdentityQuat

	childStart := len(p.asset.Nodes)

	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				node := ir.Node{LODGroupIndex: ir.NoIndex,
					Name:        name,
					MeshIndex:   ir.NoIndex,
					SkinIndex:   ir.NoIndex,
					CameraIndex: ir.NoIndex,
					LightIndex:  ir.NoIndex,
					Transform:   transform,
				}
				p.asset.Nodes = append(p.asset.Nodes, node)
				parentIdx := len(p.asset.Nodes) - 1
				p.wireChildren(parentIdx, childStart)
				return
			}
		case strings.HasPrefix(line, usdaDefMesh):
			p.parseMeshPrim(line)
		case strings.HasPrefix(line, usdaDefCamera):
			p.parseCameraPrim(line)
		case strings.HasPrefix(line, usdaDefDistLight),
			strings.HasPrefix(line, usdaDefSphereLight),
			strings.HasPrefix(line, usdaDefDiskLight),
			strings.HasPrefix(line, usdaDefRectLight),
			strings.HasPrefix(line, usdaDefCylLight):
			p.parseLightPrim(line)
		case strings.HasPrefix(line, usdaDefXform):
			p.parseXformPrim(line)
		case strings.HasPrefix(line, usdaDefScope):
			p.parseScopePrim(line)
		case strings.HasPrefix(line, usdaDefMaterial):
			p.parseMaterialPrim(line)
		case strings.HasPrefix(line, usdaDefSkeleton):
			p.parseSkeletonPrim(line)
		case strings.HasPrefix(line, usdaDefSkelAnim):
			p.parseSkelAnimPrim(line)
		case strings.HasPrefix(line, usdaDefCube),
			strings.HasPrefix(line, usdaDefSphere),
			strings.HasPrefix(line, usdaDefCylinder),
			strings.HasPrefix(line, usdaDefCone),
			strings.HasPrefix(line, usdaDefCapsule):
			p.parseProceduralPrim(line)
		case strings.HasPrefix(line, usdaDefBasisCurves):
			p.parseBasisCurvesPrim(line)
		case strings.HasPrefix(line, usdaDefPoints):
			p.parsePointsPrim(line)
		case strings.HasPrefix(line, usdaDefNurbsCurves):
			p.parseBasisCurvesPrim(line)
		case strings.Contains(line, usdaRefPrefix),
			strings.Contains(line, usdaPayload):
			p.resolveReference(line)
		case strings.Contains(line, usdaSublayers):
			p.parseSublayers(line)
		case strings.Contains(line, usdaXformTranslate):
			v := parseVec3Value(p.extractValue(line))
			transform.Translation = v
		case strings.Contains(line, usdaXformRotateXYZ):
			v := parseVec3Value(p.extractValue(line))
			xr, yr, zr := float64(v[0])*mathx.DegToRad, float64(v[1])*mathx.DegToRad, float64(v[2])*mathx.DegToRad
			transform.Rotation = mathx.EulerToQuat(xr, yr, zr)
		case strings.Contains(line, usdaXformScale):
			v := parseVec3Value(p.extractValue(line))
			transform.Scale = v
		case strings.Contains(line, usdaXformMatrix):
			transform.Matrix = parseMatrix4d(p.extractValue(line))
		}
	}
}

func (p *usdaParser) parseScopePrim(defLine string) {
	name := extractQuotedName(defLine)
	depth := 1

	childStart := len(p.asset.Nodes)

	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				node := ir.Node{LODGroupIndex: ir.NoIndex,
					Name:        name,
					MeshIndex:   ir.NoIndex,
					SkinIndex:   ir.NoIndex,
					CameraIndex: ir.NoIndex,
					LightIndex:  ir.NoIndex,
				}
				p.asset.Nodes = append(p.asset.Nodes, node)
				parentIdx := len(p.asset.Nodes) - 1
				p.wireChildren(parentIdx, childStart)
				return
			}
		case strings.HasPrefix(line, usdaDefMesh):
			p.parseMeshPrim(line)
		case strings.HasPrefix(line, usdaDefCamera):
			p.parseCameraPrim(line)
		case strings.HasPrefix(line, usdaDefDistLight),
			strings.HasPrefix(line, usdaDefSphereLight),
			strings.HasPrefix(line, usdaDefDiskLight),
			strings.HasPrefix(line, usdaDefRectLight),
			strings.HasPrefix(line, usdaDefCylLight):
			p.parseLightPrim(line)
		case strings.HasPrefix(line, usdaDefXform):
			p.parseXformPrim(line)
		case strings.HasPrefix(line, usdaDefScope):
			p.parseScopePrim(line)
		case strings.HasPrefix(line, usdaDefMaterial):
			p.parseMaterialPrim(line)
		case strings.HasPrefix(line, usdaDefSkeleton):
			p.parseSkeletonPrim(line)
		case strings.HasPrefix(line, usdaDefSkelAnim):
			p.parseSkelAnimPrim(line)
		case strings.HasPrefix(line, usdaDefCube),
			strings.HasPrefix(line, usdaDefSphere),
			strings.HasPrefix(line, usdaDefCylinder),
			strings.HasPrefix(line, usdaDefCone),
			strings.HasPrefix(line, usdaDefCapsule):
			p.parseProceduralPrim(line)
		case strings.HasPrefix(line, usdaDefBasisCurves):
			p.parseBasisCurvesPrim(line)
		case strings.HasPrefix(line, usdaDefPoints):
			p.parsePointsPrim(line)
		case strings.HasPrefix(line, usdaDefNurbsCurves):
			p.parseBasisCurvesPrim(line)
		case strings.Contains(line, usdaRefPrefix),
			strings.Contains(line, usdaPayload):
			p.resolveReference(line)
		case strings.Contains(line, usdaSublayers):
			p.parseSublayers(line)
		}
	}
}

func (p *usdaParser) wireChildren(parentIdx, childStart int) {
	var children []int
	var newRoots []int
	for _, ri := range p.asset.RootNodes {
		if ri >= childStart && ri < parentIdx {
			children = append(children, ri)
		} else {
			newRoots = append(newRoots, ri)
		}
	}
	p.asset.Nodes[parentIdx].Children = children
	newRoots = append(newRoots, parentIdx)
	p.asset.RootNodes = newRoots
}

func (p *usdaParser) parseMaterialPrim(defLine string) { //nolint:funlen,gocyclo // material parsing handles many shader inputs
	name := extractQuotedName(defLine)
	mat := &ir.Material{
		Name:            name,
		BaseColorFactor: [4]float32{1, 1, 1, 1},
		RoughnessFactor: 0.5,
	}
	depth := 1
	var texShaders []texShader
	var connections map[string]string

	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case strings.HasSuffix(line, "{"):
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				p.wireTextures(mat, texShaders, connections)
				p.asset.Materials = append(p.asset.Materials, mat)
				return
			}
		case strings.Contains(line, usdaShaderConnect):
			if connections == nil {
				connections = make(map[string]string)
			}
			before, after, ok := strings.Cut(line, usdaShaderConnect)
			if ok {
				key := strings.TrimSpace(before)
				if li := strings.LastIndex(key, " "); li >= 0 {
					key = key[li+1:]
				}
				val := strings.TrimSpace(after)
				val = strings.Trim(val, "= <>")
				if key != "" {
					connections[key] = val
				}
			}
		case strings.Contains(line, usdaShaderUVTex):
			shaderName := extractQuotedName(line)
			filePath, wrapS, wrapT := p.parseTexShaderBlock()
			depth--
			if filePath != "" {
				texShaders = append(texShaders, texShader{name: shaderName, path: filePath, wrapS: wrapS, wrapT: wrapT})
			}
		case strings.Contains(line, usdaShaderDiffuse) && !strings.Contains(line, usdaShaderConnect):
			v := parseVec3Value(p.extractValue(line))
			mat.BaseColorFactor = [4]float32{v[0], v[1], v[2], 1}
		case strings.Contains(line, usdaShaderMetallic) && !strings.Contains(line, usdaShaderConnect):
			mat.MetallicFactor = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaShaderRough) && !strings.Contains(line, usdaShaderConnect):
			mat.RoughnessFactor = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaShaderOpacityThr) && !strings.Contains(line, usdaShaderConnect):
			mat.AlphaCutoff = parseF32(p.extractValue(line))
			if mat.AlphaCutoff > 0 {
				mat.AlphaMode = ir.AlphaMask
			}
		case strings.Contains(line, usdaShaderOpacity) && !strings.Contains(line, usdaShaderConnect):
			a := parseF32(p.extractValue(line))
			mat.BaseColorFactor[3] = a
		case strings.Contains(line, usdaShaderEmissive) && !strings.Contains(line, usdaShaderConnect):
			v := parseVec3Value(p.extractValue(line))
			mat.EmissiveFactor = v
		case strings.Contains(line, usdaShaderClearcoat) && !strings.Contains(line, usdaShaderConnect) &&
			!strings.Contains(line, usdaShaderClearcoatR):
			if mat.Properties == nil {
				mat.Properties = make(map[string]any)
			}
			mat.Properties[propClearcoat] = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaShaderClearcoatR) && !strings.Contains(line, usdaShaderConnect):
			if mat.Properties == nil {
				mat.Properties = make(map[string]any)
			}
			mat.Properties[propClearcoatR] = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaShaderIOR) && !strings.Contains(line, usdaShaderConnect):
			if mat.Properties == nil {
				mat.Properties = make(map[string]any)
			}
			mat.Properties[propIOR] = parseF32(p.extractValue(line))
		}
	}
}

func (p *usdaParser) parseTexShaderBlock() (filePath, wrapS, wrapT string) {
	depth := 1
	for sb := p.ls.Next(); sb != nil; sb = p.ls.Next() {
		sline := decutil.Bstr(sb)
		switch {
		case sline == "{":
			depth++
		case sline == "}":
			depth--
			if depth == 0 {
				return
			}
		case strings.Contains(sline, usdaShaderTexFile):
			filePath = extractAssetPath(sline)
		case strings.Contains(sline, usdaTexWrapS):
			wrapS = extractQuotedToken(sline)
		case strings.Contains(sline, usdaTexWrapT):
			wrapT = extractQuotedToken(sline)
		}
	}
	return
}

type texShader struct {
	name  string
	path  string
	wrapS string
	wrapT string
}

func (p *usdaParser) wireTextures(mat *ir.Material, shaders []texShader, connections map[string]string) {
	for _, ts := range shaders {
		tex := &ir.Texture{
			Name:  ts.name,
			WrapS: mapWrapMode(ts.wrapS),
			WrapT: mapWrapMode(ts.wrapT),
		}
		tex.ImageIndex = len(p.asset.Images)
		p.asset.Images = append(p.asset.Images, &ir.ImageAsset{
			Name:       ts.name,
			SourcePath: ts.path,
		})
		texIdx := len(p.asset.Textures)
		p.asset.Textures = append(p.asset.Textures, tex)
		ref := &ir.TextureRef{TextureIndex: texIdx, Tiling: [2]float32{1, 1}}

		for channel, target := range connections {
			if !strings.Contains(target, ts.name) {
				continue
			}
			switch {
			case strings.Contains(channel, chanDiffuse):
				mat.BaseColorTexture = ref
			case strings.Contains(channel, chanMetallic):
				mat.MetallicTexture = ref
			case strings.Contains(channel, chanRoughness):
				mat.RoughnessTexture = ref
			case strings.Contains(channel, chanNormal):
				mat.NormalTexture = ref
			case strings.Contains(channel, chanEmissive):
				mat.EmissiveTexture = ref
			case strings.Contains(channel, chanOcclusion):
				mat.OcclusionTexture = ref
			}
		}
	}
}

func extractAssetPath(line string) string {
	at1 := strings.Index(line, "@")
	if at1 < 0 {
		return ""
	}
	at2 := strings.Index(line[at1+1:], "@")
	if at2 < 0 {
		return ""
	}
	return line[at1+1 : at1+1+at2]
}

func (p *usdaParser) collectArray(firstLine string) string {
	if _, after, ok := strings.Cut(firstLine, "= ["); ok {
		if content, _, closed := strings.Cut(after, "]"); closed {
			return content
		}
		var sb strings.Builder
		sb.Grow(len(after))
		sb.WriteString(after)
		for b := p.ls.Next(); b != nil; b = p.ls.Next() {
			line := decutil.Bstr(b)
			if content, _, closed := strings.Cut(line, "]"); closed {
				sb.WriteString(content)
				break
			}
			sb.WriteString(line)
		}
		return sb.String()
	}
	return ""
}

func (p *usdaParser) parseUpAxis(line string) {
	if strings.Contains(line, usdaUpZ) {
		p.asset.UpAxis = ir.ZUp
	}
}

func (p *usdaParser) parseUnit(line string) {
	_, val, ok := strings.Cut(line, "=")
	if !ok {
		return
	}
	s := strings.TrimSpace(val)
	s = strings.TrimRight(s, ")")
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		p.asset.Unit = v
	}
}

func extractQuotedName(line string) string {
	_, afterQ1, ok := strings.Cut(line, "\"")
	if !ok {
		return usdaUnnamed
	}
	name, _, ok := strings.Cut(afterQ1, "\"")
	if !ok {
		return usdaUnnamed
	}
	return name
}

func parseColor3Array(s string) [][4]float32 {
	v3 := parseVec3Array(s)
	result := make([][4]float32, len(v3))
	for i, c := range v3 {
		result[i] = [4]float32{c[0], c[1], c[2], 1}
	}
	return result
}

func extractMatBindingName(line string) string {
	_, path, ok := strings.Cut(line, "</")
	if !ok {
		return ""
	}
	path, _, _ = strings.Cut(path, ">")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func parseVec3Array(s string) [][3]float32 {
	result := make([][3]float32, 0, strings.Count(s, "(")+1)
	for {
		open := strings.IndexByte(s, '(')
		if open < 0 {
			break
		}
		pEnd := strings.IndexByte(s[open:], ')')
		if pEnd < 0 {
			break
		}
		inner := s[open+1 : open+pEnd]
		s = s[open+pEnd+1:]

		c1 := strings.IndexByte(inner, ',')
		if c1 < 0 {
			continue
		}
		c2 := strings.IndexByte(inner[c1+1:], ',')
		if c2 < 0 {
			continue
		}
		c2 += c1 + 1
		x := parseF32(inner[:c1])
		y := parseF32(inner[c1+1 : c2])
		z := parseF32(inner[c2+1:])
		result = append(result, [3]float32{x, y, z})
	}
	return result
}

func parseVec2Array(s string) [][2]float32 {
	result := make([][2]float32, 0, strings.Count(s, "(")+1)
	for {
		open := strings.IndexByte(s, '(')
		if open < 0 {
			break
		}
		pEnd := strings.IndexByte(s[open:], ')')
		if pEnd < 0 {
			break
		}
		inner := s[open+1 : open+pEnd]
		s = s[open+pEnd+1:]

		c1 := strings.IndexByte(inner, ',')
		if c1 < 0 {
			continue
		}
		u := parseF32(inner[:c1])
		v := parseF32(inner[c1+1:])
		result = append(result, [2]float32{u, v})
	}
	return result
}

func parseIntArray(s string) []int {
	result := make([]int, 0, strings.Count(s, ",")+1)
	for s != "" {
		idx := strings.IndexByte(s, ',')
		var tok string
		if idx < 0 {
			tok = strings.TrimSpace(s)
			s = ""
		} else {
			tok = strings.TrimSpace(s[:idx])
			s = s[idx+1:]
		}
		if tok == "" {
			continue
		}
		if v, err := strconv.Atoi(tok); err == nil {
			result = append(result, v)
		}
	}
	return result
}

func parseUint32Array(s string) []uint32 {
	result := make([]uint32, 0, strings.Count(s, ",")+1)
	for s != "" {
		idx := strings.IndexByte(s, ',')
		var tok string
		if idx < 0 {
			tok = strings.TrimSpace(s)
			s = ""
		} else {
			tok = strings.TrimSpace(s[:idx])
			s = s[idx+1:]
		}
		if tok == "" {
			continue
		}
		if v, err := strconv.ParseUint(tok, usdaUint32Base, usdaUint32Bits); err == nil {
			result = append(result, uint32(v))
		}
	}
	return result
}

func parseF32(s string) float32 {
	return decutil.ParseF32(strings.TrimSpace(s))
}

func (p *usdaParser) extractValue(line string) string {
	_, val, ok := strings.Cut(line, "=")
	if !ok {
		return ""
	}
	return strings.TrimSpace(val)
}

func parseVec3Value(s string) [3]float32 {
	s = strings.Trim(s, "() ")
	c1 := strings.IndexByte(s, ',')
	if c1 < 0 {
		return [3]float32{}
	}
	c2 := strings.IndexByte(s[c1+1:], ',')
	if c2 < 0 {
		return [3]float32{}
	}
	c2 += c1 + 1
	return [3]float32{
		parseF32(strings.TrimSpace(s[:c1])),
		parseF32(strings.TrimSpace(s[c1+1 : c2])),
		parseF32(strings.TrimSpace(s[c2+1:])),
	}
}

func (p *usdaParser) parseCameraPrim(defLine string) {
	name := extractQuotedName(defLine)
	focalLen := float32(defaultFocalLen)
	hAperture := float32(defaultHAperture)
	vAperture := float32(defaultVAperture)
	var near, far float32
	isOrtho := false

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				cam := &ir.Camera{Name: name}
				if isOrtho {
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
				camIdx := len(p.asset.Cameras)
				p.asset.Cameras = append(p.asset.Cameras, cam)
				node := ir.Node{LODGroupIndex: ir.NoIndex,
					Name:        name,
					MeshIndex:   ir.NoIndex,
					SkinIndex:   ir.NoIndex,
					CameraIndex: camIdx,
					LightIndex:  ir.NoIndex,
				}
				p.asset.Nodes = append(p.asset.Nodes, node)
				p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
				return
			}
		case strings.Contains(line, usdaCamProjection):
			if strings.Contains(line, usdaCamOrtho) {
				isOrtho = true
			}
		case strings.Contains(line, usdaCamFocalLen):
			focalLen = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaCamHAperture):
			hAperture = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaCamVAperture):
			vAperture = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaCamClipRange):
			v := parseVec2Value(p.extractValue(line))
			near, far = v[0], v[1]
		}
	}
}

func (p *usdaParser) parseLightPrim(defLine string) {
	name := extractQuotedName(defLine)
	light := &ir.Light{
		Name:      name,
		Color:     [3]float32{1, 1, 1},
		Intensity: defaultLightIntens,
	}

	isDist := strings.HasPrefix(defLine, usdaDefDistLight)
	isDisk := strings.HasPrefix(defLine, usdaDefDiskLight)
	var coneAngle float32

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				switch {
				case isDist:
					light.Directional = &ir.DirectionalLight{}
				case isDisk:
					light.Spot = &ir.SpotLight{
						OuterConeAngle: float32(float64(coneAngle) * mathx.DegToRad),
						InnerConeAngle: float32(float64(coneAngle) * mathx.DegToRad),
					}
				default:
					light.Point = &ir.PointLight{}
				}
				lightIdx := len(p.asset.Lights)
				p.asset.Lights = append(p.asset.Lights, light)
				node := ir.Node{LODGroupIndex: ir.NoIndex,
					Name:        name,
					MeshIndex:   ir.NoIndex,
					SkinIndex:   ir.NoIndex,
					CameraIndex: ir.NoIndex,
					LightIndex:  lightIdx,
				}
				p.asset.Nodes = append(p.asset.Nodes, node)
				p.asset.RootNodes = append(p.asset.RootNodes, len(p.asset.Nodes)-1)
				return
			}
		case strings.Contains(line, usdaLightColor):
			c := parseVec3Value(p.extractValue(line))
			light.Color = c
		case strings.Contains(line, usdaLightIntensity):
			light.Intensity = parseF32(p.extractValue(line))
		case strings.Contains(line, usdaLightConeAngle):
			coneAngle = parseF32(p.extractValue(line))
		}
	}
}

func parseVec2Value(s string) [2]float32 {
	s = strings.Trim(s, "() ")
	c := strings.IndexByte(s, ',')
	if c < 0 {
		return [2]float32{}
	}
	return [2]float32{parseF32(strings.TrimSpace(s[:c])), parseF32(strings.TrimSpace(s[c+1:]))}
}

const mat4Elems = 16

func parseMatrix4d(s string) [16]float32 {
	var m [16]float32
	i := 0
	for i < mat4Elems && s != "" {
		for s != "" && (s[0] == '(' || s[0] == ')' || s[0] == ' ') {
			s = s[1:]
		}
		if s == "" {
			break
		}
		comma := strings.IndexByte(s, ',')
		var field string
		if comma >= 0 {
			field = strings.TrimSpace(s[:comma])
			s = s[comma+1:]
		} else {
			field = strings.TrimRight(strings.TrimSpace(s), ")")
			s = ""
		}
		if field != "" {
			m[i] = parseF32(field)
			i++
		}
	}
	return m
}
