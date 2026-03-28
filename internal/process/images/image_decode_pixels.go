package images

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type decodePixelsStep struct{}

func (s *decodePixelsStep) Name() string      { return "DecodePixels" }
func (s *decodePixelsStep) Flag() core.PPFlag { return core.PPDecodePixels }

func (s *decodePixelsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Images {
		img := asset.Images[i]
		if img == nil || img.IsGPUCompressed() || img.PixelDecode == nil {
			continue
		}
		if _, err := img.DecodePixels(); err != nil {
			return nil, err
		}
	}
	return asset, nil
}
