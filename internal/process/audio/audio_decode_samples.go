package audio

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type decodeSamplesStep struct{}

func (s *decodeSamplesStep) Name() string      { return "DecodeSamples" }
func (s *decodeSamplesStep) Flag() core.PPFlag { return core.PPDecodeSamples }

func (s *decodeSamplesStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for _, clip := range asset.AudioClips {
		if clip == nil {
			continue
		}
		if _, err := clip.DecodeSamples(); err != nil {
			return nil, err
		}
	}
	return asset, nil
}
