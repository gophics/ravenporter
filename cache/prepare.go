package cache

import "github.com/gophics/ravenporter/ir"

func prepareAssetForWrite(asset *ir.Asset, cfg writeConfig) (*ir.Asset, error) {
	if asset == nil {
		return nil, nil
	}

	prepared := cloneAssetForWrite(asset)
	prepared.NormalizeGraph()

	if len(asset.Images) != 0 {
		prepared.Images = make([]*ir.ImageAsset, len(asset.Images))
		for i, image := range asset.Images {
			clonedImage, err := cloneImageAssetForWrite(image, cfg)
			if err != nil {
				return nil, err
			}
			prepared.Images[i] = clonedImage
		}
	}
	return prepared, nil
}

func cloneAssetForWrite(asset *ir.Asset) *ir.Asset {
	cloned := *asset
	cloned.RootNodes = cloneInts(asset.RootNodes)
	cloned.Nodes = append([]ir.Node(nil), asset.Nodes...)
	for i := range cloned.Nodes {
		cloned.Nodes[i].Children = cloneInts(asset.Nodes[i].Children)
		if len(asset.Nodes[i].MorphWeights) != 0 {
			cloned.Nodes[i].MorphWeights = append([]float32(nil), asset.Nodes[i].MorphWeights...)
		}
	}
	if len(asset.Scenes) != 0 {
		cloned.Scenes = make([]*ir.Scene, len(asset.Scenes))
		for i, scene := range asset.Scenes {
			if scene == nil {
				continue
			}
			sceneClone := *scene
			sceneClone.RootNodes = cloneInts(scene.RootNodes)
			cloned.Scenes[i] = &sceneClone
		}
	}
	return &cloned
}

func cloneInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]int, len(values))
	copy(cloned, values)
	return cloned
}

func cloneImageAssetForWrite(image *ir.ImageAsset, cfg writeConfig) (*ir.ImageAsset, error) {
	if image == nil {
		return nil, nil
	}

	compressed, err := image.CompressedBytes()
	if err != nil {
		return nil, err
	}

	cloned := &ir.ImageAsset{
		Name:              image.Name,
		Format:            image.Format,
		Width:             image.Width,
		Height:            image.Height,
		Topology:          image.Topology,
		Depth:             image.Depth,
		Layers:            image.Layers,
		Channels:          image.Channels,
		ColorSpace:        image.ColorSpace,
		MipLevels:         image.MipLevels,
		Compressed:        cloneBytes(compressed),
		SourceFormat:      image.SourceFormat,
		SourcePath:        image.SourcePath,
		CompressionFormat: image.CompressionFormat,
		Metadata:          cloneStringMap(image.Metadata),
		PixelDecode:       image.PixelDecode,
	}
	cloned.NormalizeTopology()

	if cloned.SourcePath != "" && isDataURI(cloned.SourcePath) {
		data, err := decodeDataURI(cloned.SourcePath)
		if err != nil {
			return nil, err
		}
		if len(cloned.Compressed) == 0 {
			cloned.SetCompressedBytes(data)
		}
		cloned.SourcePath = ""
	}

	pixels, err := selectImagePixelsForWrite(image, cfg, cloned.HasCompressedBytes())
	if err != nil {
		return nil, err
	}
	if pixels != nil {
		cloned.SetPixels(pixels)
	}

	return cloned, nil
}

func clonePixelBuffer(pixels *ir.PixelBuffer) *ir.PixelBuffer {
	if pixels == nil {
		return nil
	}

	cloned := &ir.PixelBuffer{
		Data:     cloneBytes(pixels.Data),
		DataType: pixels.DataType,
		BitDepth: pixels.BitDepth,
	}
	if len(pixels.Mipmaps) != 0 {
		cloned.Mipmaps = make([][]byte, len(pixels.Mipmaps))
		for i, mip := range pixels.Mipmaps {
			cloned.Mipmaps[i] = cloneBytes(mip)
		}
	}
	return cloned
}

func selectImagePixelsForWrite(
	image *ir.ImageAsset,
	cfg writeConfig,
	hasCompressed bool,
) (*ir.PixelBuffer, error) {
	if image == nil {
		return nil, nil
	}
	if pixels := image.Pixels(); pixels != nil {
		if cfg.imagePixels == ImagePixelsAlways || cfg.imagePixels == ImagePixelsIfPresent || !hasCompressed {
			return clonePixelBuffer(pixels), nil
		}
		return nil, nil
	}
	if image.PixelDecode == nil || image.IsGPUCompressed() {
		return nil, nil
	}
	if cfg.imagePixels != ImagePixelsAlways && hasCompressed {
		return nil, nil
	}
	pixels, err := image.DecodePixels()
	if err != nil {
		return nil, err
	}
	if pixels == nil {
		return nil, nil
	}
	return clonePixelBuffer(pixels), nil
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
