package audio_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createClipNormalize(samples []float32, layout ir.ChannelLayout) *ir.AudioClip {
	c := &ir.AudioClip{
		Layout:       layout,
		SampleDecode: func(_ *ir.AudioClip) ([]float32, error) { return samples, nil },
	}
	return c
}

func TestNormalizeAudio(t *testing.T) {
	tests := []struct {
		name     string
		samples  []float32
		wantPeak float64
	}{
		{name: "scales_to_unity", samples: []float32{0.5, -0.25, 0.1}, wantPeak: 1.0},
		{name: "already_normalized", samples: []float32{1.0, -0.5, 0.3}, wantPeak: 1.0},
		{name: "negative_peak", samples: []float32{-0.8, 0.2}, wantPeak: 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{AudioClips: []*ir.AudioClip{createClipNormalize(tt.samples, ir.LayoutMono)}}
			require.NoError(t, process.Apply(asset, process.PPNormalizeAudio, process.Options{}))
			var maxAmp float32
			s, _ := asset.AudioClips[0].DecodeSamples()
			for _, s := range s {
				if s < 0 {
					s = -s
				}
				if s > maxAmp {
					maxAmp = s
				}
			}
			assert.InDelta(t, tt.wantPeak, float64(maxAmp), 0.01)
		})
	}

	t.Run("nil_clip", func(t *testing.T) {
		asset := &ir.Asset{AudioClips: []*ir.AudioClip{nil}}
		require.NoError(t, process.Apply(asset, process.PPNormalizeAudio, process.Options{}))
	})

	t.Run("empty_samples", func(t *testing.T) {
		asset := &ir.Asset{AudioClips: []*ir.AudioClip{createClipNormalize([]float32{}, ir.LayoutMono)}}
		require.NoError(t, process.Apply(asset, process.PPNormalizeAudio, process.Options{}))
	})

	t.Run("all_zero", func(t *testing.T) {
		asset := &ir.Asset{AudioClips: []*ir.AudioClip{createClipNormalize([]float32{0, 0, 0}, ir.LayoutMono)}}
		require.NoError(t, process.Apply(asset, process.PPNormalizeAudio, process.Options{}))
		s, _ := asset.AudioClips[0].DecodeSamples()
		assert.Equal(t, float32(0), s[0])
	})
}

func TestTrimAudio(t *testing.T) {
	tests := []struct {
		name    string
		layout  ir.ChannelLayout
		samples []float32
		wantLen int
	}{
		{
			name:    "trim_leading_trailing_mono",
			layout:  ir.LayoutMono,
			samples: []float32{0, 0, 0.5, 0.8, 0.3, 0, 0},
			wantLen: 3,
		},
		{
			name:    "trim_stereo",
			layout:  ir.LayoutStereo,
			samples: []float32{0, 0, 0, 0, 0.5, 0.6, 0.3, 0.4, 0, 0},
			wantLen: 4,
		},
		{
			name:    "all_silence",
			layout:  ir.LayoutMono,
			samples: []float32{0, 0, 0},
			wantLen: 0,
		},
		{
			name:    "no_trim_needed",
			layout:  ir.LayoutMono,
			samples: []float32{0.5, 0.8, 0.3},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{AudioClips: []*ir.AudioClip{createClipNormalize(tt.samples, tt.layout)}}
			require.NoError(t, process.Apply(asset, process.PPTrimAudio, process.Options{}))
			s, _ := asset.AudioClips[0].DecodeSamples()
			if tt.wantLen == 0 {
				assert.Len(t, s, 0)
			} else {
				assert.Len(t, s, tt.wantLen)
			}
		})
	}

	t.Run("nil_clip", func(t *testing.T) {
		asset := &ir.Asset{AudioClips: []*ir.AudioClip{nil}}
		require.NoError(t, process.Apply(asset, process.PPTrimAudio, process.Options{}))
	})
}
