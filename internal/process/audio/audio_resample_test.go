package audio_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createClipResample(name string, sr int, layout ir.ChannelLayout, samples []float32) *ir.AudioClip {
	c := &ir.AudioClip{
		Name:         name,
		SampleRate:   sr,
		Layout:       layout,
		SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return samples, nil },
	}
	return c
}

func TestResampleAudio(t *testing.T) {
	samples := make([]float32, 100)
	for i := range samples {
		samples[i] = float32(i)
	}
	asset := &ir.Asset{
		AudioClips: []*ir.AudioClip{
			createClipResample("test", 22050, ir.LayoutMono, samples),
		},
	}

	opts := process.Options{TargetSampleRate: 44100}
	require.NoError(t, process.Apply(asset, process.PPResampleAudio, opts))

	assert.Equal(t, 44100, asset.AudioClips[0].SampleRate)
	s, _ := asset.AudioClips[0].DecodeSamples()
	assert.Greater(t, len(s), 100)
}

func TestMixdownAudio(t *testing.T) {
	asset := &ir.Asset{
		AudioClips: []*ir.AudioClip{
			createClipResample("stereo", 44100, ir.LayoutStereo, []float32{1, 0, 0.5, 0.5, 0, 1}),
		},
	}

	opts := process.Options{TargetChannels: 1}
	require.NoError(t, process.Apply(asset, process.PPMixdownAudio, opts))

	assert.Equal(t, ir.LayoutMono, asset.AudioClips[0].Layout)
	s, _ := asset.AudioClips[0].DecodeSamples()
	assert.Len(t, s, 3) // 6 stereo → 3 mono
}

func TestUpmixAudio(t *testing.T) {
	asset := &ir.Asset{
		AudioClips: []*ir.AudioClip{
			createClipResample("mono", 44100, ir.LayoutMono, []float32{1, 0.5, 0}),
		},
	}

	opts := process.Options{TargetChannels: 2}
	require.NoError(t, process.Apply(asset, process.PPMixdownAudio, opts))

	assert.Equal(t, ir.LayoutStereo, asset.AudioClips[0].Layout)
	s, _ := asset.AudioClips[0].DecodeSamples()
	assert.Len(t, s, 6) // 3 mono → 6 stereo
	assert.Equal(t, float32(1), s[0])
	assert.Equal(t, float32(1), s[1])
}

func TestResampleAudioStereo(t *testing.T) {
	samples := make([]float32, 100) // 50 frames
	for i := range samples {
		samples[i] = float32(i)
	}
	asset := &ir.Asset{
		AudioClips: []*ir.AudioClip{
			createClipResample("stereo", 22050, ir.LayoutStereo, samples),
		},
	}

	opts := process.Options{TargetSampleRate: 44100}
	require.NoError(t, process.Apply(asset, process.PPResampleAudio, opts))

	assert.Equal(t, 44100, asset.AudioClips[0].SampleRate)
	s, _ := asset.AudioClips[0].DecodeSamples()
	assert.Greater(t, len(s), 100)
}
