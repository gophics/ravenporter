package usda

import (
	"errors"
	"math"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/pool"
)

const (
	usdcMagic      = "PXR-USDC"
	usdcMagicLen   = 8
	usdcHeaderSize = 88
	usdcVersionOff = 8
	usdcTocOff     = 16
	usdcExtension  = ".usdc"

	sectionTokens    = "TOKENS"
	sectionStrings   = "STRINGS"
	sectionFields    = "FIELDS"
	sectionFieldSets = "FIELDSETS"
	sectionPaths     = "PATHS"
	sectionSpecs     = "SPECS"

	crateFieldSize   = 12
	crateSpecSize    = 12
	cratePathSize    = 16
	crateU64Size     = 8
	crateU32Size     = 4
	maxTocSections   = 10000
	crateF32Size     = 4
	crateTocNameLen  = 16
	crateTocEntryLen = 32
	crateSiblingOff  = 12

	specTypePrim       = 1
	specTypePseudoRoot = 2

	valueRepPayShift = 8
	valueRepPayMask  = 0xFFFFFFFF
	valueRepAddrMask = 0x00FFFFFFFFFFFFFF

	lz4CompressedFlag    = 1
	maxLZ4UncompressSize = 256 << 20

	tokMesh       = "Mesh"
	tokCamera     = "Camera"
	tokXform      = "Xform"
	tokDistLight  = "DistantLight"
	tokSphLight   = "SphereLight"
	tokDiskLight  = "DiskLight"
	tokPoints     = "points"
	tokFaceIdx    = "faceVertexIndices"
	tokFaceCounts = "faceVertexCounts"
	tokNormals    = "normals"
	tokST         = "primvars:st"
	tokType       = "typeName"
	tokFocalLen   = "focalLength"
	tokHAperture  = "horizontalAperture"
	tokVAperture  = "verticalAperture"
	tokClipRange  = "clippingRange"
	tokColor      = "inputs:color"
	tokIntensity  = "inputs:intensity"
	tokConeAngle  = "inputs:shaping:cone:angle"

	vec3fStride  = 12
	vec2fStride  = 8
	vec3dStride  = 24
	crateF64Size = 8
	mat4dElems   = 16
	mat4dStride  = 128

	tokTranslate  = "xformOp:translate"
	tokRotateXYZ  = "xformOp:rotateXYZ"
	tokXformScale = "xformOp:scale"
	tokTransform  = "xformOp:transform"

	tokScope        = "Scope"
	tokRectLight    = "RectLight"
	tokCylLight     = "CylinderLight"
	tokDisplayColor = "primvars:displayColor"
	tokSkeleton     = "Skeleton"
	tokMaterial     = "Material"
	tokProjection   = "projection"
	tokDoubleSided  = "doubleSided"
	tokCube         = "Cube"
	tokSphere       = "Sphere"
	tokCylinder     = "Cylinder"
	tokCone         = "Cone"
	tokCapsule      = "Capsule"
	tokJoints       = "joints"
	tokSize         = "size"
	tokRadius       = "radius"
	tokHeight       = "height"
	tokBindXforms   = "bindTransforms"

	tokDisplayOpacity   = "primvars:displayOpacity"
	tokOrientation      = "orientation"
	tokJointIndices     = "primvars:skel:jointIndices"
	tokJointWeights     = "primvars:skel:jointWeights"
	tokSkelBinding      = "skel:skeleton"
	tokBasisCurves      = "BasisCurves"
	tokPointsPrim       = "Points"
	tokNurbsCurves      = "NurbsCurves"
	tokCurveVertCounts  = "curveVertexCounts"
	tokGeomSubset       = "GeomSubset"
	tokSubsetIndices    = "indices"
	tokElementType      = "elementType"
	tokFamilyName       = "familyName"
	tokMaterialBind     = "materialBind"
	tokSkelAnim         = "SkelAnimation"
	tokAnimTranslations = "translations"
	tokAnimRotations    = "rotations"
	tokAnimScales       = "scales"
	tokBlendShape       = "BlendShape"
	tokBlendOffsets     = "offsets"
	tokBlendPointIdx    = "pointIndices"
	tokBlendNormalOff   = "normalOffsets"

	tokDiffuseColor   = "inputs:diffuseColor"
	tokMetallic       = "inputs:metallic"
	tokRoughness      = "inputs:roughness"
	tokOpacity        = "inputs:opacity"
	tokEmissiveColor  = "inputs:emissiveColor"
	tokOpacityThr     = "inputs:opacityThreshold"
	tokClearcoat      = "inputs:clearcoat"
	tokClearcoatRough = "inputs:clearcoatRoughness"
	tokIOR            = "inputs:ior"
	tokMatBinding     = "material:binding"

	tokUpAxis      = "upAxis"
	tokUnit        = "metersPerUnit"
	tokDefPrim     = "defaultPrim"
	tokVariantSets = "variantSets"
	tokSubLayers   = "subLayers"
	tokReferences  = "references"
	tokPayload     = "payload"
	tokInherits    = "inherits"
	tokSpecializes = "specializes"
	tokTimeSamples = "points.timeSamples"

	tokShader      = "Shader"
	tokInfoID      = "info:id"
	tokInputsFile  = "inputs:file"
	tokInputsWrapS = "inputs:wrapS"
	tokInputsWrapT = "inputs:wrapT"
	tokUVTexture   = "UsdUVTexture"
)

var (
	errCrateShort    = errors.New("usdc: file too short")
	errCrateBadMagic = errors.New("usdc: invalid magic")
	errCrateBadTOC   = errors.New("usdc: invalid TOC")
)

type crateReader struct {
	data      []byte
	version   [3]uint8
	tokens    []string
	strings   []string
	fields    []crateField
	fieldSets []int32
	paths     []cratePath
	specs     []crateSpec
}

type crateField struct {
	tokenIdx int32
	valueRep uint64
}

type cratePath struct {
	tokenIdx   int32
	parentIdx  int32
	childIdx   int32
	siblingIdx int32
}

type crateSpec struct {
	pathIdx     int32
	fieldSetIdx int32
	specType    int32
}

func parseCrate(raw []byte) (*crateReader, error) {
	if len(raw) < usdcHeaderSize {
		return nil, errCrateShort
	}
	if !hasUSDCMagic(raw) {
		return nil, errCrateBadMagic
	}

	cr := &crateReader{data: raw}
	cr.version[0] = raw[usdcVersionOff]
	cr.version[1] = raw[usdcVersionOff+1]
	cr.version[2] = raw[usdcVersionOff+2]

	tocOffset := binread.ReadU64LE(raw[usdcTocOff:])
	if tocOffset == 0 || tocOffset >= uint64(len(raw)) {
		return nil, errCrateBadTOC
	}

	if err := cr.readTOC(int(tocOffset)); err != nil { //nolint:gosec // bounded
		return nil, err
	}
	return cr, nil
}

func (cr *crateReader) readTOC(offset int) error {
	if offset+crateU64Size > len(cr.data) {
		return errCrateBadTOC
	}
	numSections := binread.ReadU64LE(cr.data[offset:])
	if numSections > maxTocSections {
		return errCrateBadTOC
	}
	off := offset + crateU64Size

	for range numSections {
		if off+crateTocEntryLen > len(cr.data) {
			return errCrateBadTOC
		}

		nameEnd := off
		for nameEnd < off+crateTocNameLen && nameEnd < len(cr.data) && cr.data[nameEnd] != 0 {
			nameEnd++
		}
		name := string(cr.data[off:nameEnd])
		sectionOff := binread.ReadU64LE(cr.data[off+crateTocNameLen:])
		sectionSize := binread.ReadU64LE(cr.data[off+crateTocNameLen+crateU64Size:])
		off += crateTocEntryLen

		endU := sectionOff + sectionSize
		if sectionSize > uint64(len(cr.data)) || endU > uint64(len(cr.data)) || endU < sectionOff {
			continue
		}
		sData := cr.data[sectionOff:endU]

		switch name {
		case sectionTokens:
			cr.parseTokens(sData)
		case sectionStrings:
			cr.parseStrings(sData)
		case sectionFields:
			cr.parseFields(sData)
		case sectionFieldSets:
			cr.parseFieldSets(sData)
		case sectionPaths:
			cr.parsePaths(sData)
		case sectionSpecs:
			cr.parseSpecs(sData)
		}
	}
	return nil
}

func (cr *crateReader) parseTokens(d []byte) {
	if len(d) < crateU64Size {
		return
	}
	count := binread.ReadU64LE(d)
	off := crateU64Size

	if off < len(d) {
		compFlag := d[off]
		off++
		if compFlag&lz4CompressedFlag != 0 && off+crateU64Size <= len(d) {
			uncompSize := binread.ReadU64LE(d[off:])
			if uncompSize > maxLZ4UncompressSize { // 256MB max for metadata sections
				return
			}
			off += crateU64Size
			decompressed, err := pool.LZ4DecodeBlock(d[off:], int(uncompSize))
			if err == nil {
				d = decompressed
				off = 0
			}
		}
	}

	allocCount := count
	if maxT := uint64(len(d)); allocCount > maxT {
		allocCount = maxT // max 1 char per string
	}
	cr.tokens = make([]string, 0, allocCount)
	for uint64(len(cr.tokens)) < count && off < len(d) {
		start := off
		for off < len(d) && d[off] != 0 {
			off++
		}
		cr.tokens = append(cr.tokens, string(d[start:off]))
		off++
	}
}

func (cr *crateReader) parseStrings(d []byte) {
	if len(d) < crateU32Size {
		return
	}
	count := binread.ReadU32LE(d)
	allocCount := count
	if maxT := uint32(len(d)) / crateU32Size; allocCount > maxT { //nolint:gosec // positive length
		allocCount = maxT
	}
	cr.strings = make([]string, allocCount)
	off := crateU32Size
	for i := uint32(0); i < count && off+crateU32Size <= len(d); i++ {
		idx := binread.ReadU32LE(d[off:])
		off += crateU32Size
		if int(idx) < len(cr.tokens) {
			cr.strings[i] = cr.tokens[idx]
		}
	}
}

func (cr *crateReader) parseFields(d []byte) {
	if len(d) < crateU64Size {
		return
	}
	count := binread.ReadU64LE(d)
	off := crateU64Size

	if off < len(d) {
		compFlag := d[off]
		off++
		if compFlag&lz4CompressedFlag != 0 && off+crateU64Size <= len(d) {
			uncompSize := binread.ReadU64LE(d[off:])
			if uncompSize > maxLZ4UncompressSize {
				return
			}
			off += crateU64Size
			decompressed, err := pool.LZ4DecodeBlock(d[off:], int(uncompSize))
			if err == nil {
				d = decompressed
				off = 0
			}
		}
	}

	allocCount := count
	if maxF := uint64(len(d)-off) / crateFieldSize; allocCount > maxF { //nolint:gosec // bounded by size checked previously
		allocCount = maxF
	}
	cr.fields = make([]crateField, 0, allocCount)
	for uint64(len(cr.fields)) < count && off+crateFieldSize <= len(d) {
		f := crateField{
			tokenIdx: binread.ReadI32LE(d[off:]),
			valueRep: binread.ReadU64LE(d[off+crateU32Size:]),
		}
		cr.fields = append(cr.fields, f)
		off += crateFieldSize
	}
}

func (cr *crateReader) parseFieldSets(d []byte) {
	if len(d) < crateU64Size {
		return
	}
	count := binread.ReadU64LE(d)
	off := crateU64Size
	allocCount := count
	if maxFS := uint64(len(d)-off) / crateU32Size; allocCount > maxFS { //nolint:gosec // bounded length check
		allocCount = maxFS
	}
	cr.fieldSets = make([]int32, 0, allocCount)
	for uint64(len(cr.fieldSets)) < count && off+crateU32Size <= len(d) {
		cr.fieldSets = append(cr.fieldSets, binread.ReadI32LE(d[off:]))
		off += crateU32Size
	}
}

func (cr *crateReader) parsePaths(d []byte) {
	if len(d) < crateU64Size {
		return
	}
	count := binread.ReadU64LE(d)
	off := crateU64Size
	allocCount := count
	if maxP := uint64(len(d)-off) / cratePathSize; allocCount > maxP { //nolint:gosec // bounded length check
		allocCount = maxP
	}
	cr.paths = make([]cratePath, 0, allocCount)
	for uint64(len(cr.paths)) < count && off+cratePathSize <= len(d) {
		p := cratePath{
			tokenIdx:   binread.ReadI32LE(d[off:]),
			parentIdx:  binread.ReadI32LE(d[off+crateU32Size:]),
			childIdx:   binread.ReadI32LE(d[off+crateU32Size*2:]),
			siblingIdx: binread.ReadI32LE(d[off+crateSiblingOff:]),
		}
		cr.paths = append(cr.paths, p)
		off += cratePathSize
	}
}

func (cr *crateReader) parseSpecs(d []byte) {
	if len(d) < crateU64Size {
		return
	}
	count := binread.ReadU64LE(d)
	off := crateU64Size
	allocCount := count
	if maxS := uint64(len(d)-off) / crateSpecSize; allocCount > maxS { //nolint:gosec // bounded length check
		allocCount = maxS
	}
	cr.specs = make([]crateSpec, 0, allocCount)
	for uint64(len(cr.specs)) < count && off+crateSpecSize <= len(d) {
		s := crateSpec{
			pathIdx:     binread.ReadI32LE(d[off:]),
			fieldSetIdx: binread.ReadI32LE(d[off+crateU32Size:]),
			specType:    binread.ReadI32LE(d[off+crateU32Size*2:]),
		}
		cr.specs = append(cr.specs, s)
		off += crateSpecSize
	}
}

func (cr *crateReader) tokenName(idx int32) string {
	if idx >= 0 && int(idx) < len(cr.tokens) {
		return cr.tokens[idx]
	}
	return ""
}

func (cr *crateReader) pathName(idx int32) string {
	if idx >= 0 && int(idx) < len(cr.paths) {
		return cr.tokenName(cr.paths[idx].tokenIdx)
	}
	return ""
}

func (cr *crateReader) parentPathIdx(idx int32) int32 {
	if idx >= 0 && int(idx) < len(cr.paths) {
		return cr.paths[idx].parentIdx
	}
	return -1
}

func (cr *crateReader) specFields(spec crateSpec) []crateField {
	idx := int(spec.fieldSetIdx)
	if idx < 0 || idx >= len(cr.fieldSets) {
		return nil
	}
	var fields []crateField
	for i := idx; i < len(cr.fieldSets); i++ {
		fi := cr.fieldSets[i]
		if fi < 0 {
			break
		}
		if int(fi) < len(cr.fields) {
			fields = append(fields, cr.fields[fi])
		}
	}
	return fields
}

func (cr *crateReader) findFieldValue(fields []crateField, name string) (crateField, bool) {
	for _, f := range fields {
		if cr.tokenName(f.tokenIdx) == name {
			return f, true
		}
	}
	return crateField{}, false
}

func (cr *crateReader) readInlineFloat(vr uint64) float32 {
	bits := uint32((vr >> valueRepPayShift) & valueRepPayMask)
	return math.Float32frombits(bits)
}

func (cr *crateReader) readInlineToken(vr uint64) string {
	idx := int32((vr >> valueRepPayShift) & valueRepPayMask) //nolint:gosec // intentional truncation
	return cr.tokenName(idx)
}

func (cr *crateReader) readInlineString(vr uint64) string {
	idx := int32((vr >> valueRepPayShift) & valueRepPayMask) //nolint:gosec // intentional truncation
	if idx >= 0 && int(idx) < len(cr.strings) {
		return cr.strings[idx]
	}
	return ""
}

func (cr *crateReader) readInlineBool(vr uint64) bool {
	return (vr>>valueRepPayShift)&1 != 0
}

func (cr *crateReader) readVec3fArray(vr uint64) [][3]float32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*vec3fStride > len(cr.data) {
		return nil
	}
	result := make([][3]float32, count)
	for i := range count {
		off := start + i*vec3fStride
		result[i] = [3]float32{
			binread.ReadF32LE(cr.data[off:]),
			binread.ReadF32LE(cr.data[off+crateF32Size:]),
			binread.ReadF32LE(cr.data[off+crateF32Size*2:]),
		}
	}
	return result
}

func (cr *crateReader) readQuatfArray(vr uint64) [][4]float32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	const quatStride = crateF32Size * quatComponents
	if start+count*quatStride > len(cr.data) {
		return nil
	}
	result := make([][4]float32, count)
	for i := range count {
		off := start + i*quatStride
		w := binread.ReadF32LE(cr.data[off:])
		x := binread.ReadF32LE(cr.data[off+crateF32Size:])
		y := binread.ReadF32LE(cr.data[off+crateF32Size*2:])
		z := binread.ReadF32LE(cr.data[off+crateF32Size*3:])
		result[i] = [4]float32{x, y, z, w}
	}
	return result
}

func (cr *crateReader) readVec2fArray(vr uint64) [][2]float32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*vec2fStride > len(cr.data) {
		return nil
	}
	result := make([][2]float32, count)
	for i := range count {
		off := start + i*vec2fStride
		result[i] = [2]float32{
			binread.ReadF32LE(cr.data[off:]),
			binread.ReadF32LE(cr.data[off+crateF32Size:]),
		}
	}
	return result
}

func (cr *crateReader) readIntArray(vr uint64) []int32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*crateU32Size > len(cr.data) {
		return nil
	}
	result := make([]int32, count)
	for i := range count {
		result[i] = binread.ReadI32LE(cr.data[start+i*crateU32Size:])
	}
	return result
}

func (cr *crateReader) readUint32Array(vr uint64) []uint32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*crateU32Size > len(cr.data) {
		return nil
	}
	result := make([]uint32, count)
	for i := range count {
		result[i] = binread.ReadU32LE(cr.data[start+i*crateU32Size:])
	}
	return result
}

func (cr *crateReader) readFloatArray(vr uint64) []float32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*crateF32Size > len(cr.data) {
		return nil
	}
	result := make([]float32, count)
	for i := range count {
		result[i] = binread.ReadF32LE(cr.data[start+i*crateF32Size:])
	}
	return result
}

func (cr *crateReader) readJointIndices(vr uint64) [][4]uint16 {
	raw := cr.readIntArray(vr)
	if len(raw) == 0 {
		return nil
	}
	result := make([][4]uint16, (len(raw)+jointStride-1)/jointStride)
	for i, v := range raw {
		result[i/jointStride][i%jointStride] = uint16(v)
	}
	return result
}

func (cr *crateReader) readJointWeights(vr uint64) [][4]float32 {
	raw := cr.readFloatArray(vr)
	if len(raw) == 0 {
		return nil
	}
	result := make([][4]float32, (len(raw)+jointStride-1)/jointStride)
	for i, v := range raw {
		result[i/jointStride][i%jointStride] = v
	}
	return result
}

func (cr *crateReader) readFloat64(vr uint64) float64 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateF64Size > len(cr.data) {
		return 0
	}
	return binread.ReadF64LE(cr.data[offset:])
}

func (cr *crateReader) readVec3dArray(vr uint64) [][3]float32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*vec3dStride > len(cr.data) {
		return nil
	}
	result := make([][3]float32, count)
	for i := range count {
		off := start + i*vec3dStride
		result[i] = [3]float32{
			float32(binread.ReadF64LE(cr.data[off:])),
			float32(binread.ReadF64LE(cr.data[off+crateF64Size:])),
			float32(binread.ReadF64LE(cr.data[off+crateF64Size*2:])),
		}
	}
	return result
}

func (cr *crateReader) readMatrix4d(vr uint64) [16]float32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+mat4dStride > len(cr.data) {
		return [16]float32{}
	}
	var m [16]float32
	for i := range mat4dElems {
		m[i] = float32(binread.ReadF64LE(cr.data[offset+i*crateF64Size:]))
	}
	return m
}

func (cr *crateReader) readMatrix4dArray(vr uint64) [][16]float32 {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*mat4dStride > len(cr.data) {
		return nil
	}
	result := make([][16]float32, count)
	for i := range count {
		for j := range mat4dElems {
			result[i][j] = float32(binread.ReadF64LE(cr.data[start+i*mat4dStride+j*crateF64Size:]))
		}
	}
	return result
}

func (cr *crateReader) readTokenArray(vr uint64) []string {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*crateU32Size > len(cr.data) {
		return nil
	}
	result := make([]string, count)
	for i := range count {
		idx := binread.ReadI32LE(cr.data[start+i*crateU32Size:])
		result[i] = cr.tokenName(idx)
	}
	return result
}

func (cr *crateReader) readStringArray(vr uint64) []string {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*crateU32Size > len(cr.data) {
		return nil
	}
	result := make([]string, count)
	for i := range count {
		idx := binread.ReadI32LE(cr.data[start+i*crateU32Size:])
		if idx >= 0 && int(idx) < len(cr.strings) {
			result[i] = cr.strings[idx]
		}
	}
	return result
}

func (cr *crateReader) readTimeSampledVec3(vr uint64) (times []float32, frames [][][3]float32) {
	offset := int((vr >> valueRepPayShift) & valueRepAddrMask)
	if offset <= 0 || offset+crateU64Size > len(cr.data) {
		return nil, nil
	}
	count := int(binread.ReadU64LE(cr.data[offset:])) //nolint:gosec // bounded
	start := offset + crateU64Size
	if start+count*(crateF64Size+crateU64Size) > len(cr.data) {
		return nil, nil
	}
	times = make([]float32, 0, count)
	frames = make([][][3]float32, 0, count)
	for i := range count {
		entryOff := start + i*(crateF64Size+crateU64Size)
		frameVR := binread.ReadU64LE(cr.data[entryOff+crateF64Size:])
		frame := cr.readVec3fArray(frameVR)
		if frame == nil {
			frame = cr.readVec3dArray(frameVR)
		}
		if frame == nil {
			return nil, nil
		}
		times = append(times, float32(binread.ReadF64LE(cr.data[entryOff:])))
		frames = append(frames, frame)
	}
	return times, frames
}
