package abc

import (
	"errors"
	"io"
	"math"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatAlembic, Decoder: &Decoder{}}}
}

type Decoder struct{}

const (
	abcFormatName   = "Alembic"
	extABC          = ".abc"
	defaultObjName  = "Object"
	defaultMeshName = "AlembicMesh"

	ogawaHeaderSize = 16
	ogawaVersionOff = 6
	ogawaRootPosOff = 8
	ogawaMaxVersion = 9999
	ogawaU64Size    = 8

	ogawaDataFlag = 1 << 63
	ogawaAddrMask = ogawaDataFlag - 1

	rootIdxVersion    = 0
	rootIdxRootObject = 2
	rootIdxMetadata   = 3
	rootMinChildren   = 6

	maxRecurseDepth = 64
	maxChildCount   = 100000

	f32Size        = 4
	i32Size        = 4
	f64Size        = 8
	vec3Floats     = 3
	vec3Stride     = vec3Floats * f32Size
	vec2Floats     = 2
	vec2Stride     = vec2Floats * f32Size
	vec4Floats     = 4
	vec4Stride     = vec4Floats * f32Size
	minVertCount   = 3
	maxVertCount   = 1000000
	vertScanWindow = 512
	vertValidCount = 12
	vertCoordMax   = 1e6

	abcPropP          = "P"
	abcPropFaceIdx    = ".faceIndices"
	abcPropFaceCounts = ".faceCounts"
	abcPropN          = "N"
	abcPropUV         = "uv"
	abcPropST         = "st"
	abcPropCs         = "Cs"

	abcPropVals   = ".vals"
	xformF64Count = 16
	xformByteSize = xformF64Count * f64Size

	rootIdxTimeSampling = 4
	tsF64Size           = 8
	tsMinBytes          = tsF64Size * 2

	abcPropArbGeom = ".arbGeomParams"
	abcDefaultFPS  = 24.0
	abcVersionDiv  = 10000
	maxPropNameLen = 256
	xformAnimName  = "XformAnimation"
	abcMetaUpAxis  = "upAxis"
	abcMetaUpZ     = "Z"

	abcPropFocalLen  = "focalLength"
	abcPropHorizAp   = "horizontalAperture"
	abcPropVertAp    = "verticalAperture"
	abcPropNearClip  = "nearClippingPlane"
	abcPropFarClip   = "farClippingPlane"
	abcPropFocalLen2 = ".focalLength"
	fovFactor        = 2.0
)

var (
	ogawaMagic    = []byte{0x4F, 0x67, 0x61, 0x77, 0x61}
	errTruncated  = errors.New("alembic: truncated file")
	errBadArchive = errors.New("alembic: invalid archive structure")

	defaultBaseColor = [4]float32{0.8, 0.8, 0.8, 1.0}
)

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeBytes(r, ogawaMagic)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	data, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatAlembic, "failed to read", err)
	}

	p := &ogawaParser{
		data:  data,
		asset: ir.NewAsset(ir.FormatAlembic),
	}
	p.asset.UpAxis = ir.YUp
	p.asset.Metadata.SourceVersion = "1.0"

	if err := p.parse(); err != nil {
		return nil, decutil.DecodeErr(ir.FormatAlembic, "ogawa parse failed", err)
	}
	return p.asset, nil
}

func (d *Decoder) Extensions() []string { return []string{extABC} }
func (d *Decoder) FormatName() string   { return abcFormatName }

type ogawaParser struct {
	data         []byte
	asset        *ir.Asset
	startTime    float64
	timePerCycle float64
}

func (p *ogawaParser) parse() error {
	if len(p.data) < ogawaHeaderSize {
		return errTruncated
	}

	version := binread.ReadU16LE(p.data[ogawaVersionOff:])
	if version > ogawaMaxVersion {
		return errBadArchive
	}

	rootPos := binread.ReadU64LE(p.data[ogawaRootPosOff:])
	if rootPos == 0 || rootPos > uint64(len(p.data)) {
		return errTruncated
	}

	rootGroup, err := p.loadGroup(int(rootPos)) //nolint:gosec // bounded above
	if err != nil {
		return err
	}
	if len(rootGroup) < rootMinChildren {
		return errBadArchive
	}

	p.readArchiveVersion(rootGroup)
	p.readArchiveMetadata(rootGroup)
	p.readTimeSampling(rootGroup)

	if isGroup(rootGroup[rootIdxRootObject]) {
		if objPos := childAddr(rootGroup[rootIdxRootObject]); objPos > 0 {
			p.walkObject(objPos, 0)
		}
	}

	return nil
}

func (p *ogawaParser) readArchiveVersion(rootGroup []uint64) {
	if !isData(rootGroup[rootIdxVersion]) {
		return
	}
	dataPos := childAddr(rootGroup[rootIdxVersion])
	if dataPos <= 0 || dataPos > len(p.data)-ogawaU64Size-i32Size {
		return
	}
	abcVer := binread.ReadU32LE(p.data[dataPos+ogawaU64Size:])
	p.asset.Metadata.SourceVersion = versionString(abcVer)
}

func (p *ogawaParser) readArchiveMetadata(rootGroup []uint64) {
	if !isData(rootGroup[rootIdxMetadata]) {
		return
	}
	dataPos := childAddr(rootGroup[rootIdxMetadata])
	if dataPos <= 0 || dataPos > len(p.data)-ogawaU64Size {
		return
	}
	size := binread.ReadU64LE(p.data[dataPos:])
	start := dataPos + ogawaU64Size
	if size == 0 || size > uint64(len(p.data)) {
		return
	}
	end := start + int(size) //nolint:gosec // bounded above
	if end > len(p.data) || end < start {
		return
	}
	p.parseMetadata(string(p.data[start:end]))
}

func (p *ogawaParser) readTimeSampling(rootGroup []uint64) {
	p.timePerCycle = 1.0 / abcDefaultFPS
	if len(rootGroup) <= rootIdxTimeSampling {
		return
	}
	child := rootGroup[rootIdxTimeSampling]
	if !isData(child) {
		return
	}
	pos := childAddr(child)
	if pos <= 0 || pos >= len(p.data)-ogawaU64Size {
		return
	}
	size := binread.ReadU64LE(p.data[pos:])
	start := pos + ogawaU64Size
	if size < tsMinBytes || size > uint64(len(p.data)-start) { //nolint:gosec // bounded above
		return
	}
	p.startTime = math.Float64frombits(binread.ReadU64LE(p.data[start:]))
	p.timePerCycle = math.Float64frombits(binread.ReadU64LE(p.data[start+tsF64Size:]))
	if p.timePerCycle <= 0 {
		p.timePerCycle = 1.0 / abcDefaultFPS
	}
}

func (p *ogawaParser) loadGroup(offset int) ([]uint64, error) {
	if offset < 0 || offset > len(p.data)-ogawaU64Size {
		return nil, errTruncated
	}
	childCount := binread.ReadU64LE(p.data[offset:])
	if childCount == 0 || childCount > maxChildCount {
		return nil, nil
	}

	startOff := offset + ogawaU64Size
	endOff := startOff + int(childCount)*ogawaU64Size
	if endOff > len(p.data) || endOff < startOff {
		return nil, errTruncated
	}

	children := make([]uint64, childCount)
	for i := range children {
		children[i] = binread.ReadU64LE(p.data[startOff+i*ogawaU64Size:])
	}
	return children, nil
}

func (p *ogawaParser) walkObject(offset, depth int) {
	if depth > maxRecurseDepth {
		return
	}

	children, err := p.loadGroup(offset)
	if err != nil || len(children) == 0 {
		return
	}

	var xformSamples [][16]float32
	hasXform := false
	if isGroup(children[0]) {
		if cpPos := childAddr(children[0]); cpPos > 0 {
			p.tryExtractMeshFromProperties(cpPos, depth+1)
			xformSamples, hasXform = p.tryExtractXform(cpPos)
		}
	}

	objName := defaultObjName
	lastIdx := len(children) - 1
	if isData(children[lastIdx]) {
		objName = p.readObjectName(children[lastIdx])
	}

	if len(p.asset.Meshes) > 0 {
		last := p.asset.Meshes[len(p.asset.Meshes)-1]
		if last.Name == defaultMeshName && objName != defaultObjName {
			last.Name = objName
		}
	}

	if hasXform && len(p.asset.Nodes) > 0 {
		nodeIdx := len(p.asset.Nodes) - 1
		node := &p.asset.Nodes[nodeIdx]
		if node.Name == defaultMeshName && objName != defaultObjName {
			node.Name = objName
		}
		node.Transform = ir.Transform{Matrix: xformSamples[0]}

		if len(xformSamples) > 1 {
			p.asset.Animations = append(p.asset.Animations, buildXformAnimation(nodeIdx, xformSamples, p.startTime, p.timePerCycle))
		}
	}

	for i := 1; i < lastIdx; i++ {
		if !isGroup(children[i]) {
			continue
		}
		if subPos := childAddr(children[i]); subPos > 0 {
			p.walkObject(subPos, depth+1)
		}
	}
}

func (p *ogawaParser) readObjectName(child uint64) string {
	pos := childAddr(child)
	if pos <= 0 || pos >= len(p.data)-ogawaU64Size {
		return defaultObjName
	}
	size := binread.ReadU64LE(p.data[pos:])
	start := pos + ogawaU64Size
	if size < f32Size || size > uint64(len(p.data)-start) { //nolint:gosec // bounded above
		return defaultObjName
	}
	nameLen := int(binread.ReadU32LE(p.data[start:]))
	nameStart := start + f32Size
	nameEnd := nameStart + nameLen
	if nameLen < 0 || nameStart > len(p.data)-nameLen || nameEnd > start+int(size) { //nolint:gosec // size bounded above
		return defaultObjName
	}
	return string(p.data[nameStart:nameEnd])
}
