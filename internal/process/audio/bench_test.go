package audio_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

func createClipWithSamples(samples []float32, sr int) *ir.AudioClip {
	c := &ir.AudioClip{
		SampleRate:   sr,
		SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return samples, nil },
	}
	return c
}

func BenchmarkResampleAudio(b *testing.B) {
	asset := &ir.Asset{
		AudioClips: []*ir.AudioClip{
			createClipWithSamples(make([]float32, 44100*2), 44100), // 1 second of stereo audio
		},
	}
	opts := process.Options{TargetSampleRate: 48000}

	b.ReportAllocs()
	for b.Loop() {
		// Clone buffer to avoid caching skips if the pipeline checks data length
		clone := &ir.Asset{
			AudioClips: []*ir.AudioClip{
				createClipWithSamples(make([]float32, 44100*2), asset.AudioClips[0].SampleRate),
			},
		}

		_ = process.Apply(clone, process.PPResampleAudio, opts)
	}
}

func BenchmarkMixdownAudio(b *testing.B) {
	asset := &ir.Asset{
		AudioClips: []*ir.AudioClip{
			createClipWithSamples(make([]float32, 44100*4), 44100), // 1 second of quad audio
		},
	}
	opts := process.Options{TargetChannels: 2}

	b.ReportAllocs()
	for b.Loop() {
		clone := &ir.Asset{
			AudioClips: []*ir.AudioClip{
				createClipWithSamples(make([]float32, 44100*4), asset.AudioClips[0].SampleRate),
			},
		}
		_ = process.Apply(clone, process.PPMixdownAudio, opts)
	}
}
