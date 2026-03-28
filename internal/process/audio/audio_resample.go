package audio

import (
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type mixdownAudioStep struct{}

func (s *mixdownAudioStep) Name() string      { return "MixdownAudio" }
func (s *mixdownAudioStep) Flag() core.PPFlag { return core.PPMixdownAudio }

func (s *mixdownAudioStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	if opts.TargetChannels <= 0 {
		opts.TargetChannels = 1
	}

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

		var currentCh int
		switch a.Layout {
		case ir.LayoutMono:
			currentCh = 1
		case ir.LayoutStereo:
			currentCh = 2
		default:
			continue
		}

		if currentCh == opts.TargetChannels {
			continue
		}

		frames := len(samples) / currentCh

		if opts.TargetChannels == 1 && currentCh > 1 { // Stereo to Mono mixdown
			mono := make([]float32, frames)
			for f := 0; f < frames; f++ {
				base := f * currentCh
				var sum float32
				for c := 0; c < currentCh; c++ {
					sum += samples[base+c]
				}
				mono[f] = sum / float32(currentCh)
			}
			asset.AudioClips[i] = &ir.AudioClip{
				Name:         a.Name,
				Format:       a.Format,
				SampleRate:   a.SampleRate,
				Layout:       ir.LayoutMono,
				BitDepth:     a.BitDepth,
				Duration:     a.Duration,
				LoopStart:    a.LoopStart,
				LoopEnd:      a.LoopEnd,
				Metadata:     a.Metadata,
				SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return mono, nil },
			}
		} else if opts.TargetChannels == 2 && currentCh == 1 { // Mono to Stereo upmix
			const targetStereo = 2
			stereo := make([]float32, frames*targetStereo)
			for f := 0; f < frames; f++ {
				val := samples[f]
				stereo[f*2] = val
				stereo[f*2+1] = val
			}
			asset.AudioClips[i] = &ir.AudioClip{
				Name:         a.Name,
				Format:       a.Format,
				SampleRate:   a.SampleRate,
				Layout:       ir.LayoutStereo,
				BitDepth:     a.BitDepth,
				Duration:     a.Duration,
				LoopStart:    a.LoopStart,
				LoopEnd:      a.LoopEnd,
				Metadata:     a.Metadata,
				SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return stereo, nil },
			}
		}
	}
	return asset, nil
}

type resampleAudioStep struct{}

func (s *resampleAudioStep) Name() string      { return "ResampleAudio" }
func (s *resampleAudioStep) Flag() core.PPFlag { return core.PPResampleAudio }

func (s *resampleAudioStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	if opts.TargetSampleRate <= 0 {
		return asset, nil
	}
	targetRate := float64(opts.TargetSampleRate)

	for i := range asset.AudioClips {
		a := asset.AudioClips[i]
		if a == nil || a.SampleRate == 0 {
			continue
		}
		samples, err := a.DecodeSamples()
		if err != nil {
			return nil, err
		}
		if len(samples) == 0 {
			continue
		}

		srcRate := float64(a.SampleRate)
		if srcRate == targetRate {
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

		ratio := srcRate / targetRate
		srcFrames := len(samples) / chCount
		dstFrames := int(float64(srcFrames) / ratio)
		duration := decutil.AudioDuration(dstFrames, opts.TargetSampleRate)

		dst := make([]float32, dstFrames*chCount)

		for j := 0; j < dstFrames; j++ {
			srcIdx := float64(j) * ratio
			idx0 := int(srcIdx)
			idx1 := idx0 + 1
			if idx1 >= srcFrames {
				idx1 = srcFrames - 1
			}
			frac := float32(srcIdx - float64(idx0))

			for c := 0; c < chCount; c++ {
				val0 := samples[idx0*chCount+c]
				val1 := samples[idx1*chCount+c]
				dst[j*chCount+c] = val0 + frac*(val1-val0)
			}
		}
		asset.AudioClips[i] = &ir.AudioClip{
			Name:         a.Name,
			Format:       a.Format,
			SampleRate:   opts.TargetSampleRate,
			Layout:       a.Layout,
			BitDepth:     a.BitDepth,
			Duration:     duration,
			LoopStart:    a.LoopStart,
			LoopEnd:      a.LoopEnd,
			Metadata:     a.Metadata,
			SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return dst, nil },
		}
	}

	return asset, nil
}
