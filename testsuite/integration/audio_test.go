//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/testsuite/corpus"
)

func TestIntegration_Audio(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectedFmt  ir.AudioFormat
		sourceFormat ir.FormatID
		hasSamples   bool // false for Opus (streams via pion/opus, no pre-decoded PCM)
		verifyFn     func(t *testing.T, clip *ir.AudioClip)
	}{
		{"WAV", corpus.AudioWAV, ir.AudioWAV, ir.FormatWAV, true, func(t *testing.T, c *ir.AudioClip) {
			assert.Equal(t, ir.LayoutMono, c.Layout)
			assert.Equal(t, ir.BitDepth16, c.BitDepth)
			assert.Equal(t, 44100, c.SampleRate)
		}},
		{"OGG", corpus.AudioOGG, ir.AudioOGG, ir.FormatOGG, true, func(t *testing.T, c *ir.AudioClip) {
			assert.Equal(t, ir.LayoutStereo, c.Layout)
			assert.Equal(t, ir.BitDepth16, c.BitDepth)
		}},
		{"MP3", corpus.AudioMP3, ir.AudioMP3, ir.FormatMP3, true, func(t *testing.T, c *ir.AudioClip) {
			assert.NotZero(t, c.BitDepth)
		}},
		{"FLAC", corpus.AudioFLAC, ir.AudioFLAC, ir.FormatFLAC, true, func(t *testing.T, c *ir.AudioClip) {
			assert.NotZero(t, c.BitDepth)
		}},
		{"Opus", corpus.AudioOpus, ir.AudioOpus, ir.FormatOpus, false, func(t *testing.T, c *ir.AudioClip) {
			assert.Equal(t, 48000, c.SampleRate)
			assert.Equal(t, ir.LayoutStereo, c.Layout)
			assert.Equal(t, ir.BitDepth16, c.BitDepth)
		}},
		{"AIFF", corpus.AudioAIFF, ir.AudioAIFF, ir.FormatAIFF, true, func(t *testing.T, c *ir.AudioClip) {
			assert.NotZero(t, c.BitDepth)
		}},

		// Variants
		{"FLAC_Small", corpus.AudioFLACSmall, ir.AudioFLAC, ir.FormatFLAC, true, nil},
		{"MP3_Small", corpus.AudioMP3Small, ir.AudioMP3, ir.FormatMP3, true, nil},
		{"OGG_Small", corpus.AudioOGGSmall, ir.AudioOGG, ir.FormatOGG, true, nil},
		{"WAV_Small", corpus.AudioWAVSmall, ir.AudioWAV, ir.FormatWAV, true, nil},
		{"AIFF_8bit", corpus.AudioAIFF8bit, ir.AudioAIFF, ir.FormatAIFF, true, func(t *testing.T, c *ir.AudioClip) {
			assert.NotZero(t, c.BitDepth)
		}},
		{"AIFF_24bit", corpus.AudioAIFF24bit, ir.AudioAIFF, ir.FormatAIFF, true, func(t *testing.T, c *ir.AudioClip) {
			assert.NotZero(t, c.BitDepth)
		}},

		// Exhaustive Isolation
		{"WAV_Isolation", corpus.IsoAudioFeaturesWAV, ir.AudioWAV, ir.FormatWAV, true, func(t *testing.T, c *ir.AudioClip) {
			assert.Equal(t, 44100, c.SampleRate)
			assert.Equal(t, ir.LayoutStereo, c.Layout)
			assert.Equal(t, ir.BitDepth16, c.BitDepth)
			assert.Equal(t, 10, c.LoopStart)
			assert.Equal(t, 40, c.LoopEnd)
			assert.Equal(t, "Test", c.Metadata.Title)
		}},
		{"FLAC_Isolation", corpus.IsoAudioFeaturesFLAC, ir.AudioFLAC, ir.FormatFLAC, false, func(t *testing.T, c *ir.AudioClip) {
			assert.Equal(t, "FlacTest", c.Metadata.Title)
			assert.Equal(t, "Isolator", c.Metadata.Artist)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := runPipeline(t, tc.path)
			require.Len(t, asset.AudioClips, 1, "expected exactly 1 audio clip")
			clip := asset.AudioClips[0]

			assert.Equal(t, tc.sourceFormat, asset.Metadata.SourceFormat)

			// Shared AudioClip invariants.
			assert.NotEmpty(t, clip.Name)
			assert.Equal(t, tc.expectedFmt, clip.Format)
			assert.True(t, clip.SampleRate > 0, "sample rate must be > 0")
			assert.True(t, clip.Duration > 0, "duration must be > 0")
			assert.NotZero(t, clip.Layout, "channel layout must be set")

			// Loop points must be explicitly set (NoIndex for non-looping files).
			assert.True(t, clip.LoopStart == ir.NoIndex || clip.LoopStart >= 0, "LoopStart must be NoIndex or valid")
			assert.True(t, clip.LoopEnd == ir.NoIndex || clip.LoopEnd >= 0, "LoopEnd must be NoIndex or valid")

			if tc.hasSamples {
				samples, sErr := clip.DecodeSamples()
				require.NoError(t, sErr)
				assert.True(t, len(samples) > 0, "PCM samples must be decoded")
			}

			// AudioMetadata: verify the struct is wired through the pipeline.
			// If the corpus file has tags, assert they arrived; otherwise assert zero-value.
			m := clip.Metadata
			if m.Title != "" {
				assert.NotEmpty(t, m.Title, "metadata Title must not be empty when populated")
			}
			if m.Artist != "" {
				assert.NotEmpty(t, m.Artist, "metadata Artist must not be empty when populated")
			}
			t.Logf("%s: metadata={title=%q artist=%q album=%q genre=%q comment=%q artwork=%d cuepoints=%d}",
				tc.name, m.Title, m.Artist, m.Album, m.Genre, m.Comment, len(m.Artwork), len(m.CuePoints))

			if tc.verifyFn != nil {
				tc.verifyFn(t, clip)
			}
			logSamples, _ := clip.DecodeSamples()
			t.Logf("%s: %dHz layout=%v depth=%v dur=%v samples=%d loop=[%d,%d]",
				tc.name, clip.SampleRate, clip.Layout, clip.BitDepth, clip.Duration, len(logSamples), clip.LoopStart, clip.LoopEnd)
		})
	}
}
