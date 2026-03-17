package gltf

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/url"
	"strings"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/valyala/fastjson"
)

const (
	glbMagic      = 0x46546C67
	glbVersion    = 2
	glbHeaderSize = 12
	glbChunkHdr   = 8
	chunkJSON     = 0x4E4F534A
	chunkBIN      = 0x004E4942
	dataScheme    = "data:"
	base64Suffix  = ";base64"
)

var (
	errTooShort = errors.New("gltf: file too short")
	errNotGLB   = errors.New("gltf: not a GLB file")
	errNoJSON   = errors.New("gltf: missing JSON chunk")
)

type glbData struct {
	json []byte
	bin  []byte
}

func parseGLB(data []byte) (glbData, error) {
	if len(data) < glbHeaderSize {
		return glbData{}, errTooShort
	}
	magic := binary.LittleEndian.Uint32(data[0:])
	if magic != glbMagic {
		return glbData{}, errNotGLB
	}

	var result glbData
	off := glbHeaderSize

	for off+glbChunkHdr <= len(data) {
		chunkLen := int(binary.LittleEndian.Uint32(data[off:]))
		chunkType := binary.LittleEndian.Uint32(data[off+4:])
		off += glbChunkHdr

		if off+chunkLen > len(data) {
			break
		}
		switch chunkType {
		case chunkJSON:
			result.json = data[off : off+chunkLen]
		case chunkBIN:
			result.bin = data[off : off+chunkLen]
		}
		off += chunkLen
	}

	if result.json == nil {
		return glbData{}, errNoJSON
	}
	return result, nil
}

type doc struct {
	root  *fastjson.Value
	bufs  bufferSet
	extFS detect.SeekableFS
	opts  detect.DecodeOptions
}

const gltfReportedBy = "decode:model/gltf"
const (
	gltfExtensionsUsedNote     = "gltf.extensions_used"
	gltfExtensionsRequiredNote = "gltf.extensions_required"
)

var parserPool fastjson.ParserPool

func parseDoc(p *fastjson.Parser, jsonData, binData []byte, opts detect.DecodeOptions) (*doc, error) {
	v, err := p.ParseBytes(jsonData)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatID(gltfName), err.Error(), nil)
	}

	d := &doc{root: v, extFS: opts.FS, opts: opts}
	d.bufs.views = parseBufferViews(v)
	d.bufs.buffers, err = resolveBuffers(v, binData, opts)
	if err != nil {
		return nil, err
	}
	if err := d.resolveMeshoptBufferViews(); err != nil {
		return nil, err
	}
	reportExtensions(v, opts.Reporter, keyExtensionsUsed, gltfExtensionsUsedNote)
	reportExtensions(v, opts.Reporter, keyExtensionsRequired, gltfExtensionsRequiredNote)

	return d, nil
}

func parseBufferViews(root *fastjson.Value) []bufferView {
	arr := root.GetArray(keyBufferViews)
	out := make([]bufferView, len(arr))
	for i, bv := range arr {
		out[i] = bufferView{
			buffer:     bv.GetInt(keyBuffer),
			byteOffset: bv.GetInt(keyByteOffset),
			byteLength: bv.GetInt(keyByteLength),
			byteStride: bv.GetInt(keyByteStride),
			meshopt:    parseMeshoptBufferView(bv.Get(keyExtensions, keyEXTMeshoptComp)),
		}
	}
	return out
}

func resolveBuffers(root *fastjson.Value, binData []byte, opts detect.DecodeOptions) ([][]byte, error) {
	arr := root.GetArray(keyBuffers)
	out := make([][]byte, len(arr))
	for i, b := range arr {
		if i == 0 && binData != nil {
			out[0] = binData
			continue
		}
		uri := string(b.GetStringBytes(keyURI))
		if uri == "" {
			continue
		}
		if opts.Reporter != nil {
			opts.Reporter.AddDependency("buffer", uri, "buffer", gltfReportedBy)
		}
		data, err := resolveBufferData(uri, opts)
		if err != nil {
			return nil, err
		}
		out[i] = data
	}
	return out, nil
}

func resolveBufferData(uri string, opts detect.DecodeOptions) ([]byte, error) {
	if decutil.HasASCIIPrefixFold(uri, dataScheme) {
		data, err := decodeDataURI(uri)
		if err != nil {
			return nil, decutil.DecodeErr(ir.FormatGLTF, "failed to decode buffer data URI", err)
		}
		if err := decutil.CheckMaxFileSize(data, opts.MaxFileSize); err != nil {
			return nil, decutil.DecodeErr(ir.FormatGLTF, "buffer exceeds MaxFileSize limit", err)
		}
		return data, nil
	}
	if opts.FS == nil {
		return nil, decutil.DecodeErr(ir.FormatGLTF, "missing external buffer: "+uri, nil)
	}

	rc, err := opts.FS.Open(uri)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatGLTF, "failed to open external buffer: "+uri, err)
	}
	defer rc.Close() //nolint:errcheck // best-effort close

	data, err := decutil.ReadAllLimit(rc, opts.MaxFileSize)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatGLTF, "failed to read external buffer: "+uri, err)
	}
	return data, nil
}

func decodeDataURI(uri string) ([]byte, error) {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, errors.New("malformed data URI")
	}

	header := uri[:comma]
	payload := uri[comma+1:]
	if decutil.HasASCIISuffixFold(header, base64Suffix) {
		return base64.StdEncoding.DecodeString(payload)
	}
	decoded, err := url.QueryUnescape(payload)
	if err != nil {
		return nil, err
	}
	return []byte(decoded), nil
}

func reportExtensions(root *fastjson.Value, reporter detect.DecodeReporter, key, noteKey string) {
	if reporter == nil {
		return
	}
	for _, ext := range root.GetArray(key) {
		name := decutil.Bstr(ext.GetStringBytes())
		if name == "" {
			continue
		}
		reporter.AddProvenanceNote(noteKey, name)
	}
}

func parseAccessor(v *fastjson.Value) accessor {
	tc := typeElemCount(string(v.GetStringBytes(keyType)))
	a := accessor{
		bufferView:    v.GetInt(keyBufferView),
		byteOffset:    v.GetInt(keyByteOffset),
		componentType: v.GetInt(keyComponentType),
		count:         v.GetInt(keyCount),
		elemCount:     tc,
	}
	if sp := v.Get(keySparse); sp != nil {
		a.sparseCount = sp.GetInt(keyCount)
		if idx := sp.Get(keySparseIndices); idx != nil {
			a.sparseIdxBV = idx.GetInt(keyBufferView)
			a.sparseIdxOff = idx.GetInt(keyByteOffset)
			a.sparseIdxComp = idx.GetInt(keyComponentType)
		}
		if vals := sp.Get(keySparseValues); vals != nil {
			a.sparseValBV = vals.GetInt(keyBufferView)
			a.sparseValOff = vals.GetInt(keyByteOffset)
		}
	}
	return a
}

func (d *doc) getAccessor(idx int) accessor {
	arr := d.root.GetArray(keyAccessors)
	if idx < 0 || idx >= len(arr) {
		return accessor{}
	}
	return parseAccessor(arr[idx])
}

func (d *doc) convertDoc() (*ir.Asset, error) {
	if err := d.checkRequiredExtensions(); err != nil {
		return nil, err
	}

	asset := ir.NewAsset(ir.FormatGLTF)
	asset.UpAxis = ir.YUp
	asset.Unit = defaultUnit
	asset.Metadata.SourceVersion = string(d.root.Get(keyAsset).GetStringBytes(keyVersion))
	asset.Metadata.Generator = string(d.root.Get(keyAsset).GetStringBytes(keyGenerator))

	asset.Images = d.convertImages()
	asset.Materials = d.convertMaterials()
	asset.Textures = d.convertTextures()
	meshes, err := d.convertMeshesChecked()
	if err != nil {
		return nil, err
	}
	asset.Meshes = meshes
	asset.Cameras = d.convertCameras()
	asset.Skeletons = d.convertSkins()
	asset.Animations = d.convertAnimations()
	asset.Lights = d.convertLights()
	asset.Nodes, asset.RootNodes = d.convertNodes(asset)

	d.parseKHRMaterialVariants(asset)
	d.parsePerPrimitiveVariants(asset)
	markJointNodes(asset.Nodes, asset.Skeletons)

	return asset, nil
}

var supportedExtensions = map[string]bool{ //nolint:gochecknoglobals // extension registry
	"EXT_mesh_gpu_instancing":             true,
	"EXT_meshopt_compression":             true,
	"EXT_texture_webp":                    true,
	"KHR_lights_punctual":                 true,
	"KHR_mesh_quantization":               true,
	"KHR_texture_basisu":                  true,
	"KHR_materials_unlit":                 true,
	"KHR_texture_transform":               true,
	"KHR_materials_emissive_strength":     true,
	"KHR_materials_clearcoat":             true,
	"KHR_materials_sheen":                 true,
	"KHR_materials_transmission":          true,
	"KHR_materials_volume":                true,
	"KHR_materials_ior":                   true,
	"KHR_materials_specular":              true,
	"KHR_materials_anisotropy":            true,
	"KHR_materials_pbrSpecularGlossiness": true,
	"KHR_materials_dispersion":            true,
	"KHR_materials_diffuse_transmission":  true,
	"KHR_materials_iridescence":           true,
	"KHR_materials_variants":              true,
}

const extDraco = "KHR_draco_mesh_compression"
const extMeshopt = "EXT_meshopt_compression"

func (d *doc) checkRequiredExtensions() error {
	arr := d.root.GetArray(keyExtensionsRequired)
	for _, ext := range arr {
		name := decutil.Bstr(ext.GetStringBytes())
		if name == extDraco {
			return decutil.DecodeErr(ir.FormatID(gltfName), "KHR_draco_mesh_compression is required but not supported", nil)
		}
		if !supportedExtensions[name] {
			return decutil.DecodeErr(ir.FormatID(gltfName), "unsupported required extension: "+name, nil)
		}
	}
	return nil
}

func decodeStream(sysCtx context.Context, r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	if err := sysCtx.Err(); err != nil {
		return nil, err
	}
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, err
	}
	data, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatID(gltfName), err.Error(), err)
	}
	if err := sysCtx.Err(); err != nil {
		return nil, err
	}

	p := parserPool.Get()
	defer parserPool.Put(p)

	if len(data) >= glbHeaderSize && binary.LittleEndian.Uint32(data[0:]) == glbMagic {
		glb, err := parseGLB(data)
		if err != nil {
			return nil, err
		}
		d, err := parseDoc(p, glb.json, glb.bin, opts)
		if err != nil {
			return nil, err
		}
		return d.convertDoc()
	}

	d, err := parseDoc(p, data, nil, opts)
	if err != nil {
		return nil, err
	}
	return d.convertDoc()
}
