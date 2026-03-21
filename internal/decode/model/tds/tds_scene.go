package tds

import (
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

func parseMaterial(data []byte, ctx *parseCtx) {
	mat := &ir.Material{
		BaseColorFactor: [4]float32{defaultColorVal, defaultColorVal, defaultColorVal, 1.0},
		RoughnessFactor: defaultRough,
	}
	var texPath string

	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			break
		}
		body := data[chunkHdrSize:size]

		switch id {
		case chunkMatName:
			mat.Name = binread.CString(body)
		case chunkMatAmbient:
			if mat.Properties == nil {
				mat.Properties = make(map[string]any)
			}
			mat.Properties[propAmbient] = readColorFactor(body)
		case chunkMatDiffuse:
			mat.BaseColorFactor = readColorFactor(body)
		case chunkMatSpecular:
			r, g, b := readColor24(body)
			luminance := (lumaR*float32(r) + lumaG*float32(g) + lumaB*float32(b)) / colorScale
			mat.MetallicFactor = luminance
		case chunkMatTexMap:
			texPath = parseTexMap(body)
		case chunkMatBumpMap:
			bumpPath := parseTexMap(body)
			if bumpPath != "" {
				bumpIdx := addTextureAsset(ctx.asset, bumpPath)
				mat.NormalTexture = &ir.TextureRef{TextureIndex: bumpIdx}
				mat.NormalScale = 1.0
			}
		case chunkMatSpecMap:
			addTdsTexProp(ctx, mat, body, "specularTexture")
		case chunkMatOpacMap:
			if path := parseTexMap(body); path != "" {
				addTdsTexProp(ctx, mat, body, "opacityTexture")
				mat.AlphaMode = ir.AlphaBlend
			}
		case chunkMatReflMap:
			addTdsTexProp(ctx, mat, body, "reflectionTexture")
		case chunkMatEmisMap:
			if path := parseTexMap(body); path != "" {
				texIdx := addTextureAsset(ctx.asset, path)
				mat.EmissiveTexture = &ir.TextureRef{TextureIndex: texIdx}
			}
		}

		data = data[size:]
	}

	if mat.Name == "" {
		mat.Name = defaultMatName
	}

	idx := len(ctx.asset.Materials)
	ctx.matNames[mat.Name] = idx

	if texPath != "" {
		texIdx := addTextureAsset(ctx.asset, texPath)
		mat.BaseColorTexture = &ir.TextureRef{TextureIndex: texIdx}
	}

	ctx.asset.Materials = append(ctx.asset.Materials, mat)
}

func parseTexMap(data []byte) string {
	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			break
		}
		if id == chunkMatTexFile {
			return binread.CString(data[chunkHdrSize:size])
		}
		data = data[size:]
	}
	return ""
}

func addTdsTexProp(ctx *parseCtx, mat *ir.Material, body []byte, key string) {
	path := parseTexMap(body)
	if path == "" {
		return
	}
	texIdx := addTextureAsset(ctx.asset, path)
	if mat.Properties == nil {
		mat.Properties = make(map[string]any)
	}
	mat.Properties[key] = texIdx
}

func addTextureAsset(asset *ir.Asset, path string) int {
	imageIndex := len(asset.Images)
	asset.Images = append(asset.Images, &ir.ImageAsset{
		Name:       path,
		SourcePath: path,
	})
	textureIndex := len(asset.Textures)
	asset.Textures = append(asset.Textures, &ir.Texture{
		Name:       path,
		ImageIndex: imageIndex,
	})
	return textureIndex
}

func parseLight(name string, body []byte, ctx *parseCtx) {
	if len(body) < lightDataSize {
		return
	}
	pos := [3]float32{
		binread.ReadF32LE(body[0:]),
		binread.ReadF32LE(body[u32Size:]),
		binread.ReadF32LE(body[u32Size*2:]),
	}

	light := &ir.Light{
		Name:      name,
		Color:     [3]float32{1, 1, 1},
		Intensity: 1.0,
		Point:     &ir.PointLight{},
	}

	sub := body[lightDataSize:]
	for len(sub) >= chunkHdrSize {
		id := binread.ReadU16LE(sub[:2])
		size := binread.ClampChunkSize(len(sub), binread.ReadU32LE(sub[2:6]))
		if size < chunkHdrSize {
			break
		}
		sbody := sub[chunkHdrSize:size]
		if id == chunkSpotlight && len(sbody) >= spotDataSize {
			hotspot := binread.ReadF32LE(sbody[lightDataSize:])
			light.Point = nil
			light.Spot = &ir.SpotLight{
				InnerConeAngle: hotspot * degreesToRad,
				OuterConeAngle: hotspot * degreesToRad,
			}
		}
		if id == chunkDirectional {
			light.Point = nil
			light.Directional = &ir.DirectionalLight{}
		}
		if id == chunkColor24 && len(sbody) >= colorBytes {
			light.Color = [3]float32{
				float32(sbody[0]) / colorScale,
				float32(sbody[1]) / colorScale,
				float32(sbody[2]) / colorScale,
			}
		}
		sub = sub[size:]
	}

	lightIdx := len(ctx.asset.Lights)
	ctx.asset.Lights = append(ctx.asset.Lights, light)
	addNode(ctx, name, ir.NoIndex, ir.NoIndex, lightIdx, ir.Transform{Translation: pos})
}

func parseCameraChunk(name string, body []byte, ctx *parseCtx) {
	if len(body) < cameraDataSize {
		return
	}
	pos := [3]float32{
		binread.ReadF32LE(body[0:]),
		binread.ReadF32LE(body[u32Size:]),
		binread.ReadF32LE(body[u32Size*2:]),
	}
	fov := binread.ReadF32LE(body[fovOffset:])

	cam := &ir.Camera{
		Name: name,
		Perspective: &ir.PerspectiveCamera{
			FOV:  fov * degreesToRad,
			Near: defaultNear,
			Far:  defaultFar,
		},
	}

	camIdx := len(ctx.asset.Cameras)
	ctx.asset.Cameras = append(ctx.asset.Cameras, cam)
	addNode(ctx, name, ir.NoIndex, camIdx, ir.NoIndex, ir.Transform{Translation: pos})
}

func addNode(ctx *parseCtx, name string, meshIdx, camIdx, lightIdx int, t ir.Transform) {
	node := ir.Node{LODGroupIndex: ir.NoIndex,
		Name:        name,
		MeshIndex:   meshIdx,
		SkinIndex:   ir.NoIndex,
		CameraIndex: camIdx,
		LightIndex:  lightIdx,
		Transform:   t,
	}
	idx := len(ctx.asset.Nodes)
	ctx.asset.Nodes = append(ctx.asset.Nodes, node)
	ctx.asset.RootNodes = append(ctx.asset.RootNodes, idx)
}

func readColor24(data []byte) (r, g, b byte) {
	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			break
		}
		body := data[chunkHdrSize:size]
		if id == chunkColor24 && len(body) >= colorBytes {
			return body[0], body[1], body[2]
		}
		data = data[size:]
	}
	return defaultGray, defaultGray, defaultGray
}

func readColorFactor(data []byte) [4]float32 {
	r, g, b := readColor24(data)
	return decutil.ColorToFactor(r, g, b)
}
