package audio_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

func TestDecodeSamplesAudio(t *testing.T) {
	t.Run("materializes lazy samples once", func(t *testing.T) {
		decodeCalls := 0
		clip := &ir.AudioClip{
			Compressed: []byte{1},
			SampleDecode: func(_ *ir.AudioClip) ([]float32, error) {
				decodeCalls++
				return []float32{0.25, -0.5}, nil
			},
		}

		asset := &ir.Asset{AudioClips: []*ir.AudioClip{clip}}
		require.NoError(t, process.Apply(asset, process.PPDecodeSamples, process.Options{}))
		assert.Equal(t, 1, decodeCalls)

		samples, err := asset.AudioClips[0].DecodeSamples()
		require.NoError(t, err)
		assert.Equal(t, []float32{0.25, -0.5}, samples)
		assert.Equal(t, 1, decodeCalls, "samples should be cached after PPDecodeSamples")
	})

	t.Run("propagates decode failures", func(t *testing.T) {
		wantErr := errors.New("decode failed")
		asset := &ir.Asset{AudioClips: []*ir.AudioClip{{
			Compressed: []byte{1},
			SampleDecode: func(_ *ir.AudioClip) ([]float32, error) {
				return nil, wantErr
			},
		}}}

		err := process.Apply(asset, process.PPDecodeSamples, process.Options{})
		require.ErrorIs(t, err, wantErr)
	})
}
