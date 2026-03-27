package models

import (
	"github.com/gophics/ravenporter/internal/assetio"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type embedTexturesStep struct{}

func (s *embedTexturesStep) Name() string      { return "EmbedTextures" }
func (s *embedTexturesStep) Flag() core.PPFlag { return core.PPEmbedTextures }

func (s *embedTexturesStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	for i := range asset.Textures {
		tex := asset.Textures[i]
		if tex == nil || tex.ImageIndex < 0 || tex.ImageIndex >= len(asset.Images) {
			continue
		}
		img := asset.Images[tex.ImageIndex]
		if img == nil || img.HasCompressedBytes() || img.SourcePath == "" {
			continue
		}

		result, err := readEmbeddedTexture(img.SourcePath, opts)
		if result == nil && err == nil {
			continue
		}
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.Warn("embed texture: skipping unreadable file",
					"path", img.SourcePath,
					"error", err,
				)
			}
			continue
		}

		img.SetCompressedBytes(result.Data)
		img.SourcePath = ""
		if result.FormatID != ir.FormatUnknown {
			img.SourceFormat = result.FormatID
			if img.Format == "" {
				img.Format = ir.ImageFormat(result.FormatID)
			}
		}
	}
	return asset, nil
}

func readEmbeddedTexture(path string, opts core.Options) (*assetio.ReadResult, error) {
	if opts.AssetFS != nil {
		return assetio.ReadFromFS(path, opts.AssetFS)
	}
	return assetio.ResolveAndRead(path, opts.AssetDir)
}
