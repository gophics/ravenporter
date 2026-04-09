package threemf

import (
	"image/color"

	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
	"github.com/hpinc/go3mf"
	"github.com/hpinc/go3mf/materials"
)

const (
	colorScale       = 1.0 / 255.0
	vertsPerTri      = 3
	scaleMicrometer  = 0.000001
	scaleMillimeter  = 0.001
	scaleCentimeter  = 0.01
	scaleInch        = 0.0254
	scaleFoot        = 0.3048
	scaleMeter       = 1.0
	defaultMetallic  = 0
	defaultRoughness = 1
	defaultMeshName  = "3mf"
)

var defaultWhite = [4]float32{1, 1, 1, 1}

func convertModel(model *go3mf.Model) *ir.Asset {
	asset, scene := ir.NewAssetWithScene(ir.Format3MF, "")
	asset.UpAxis = ir.YUp
	asset.Unit = unitScale(model.Units)

	matMap := extractMaterials(model, asset)
	colorMap := extractColorGroups(model)
	texMap, texGroupMap := extractTextures(model, asset)

	for _, res := range model.Resources.Objects {
		if res.Mesh == nil {
			continue
		}
		mesh := convertMesh(res, matMap, colorMap, texGroupMap)
		meshIdx := len(asset.Meshes)
		asset.Meshes = append(asset.Meshes, mesh)

		nodeIdx := len(asset.Nodes)
		asset.Nodes = append(asset.Nodes, ir.Node{LODGroupIndex: ir.NoIndex,
			Name:        res.Name,
			MeshIndex:   meshIdx,
			ParentIndex: ir.NoIndex,
			SkinIndex:   ir.NoIndex,
			CameraIndex: ir.NoIndex,
			LightIndex:  ir.NoIndex,
			Transform: ir.Transform{
				Rotation: mathx.IdentityQuat,
				Scale:    mathx.IdentityScale,
			},
		})
		scene.RootNodes = append(scene.RootNodes, nodeIdx)
	}

	if len(texMap) > 0 {
		for _, m := range asset.Materials {
			if m.Properties != nil {
				if texID, ok := m.Properties["texture2DID"]; ok {
					if tid, ok2 := texID.(uint32); ok2 {
						if idx, ok3 := texMap[tid]; ok3 {
							m.BaseColorTexture = &ir.TextureRef{TextureIndex: idx, Tiling: [2]float32{1, 1}}
						}
					}
					delete(m.Properties, "texture2DID")
				}
			}
		}
	}

	return asset
}

func convertMesh( //nolint:funlen // UV + texture support
	obj *go3mf.Object, matMap map[uint32]int, colorMap map[uint32][]color.RGBA,
	texGroups map[uint32]*materials.Texture2DGroup,
) *ir.Mesh {
	m := obj.Mesh
	positions := make([][3]float32, len(m.Vertices.Vertex))
	for i, v := range m.Vertices.Vertex {
		positions[i] = [3]float32(v)
	}

	triCount := len(m.Triangles.Triangle)
	indices := make([]uint32, 0, triCount*vertsPerTri)
	matIdx := ir.NoIndex

	var colors [][4]float32
	var uvs [][2]float32
	hasColors := false
	hasUVs := false

	for ti, tri := range m.Triangles.Triangle {
		indices = append(indices, tri.V1, tri.V2, tri.V3)

		if matIdx == ir.NoIndex && tri.PID != 0 {
			if idx, ok := matMap[tri.PID]; ok {
				matIdx = idx
			}
		}

		pid := tri.PID
		if pid == 0 {
			pid = obj.PID
		}
		if cg, ok := colorMap[pid]; ok && len(cg) > 0 {
			if !hasColors {
				hasColors = true
				colors = make([][4]float32, triCount*vertsPerTri)
				for i := range ti * vertsPerTri {
					colors[i] = defaultWhite
				}
			}
			base := ti * vertsPerTri
			colors[base] = rgbaToFloat(cg, tri.P1)
			colors[base+1] = rgbaToFloat(cg, tri.P2)
			colors[base+2] = rgbaToFloat(cg, tri.P3)
		} else if hasColors {
			base := ti * vertsPerTri
			colors[base] = defaultWhite
			colors[base+1] = defaultWhite
			colors[base+2] = defaultWhite
		}

		if tg, ok := texGroups[pid]; ok && len(tg.Coords) > 0 {
			if !hasUVs {
				hasUVs = true
				uvs = make([][2]float32, triCount*vertsPerTri)
			}
			base := ti * vertsPerTri
			if int(tri.P1) < len(tg.Coords) {
				uvs[base] = [2]float32(tg.Coords[tri.P1])
			}
			if int(tri.P2) < len(tg.Coords) {
				uvs[base+1] = [2]float32(tg.Coords[tri.P2])
			}
			if int(tri.P3) < len(tg.Coords) {
				uvs[base+2] = [2]float32(tg.Coords[tri.P3])
			}
		}
	}

	if matIdx == ir.NoIndex && obj.PID != 0 {
		if idx, ok := matMap[obj.PID]; ok {
			matIdx = idx
		}
	}

	name := obj.Name
	if name == "" {
		name = defaultMeshName
	}

	data := ir.MeshData{
		VertexCount: len(positions),
		Positions:   positions,
		Indices:     indices,
	}
	if hasColors {
		data.Colors0 = colors
	}
	if hasUVs {
		data.TexCoord0 = uvs
	}

	return &ir.Mesh{
		Name: name,
		Primitives: []ir.Primitive{{
			Mode:          ir.Triangles,
			MaterialIndex: matIdx,
			Data:          data,
		}},
	}
}

func extractColorGroups(model *go3mf.Model) map[uint32][]color.RGBA {
	cg := make(map[uint32][]color.RGBA)
	for _, asset := range model.Resources.Assets {
		if g, ok := asset.(*materials.ColorGroup); ok {
			cg[g.ID] = g.Colors
		}
	}
	return cg
}

func rgbaToFloat(colors []color.RGBA, idx uint32) [4]float32 {
	if int(idx) >= len(colors) {
		return defaultWhite
	}
	c := colors[idx]
	return [4]float32{
		float32(c.R) * colorScale,
		float32(c.G) * colorScale,
		float32(c.B) * colorScale,
		float32(c.A) * colorScale,
	}
}

func extractMaterials(model *go3mf.Model, asset *ir.Asset) map[uint32]int {
	matMap := make(map[uint32]int)

	for _, resource := range model.Resources.Assets {
		bm, ok := resource.(*go3mf.BaseMaterials)
		if !ok {
			continue
		}
		for _, base := range bm.Materials {
			idx := len(asset.Materials)
			asset.Materials = append(asset.Materials, colorToMaterial(base.Name, base.Color))
			matMap[bm.ID] = idx
		}
	}

	return matMap
}

func colorToMaterial(name string, c color.RGBA) *ir.Material {
	return &ir.Material{
		Name: name,
		BaseColorFactor: [4]float32{
			float32(c.R) * colorScale,
			float32(c.G) * colorScale,
			float32(c.B) * colorScale,
			float32(c.A) * colorScale,
		},
		MetallicFactor:  defaultMetallic,
		RoughnessFactor: defaultRoughness,
		AlphaMode:       ir.AlphaOpaque,
	}
}

func unitScale(u go3mf.Units) float64 {
	switch u {
	case go3mf.UnitMicrometer:
		return scaleMicrometer
	case go3mf.UnitMillimeter:
		return scaleMillimeter
	case go3mf.UnitCentimeter:
		return scaleCentimeter
	case go3mf.UnitInch:
		return scaleInch
	case go3mf.UnitFoot:
		return scaleFoot
	case go3mf.UnitMeter:
		return scaleMeter
	default:
		return scaleMillimeter
	}
}

func extractTextures(
	model *go3mf.Model, asset *ir.Asset,
) (texMap map[uint32]int, groupMap map[uint32]*materials.Texture2DGroup) {
	texMap = make(map[uint32]int)
	groupMap = make(map[uint32]*materials.Texture2DGroup)

	for _, resource := range model.Resources.Assets {
		switch a := resource.(type) {
		case *materials.Texture2D:
			imgIdx := len(asset.Images)
			asset.Images = append(asset.Images, &ir.ImageAsset{
				Name:       a.Path,
				SourcePath: a.Path,
			})
			idx := len(asset.Textures)
			asset.Textures = append(asset.Textures, &ir.Texture{
				Name:       a.Path,
				ImageIndex: imgIdx,
			})
			texMap[a.ID] = idx
		case *materials.Texture2DGroup:
			groupMap[a.ID] = a
		}
	}
	return texMap, groupMap
}
