package dae

import (
	"bytes"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestDecodeMatrixAnimation(t *testing.T) {
	data, err := os.ReadFile("testdata/matrix_anim.dae")
	require.NoError(t, err)

	d := &Decoder{}
	asset, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, ir.FormatDAE, asset.Metadata.SourceFormat)

	// Mesh.
	require.Len(t, asset.Meshes, 1)
	assert.Equal(t, "Cube", asset.Meshes[0].Name)

	// Matrix animation should decompose into 3 channels: T + R + S.
	require.Len(t, asset.Animations, 1)
	anim := asset.Animations[0]
	require.Len(t, anim.Channels, 3, "matrix decomposition: T + R + S")

	// Find channels by target.
	var transCh, rotCh, scaleCh *ir.AnimationChannel
	for i := range anim.Channels {
		switch anim.Channels[i].Target {
		case ir.TargetTranslation:
			transCh = &anim.Channels[i]
		case ir.TargetRotation:
			rotCh = &anim.Channels[i]
		case ir.TargetScale:
			scaleCh = &anim.Channels[i]
		}
	}

	// All 3 channels must exist with 3 keyframes each.
	require.NotNil(t, transCh, "should have translation channel")
	require.NotNil(t, rotCh, "should have rotation channel")
	require.NotNil(t, scaleCh, "should have scale channel")

	require.Len(t, transCh.Times, 3)
	require.Len(t, rotCh.Times, 3)

	// Translation channel: extracted from matrix column 3.
	// t=0: T=(5,0,0), t=0.5: T=(10,3,0), t=1.0: T=(15,6,0)
	require.Len(t, transCh.Translations, 3)
	assertVec3(t, [3]float32{5, 0, 0}, transCh.Translations[0], "t=0 translation")
	assertVec3(t, [3]float32{10, 3, 0}, transCh.Translations[1], "t=0.5 translation")
	assertVec3(t, [3]float32{15, 6, 0}, transCh.Translations[2], "t=1.0 translation")

	// Rotation channel: t=0 identity quat, t=0.5 ~45° Y, t=1.0 identity quat.
	require.Len(t, rotCh.Rotations, 3)
	// t=0: identity quaternion (0,0,0,1).
	assertQuatUnit(t, rotCh.Rotations[0], "t=0 rotation should be unit quaternion")
	// t=0.5: 45° Y rotation - quaternion y-component should be non-zero.
	assertQuatUnit(t, rotCh.Rotations[1], "t=0.5 rotation should be unit quaternion")
	// t=1.0: identity (scale 2× doesn't affect rotation).
	assertQuatUnit(t, rotCh.Rotations[2], "t=1.0 rotation should be unit quaternion")

	// Scale channel: t=0 S=(1,1,1), t=1.0 S=(2,2,2).
	require.Len(t, scaleCh.Translations, 3) // Scale uses Translations field.
	assertVec3(t, [3]float32{1, 1, 1}, scaleCh.Translations[0], "t=0 scale")
	assertVec3(t, [3]float32{2, 2, 2}, scaleCh.Translations[2], "t=1.0 scale")

	t.Logf("Matrix animation: %d channels, T=%d R=%d S=%d keyframes",
		len(anim.Channels), len(transCh.Times), len(rotCh.Times), len(scaleCh.Times))
}

func assertVec3(t *testing.T, expected, actual [3]float32, msg string) {
	t.Helper()
	assert.InDelta(t, expected[0], actual[0], 0.1, msg+" [x]")
	assert.InDelta(t, expected[1], actual[1], 0.1, msg+" [y]")
	assert.InDelta(t, expected[2], actual[2], 0.1, msg+" [z]")
}

func assertQuatUnit(t *testing.T, q [4]float32, msg string) {
	t.Helper()
	length := math.Sqrt(float64(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3]))
	assert.InDelta(t, 1.0, length, 0.1, msg+": quaternion should be unit length")
}
