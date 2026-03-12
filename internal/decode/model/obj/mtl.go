package obj

import (
	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	defaultShininess          = 100.0
	defaultAlpha              = 1.0
	defaultRoughness          = 0.5
	defaultAlphaCutoff        = 0.5
	minColorFields            = 4
	minFields                 = 2
	propSpecularKey           = "specular"
	propAmbientKey            = "ambient"
	propIORKey                = "ior"
	propSheenKey              = "sheenFactor"
	propClearcoatKey          = "clearcoatFactor"
	propClearcoatRoughKey     = "clearcoatRoughnessFactor"
	propTransmittanceKey      = "transmittanceFilter"
	propAnisotropyStrengthKey = "anisotropyStrength"
	propAnisotropyRotationKey = "anisotropyRotation"

	mtlNewMtl     = "newmtl"
	mtlKa         = "Ka"
	mtlKd         = "Kd"
	mtlKs         = "Ks"
	mtlNs         = "Ns"
	mtlD          = "d"
	mtlTr         = "Tr"
	mtlPm         = "Pm"
	mtlPr         = "Pr"
	mtlPs         = "Ps"
	mtlPc         = "Pc"
	mtlPcr        = "Pcr"
	mtlTf         = "Tf"
	mtlAniso      = "aniso"
	mtlAnisor     = "anisor"
	mtlMapKd      = "map_Kd"
	mtlMapKs      = "map_Ks"
	mtlMapNs      = "map_Ns"
	mtlMapD       = "map_d"
	mtlMapKa      = "map_Ka"
	mtlMapBump    = "map_bump"
	mtlBump       = "bump"
	mtlNorm       = "norm"
	mtlKe         = "Ke"
	mtlMapKe      = "map_Ke"
	mtlIllum      = "illum"
	mtlNi         = "Ni"
	mtlMapPr      = "map_Pr"
	mtlMapPm      = "map_Pm"
	mtlMapPs      = "map_Ps"
	mtlDisp       = "disp"
	mtlRefl       = "refl"
	mtlOff        = "off"
	mtlZero       = "0"
	objReportedBy = "decode:model/obj"
)

func parseMTL(fs detect.SeekableFS, reporter detect.DecodeReporter, mtlLib string, maxFileSize int64, asset *ir.Asset) error {
	if fs == nil || mtlLib == "" {
		return nil
	}
	if reporter != nil {
		reporter.AddDependency("material-library", mtlLib, "material-library", objReportedBy)
	}
	f, err := fs.Open(mtlLib)
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck // best-effort close

	raw, err := decutil.ReadAllLimit(f, maxFileSize)
	if err != nil {
		return err
	}
	parseMTLBytes(raw, asset)
	return nil
}

func parseMTLBytes(raw []byte, asset *ir.Asset) { //nolint:funlen,gocyclo // dispatch switch
	sc := byteScanner{data: raw}
	var fs fieldSplitter
	var mat *ir.Material

	for {
		line, ok := sc.nextLine()
		if !ok {
			break
		}
		fs.split(line)
		if fs.count == 0 || fs.get(0)[0] == '#' {
			continue
		}

		cmd := decutil.Bstr(fs.get(0))
		switch cmd {
		case mtlNewMtl:
			flushMaterial(mat, asset)
			mat = newDefaultMaterial(fieldString(&fs, 1))
		case mtlKa:
			parseMTLAmbient(mat, &fs)
		case mtlKd:
			parseMTLColor(mat, &fs)
		case mtlKs:
			parseMTLSpecular(mat, &fs)
		case mtlNs:
			parseMTLShininess(mat, &fs)
		case mtlD:
			parseMTLDissolve(mat, &fs)
		case mtlTr:
			parseMTLTransparency(mat, &fs)
		case mtlPm:
			parseMTLFloatField(mat, &fs, func(m *ir.Material, v float32) { m.MetallicFactor = v })
		case mtlPr:
			parseMTLFloatField(mat, &fs, func(m *ir.Material, v float32) { m.RoughnessFactor = v })
		case mtlMapKd:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) { m.BaseColorTexture = ref })
		case mtlMapKs:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) { m.MetallicTexture = ref })
		case mtlMapNs:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) { m.RoughnessTexture = ref })
		case mtlMapD:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) {
				m.BaseColorTexture = ref
				m.AlphaMode = ir.AlphaBlend
			})
		case mtlMapBump, mtlBump, mtlNorm:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) {
				m.NormalTexture = ref
				m.NormalScale = 1.0
			})
		case mtlKe:
			parseMTLEmissive(mat, &fs)
		case mtlMapKe:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) { m.EmissiveTexture = ref })
		case mtlIllum:
			parseMTLIllum(mat, &fs)
		case mtlNi:
			parseMTLPropFloat(mat, &fs, propIORKey)
		case mtlMapPr:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) { m.RoughnessTexture = ref })
		case mtlMapPm:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) { m.MetallicTexture = ref })
		case mtlPs:
			parseMTLPropFloat(mat, &fs, propSheenKey)
		case mtlMapPs:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) {
				setProp(m, "sheenTexture", ref.TextureIndex)
			})
		case mtlPc:
			parseMTLPropFloat(mat, &fs, propClearcoatKey)
		case mtlPcr:
			parseMTLPropFloat(mat, &fs, propClearcoatRoughKey)
		case mtlTf:
			parseMTLPropColor(mat, &fs, propTransmittanceKey)
		case mtlAniso:
			parseMTLPropFloat(mat, &fs, propAnisotropyStrengthKey)
		case mtlAnisor:
			parseMTLPropFloat(mat, &fs, propAnisotropyRotationKey)
		case mtlMapKa:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) {
				setProp(m, "ambientTexture", ref.TextureIndex)
			})
		case mtlDisp:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) {
				setProp(m, "displacementTexture", ref.TextureIndex)
			})
		case mtlRefl:
			addTexture(mat, &fs, asset, func(m *ir.Material, ref *ir.TextureRef) {
				setProp(m, "reflectionTexture", ref.TextureIndex)
			})
		}
	}

	flushMaterial(mat, asset)
}

func fieldString(fs *fieldSplitter, i int) string {
	f := fs.get(i)
	if f == nil {
		return defaultGroupName
	}
	return string(f)
}

func newDefaultMaterial(name string) *ir.Material {
	return &ir.Material{
		Name:            name,
		BaseColorFactor: [4]float32{1, 1, 1, 1},
		RoughnessFactor: defaultRoughness,
		AlphaCutoff:     defaultAlphaCutoff,
	}
}

func flushMaterial(mat *ir.Material, asset *ir.Asset) {
	if mat != nil {
		asset.Materials = append(asset.Materials, mat)
	}
}

func parseMTLColor(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minColorFields {
		return
	}
	col, err := parseFloat3Bytes(fs.get(1), fs.get(2), fs.get(3)) //nolint:mnd // color channels
	if err != nil {
		return
	}
	mat.BaseColorFactor = [4]float32{col[0], col[1], col[2], mat.BaseColorFactor[3]}
}

func parseMTLAmbient(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minColorFields {
		return
	}
	col, err := parseFloat3Bytes(fs.get(1), fs.get(2), fs.get(3)) //nolint:mnd // color channels
	if err != nil {
		return
	}
	if mat.Properties == nil {
		mat.Properties = make(map[string]any)
	}
	mat.Properties[propAmbientKey] = col
}

func parseMTLSpecular(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minColorFields {
		return
	}
	col, err := parseFloat3Bytes(fs.get(1), fs.get(2), fs.get(3)) //nolint:mnd // color channels
	if err != nil {
		return
	}
	if mat.Properties == nil {
		mat.Properties = make(map[string]any)
	}
	mat.Properties[propSpecularKey] = col
}

func parseMTLShininess(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minFields {
		return
	}
	ns, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return
	}
	mat.RoughnessFactor = max(0, 1.0-ns/defaultShininess)
}

func parseMTLDissolve(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minFields {
		return
	}
	d, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return
	}
	mat.BaseColorFactor[3] = d
	if d < defaultAlpha {
		mat.AlphaMode = ir.AlphaBlend
	}
}

func parseMTLTransparency(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minFields {
		return
	}
	tr, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return
	}
	mat.BaseColorFactor[3] = 1.0 - tr
	if tr > 0 {
		mat.AlphaMode = ir.AlphaBlend
	}
}

func parseMTLFloatField(mat *ir.Material, fs *fieldSplitter, set func(*ir.Material, float32)) {
	if mat == nil || fs.count < minFields {
		return
	}
	v, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return
	}
	set(mat, v)
}

//nolint:funlen // dispatch switch for texture options
func addTexture(mat *ir.Material, fs *fieldSplitter, asset *ir.Asset, set func(*ir.Material, *ir.TextureRef)) {
	if mat == nil || fs.count < minFields {
		return
	}

	ref := &ir.TextureRef{Tiling: [2]float32{1, 1}}
	var nameField []byte

	for i := 1; i < fs.count; i++ {
		field := fs.get(i)
		switch decutil.Bstr(field) {
		case "-s":
			if i+1 < fs.count {
				if v, err := parseFloatBytes(fs.get(i + 1)); err == nil {
					ref.Tiling[0] = v
					ref.Tiling[1] = v
				}
				i++
				if i+1 < fs.count && fs.get(i + 1)[0] != '-' {
					if v, err := parseFloatBytes(fs.get(i + 1)); err == nil {
						ref.Tiling[1] = v
					}
					i++
				}
			}
		case "-o":
			if i+1 < fs.count {
				if v, err := parseFloatBytes(fs.get(i + 1)); err == nil {
					ref.Offset[0] = v
				}
				i++
				if i+1 < fs.count && fs.get(i + 1)[0] != '-' {
					if v, err := parseFloatBytes(fs.get(i + 1)); err == nil {
						ref.Offset[1] = v
					}
					i++
				}
			}
		case "-bm":
			if i+1 < fs.count {
				if v, err := parseFloatBytes(fs.get(i + 1)); err == nil {
					setProp(mat, "bumpMultiplier", v)
				}
				i++
			}
		case "-t":
			if i+1 < fs.count {
				i++
				if i+1 < fs.count && fs.get(i + 1)[0] != '-' {
					i++
				}
			}
		case "-blendu", "-blendv", "-clamp":
			if i+1 < fs.count {
				i++
			}
		case "-mm":
			if i+2 < fs.count {
				i += 2
			}
		case "-texres":
			if i+1 < fs.count {
				i++
			}
		default:
			nameField = field
		}
	}

	if len(nameField) == 0 {
		return
	}
	name := string(nameField)
	imageIndex := len(asset.Images)
	asset.Images = append(asset.Images, &ir.ImageAsset{
		Name:       name,
		SourcePath: name,
	})
	ref.TextureIndex = len(asset.Textures)
	asset.Textures = append(asset.Textures, &ir.Texture{
		Name:       name,
		ImageIndex: imageIndex,
	})
	set(mat, ref)
}

func parseMTLEmissive(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minColorFields {
		return
	}
	col, err := parseFloat3Bytes(fs.get(1), fs.get(2), fs.get(3)) //nolint:mnd // color format
	if err != nil {
		return
	}
	mat.EmissiveFactor = col
}

const illumTransparent = 4

func parseMTLIllum(mat *ir.Material, fs *fieldSplitter) {
	if mat == nil || fs.count < minFields {
		return
	}
	v, err := parseIntBytes(fs.get(1))
	if err != nil {
		return
	}
	if v >= illumTransparent && mat.BaseColorFactor[3] < defaultAlpha {
		mat.AlphaMode = ir.AlphaBlend
	}
}

func setProp(mat *ir.Material, key string, val any) {
	if mat.Properties == nil {
		mat.Properties = make(map[string]any)
	}
	mat.Properties[key] = val
}

func parseMTLPropFloat(mat *ir.Material, fs *fieldSplitter, key string) {
	if mat == nil || fs.count < minFields {
		return
	}
	v, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return
	}
	setProp(mat, key, v)
}

func parseMTLPropColor(mat *ir.Material, fs *fieldSplitter, key string) {
	if mat == nil || fs.count < minColorFields {
		return
	}
	col, err := parseFloat3Bytes(fs.get(1), fs.get(2), fs.get(3)) //nolint:mnd // color format
	if err != nil {
		return
	}
	setProp(mat, key, col)
}
