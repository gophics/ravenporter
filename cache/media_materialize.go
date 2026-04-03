package cache

import "github.com/gophics/ravenporter/ir"

func materializeAssetMedia(asset *ir.Asset) error {
	if asset == nil {
		return nil
	}
	for _, clip := range asset.AudioClips {
		if clip == nil {
			continue
		}
		if _, err := clip.CompressedBytes(); err != nil {
			return err
		}
	}
	for _, font := range asset.Fonts {
		if font == nil || font.Vector == nil {
			continue
		}
		if _, err := font.Vector.RawBytes(); err != nil {
			return err
		}
	}
	for _, image := range asset.Images {
		if image == nil {
			continue
		}
		if _, err := image.CompressedBytes(); err != nil {
			return err
		}
	}
	return nil
}
