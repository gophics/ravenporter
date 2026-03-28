package audio

import (
	"math"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

const (
	silenceThreshold = 0.001
)

type normalizeAudioStep struct{}

func (s *normalizeAudioStep) Name() string      { return "NormalizeAudio" }
func (s *normalizeAudioStep) Flag() core.PPFlag { return core.PPNormalizeAudio }

func (s *normalizeAudioStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.AudioClips {
		a := asset.AudioClips[i]
		if a == nil {
			continue
		}
		samples, err := a.DecodeSamples()
		if err != nil {
			return nil, err
		}
		if len(samples) == 0 {
			continue
		}

		var maxAmp float32
		for _, amp := range samples {
			if amp < 0 {
				amp = -amp
			}
			if amp > maxAmp {
				maxAmp = amp
			}
		}

		if maxAmp == 0 || maxAmp == 1.0 {
			continue
		}

		scale := 1.0 / maxAmp
		for idx := range samples {
			samples[idx] *= scale
		}
		asset.AudioClips[i] = &ir.AudioClip{
			Name:         a.Name,
			Format:       a.Format,
			SampleRate:   a.SampleRate,
			Layout:       a.Layout,
			BitDepth:     a.BitDepth,
			Duration:     a.Duration,
			LoopStart:    a.LoopStart,
			LoopEnd:      a.LoopEnd,
			Metadata:     a.Metadata,
			SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return samples, nil },
		}
	}
	return asset, nil
}

type trimAudioStep struct{}

func (s *trimAudioStep) Name() string      { return "TrimAudio" }
func (s *trimAudioStep) Flag() core.PPFlag { return core.PPTrimAudio }

func (s *trimAudioStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.AudioClips {
		a := asset.AudioClips[i]
		if a == nil {
			continue
		}
		samples, err := a.DecodeSamples()
		if err != nil {
			return nil, err
		}
		if len(samples) == 0 {
			continue
		}

		var chCount int
		switch a.Layout {
		case ir.LayoutMono:
			chCount = 1
		case ir.LayoutStereo:
			chCount = 2
		default:
			chCount = 1
		}

		frames := len(samples) / chCount
		if frames == 0 {
			continue
		}

		startFrame := 0
		for startFrame < frames {
			isSilent := true
			base := startFrame * chCount
			for c := 0; c < chCount; c++ {
				if math.Abs(float64(samples[base+c])) >= silenceThreshold {
					isSilent = false
					break
				}
			}
			if !isSilent {
				break
			}
			startFrame++
		}

		endFrame := frames
		for endFrame > startFrame {
			isSilent := true
			base := (endFrame - 1) * chCount
			for c := 0; c < chCount; c++ {
				if math.Abs(float64(samples[base+c])) >= silenceThreshold {
					isSilent = false
					break
				}
			}
			if !isSilent {
				break
			}
			endFrame--
		}

		var outSamples []float32
		if startFrame < endFrame {
			outSamples = samples[startFrame*chCount : endFrame*chCount]
		}
		asset.AudioClips[i] = &ir.AudioClip{
			Name:         a.Name,
			Format:       a.Format,
			SampleRate:   a.SampleRate,
			Layout:       a.Layout,
			BitDepth:     a.BitDepth,
			Duration:     a.Duration,
			LoopStart:    a.LoopStart,
			LoopEnd:      a.LoopEnd,
			Metadata:     a.Metadata,
			SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return outSamples, nil },
		}
	}
	return asset, nil
}
