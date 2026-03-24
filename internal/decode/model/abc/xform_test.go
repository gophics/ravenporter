package abc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
)

func identityMat() [16]float32 {
	return [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

func translationMat(x, y, z float32) [16]float32 {
	return [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, x, y, z, 1}
}

func TestBuildXformAnimation(t *testing.T) {
	tests := []struct {
		name        string
		matrices    [][16]float32
		wantChans   int
		wantSamples int
		checkFirst  func(t *testing.T, anim *ir.Animation)
	}{
		{
			"TwoIdentitySamples",
			[][16]float32{identityMat(), identityMat()},
			3, 2,
			func(t *testing.T, anim *ir.Animation) {
				assert.Equal(t, [3]float32{0, 0, 0}, anim.Channels[0].Translations[0])
				assert.Equal(t, [3]float32{1, 1, 1}, anim.Channels[2].Scales[0])
			},
		},
		{
			"TranslationAnimation",
			[][16]float32{translationMat(0, 0, 0), translationMat(10, 0, 0), translationMat(20, 0, 0)},
			3, 3,
			func(t *testing.T, anim *ir.Animation) {
				assert.InDelta(t, float32(0), anim.Channels[0].Translations[0][0], 0.01)
				assert.InDelta(t, float32(10), anim.Channels[0].Translations[1][0], 0.01)
				assert.InDelta(t, float32(20), anim.Channels[0].Translations[2][0], 0.01)
				assert.InDelta(t, float32(0), anim.Channels[0].Times[0], 0.01)
				assert.InDelta(t, 1.0/abcDefaultFPS, float64(anim.Channels[0].Times[1]), 0.001)
			},
		},
		{
			"SingleSample",
			[][16]float32{identityMat()},
			3, 1,
			func(t *testing.T, anim *ir.Animation) {
				assert.InDelta(t, 0.0, anim.Duration, 0.001)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := buildXformAnimation(0, tt.matrices, 0, 1.0/abcDefaultFPS)
			require.NotNil(t, anim)
			assert.Equal(t, xformAnimName, anim.Name)
			assert.Len(t, anim.Channels, tt.wantChans)

			for _, ch := range anim.Channels {
				assert.Len(t, ch.Times, tt.wantSamples)
				assert.Equal(t, 0, ch.NodeIndex)
				assert.Equal(t, ir.InterpolationLinear, ch.Interpolation)
			}

			assert.Equal(t, ir.TargetTranslation, anim.Channels[0].Target)
			assert.Equal(t, ir.TargetRotation, anim.Channels[1].Target)
			assert.Equal(t, ir.TargetScale, anim.Channels[2].Target)

			tt.checkFirst(t, anim)
		})
	}
}
