package dae

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestDecodeAnimatedCube(t *testing.T) {
	data, err := os.ReadFile("testdata/animated_cube.dae")
	require.NoError(t, err)

	d := &Decoder{}
	asset, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, ir.FormatDAE, asset.Metadata.SourceFormat)
	assert.Equal(t, "1.4.1", asset.Metadata.SourceVersion)

	require.Len(t, asset.Meshes, 1, "should have 1 mesh")
	assert.Equal(t, "Cube", asset.Meshes[0].Name)

	require.Len(t, asset.Animations, 1, "should have 1 animation group")
	anim := asset.Animations[0]
	assert.Equal(t, "default", anim.Name)
	require.Len(t, anim.Channels, 2, "should have 2 channels (translate + rotate)")

	var transCh, rotCh *ir.AnimationChannel
	for i := range anim.Channels {
		switch anim.Channels[i].Target {
		case ir.TargetTranslation:
			transCh = &anim.Channels[i]
		case ir.TargetRotation:
			rotCh = &anim.Channels[i]
		}
	}

	require.NotNil(t, transCh, "should have translation channel")
	require.Len(t, transCh.Times, 3, "3 keyframes")
	assert.InDelta(t, float32(0.0), transCh.Times[0], 0.01)
	assert.InDelta(t, float32(0.5), transCh.Times[1], 0.01)
	assert.InDelta(t, float32(1.0), transCh.Times[2], 0.01)
	require.Len(t, transCh.Translations, 3, "3 translation values")
	assert.InDelta(t, float32(0), transCh.Translations[0][0], 0.01, "t=0: x=0")
	assert.InDelta(t, float32(1), transCh.Translations[1][0], 0.01, "t=0.5: x=1")
	assert.InDelta(t, float32(2), transCh.Translations[2][0], 0.01, "t=1.0: x=2")

	require.NotNil(t, rotCh, "should have rotation channel")
	require.Len(t, rotCh.Times, 2, "2 keyframes")
	require.Len(t, rotCh.Rotations, 2, "2 rotation values")

	assert.NotEqual(t, ir.NoIndex, transCh.NodeIndex, "node index should be resolved")
	if transCh.NodeIndex >= 0 && transCh.NodeIndex < len(asset.Nodes) {
		assert.Equal(t, "CubeNode", asset.Nodes[transCh.NodeIndex].Name)
	}

	t.Logf(
		"Animation: %d channels, translate=%d keyframes, rotate=%d keyframes",
		len(anim.Channels),
		len(transCh.Times),
		len(rotCh.Times),
	)
}

func TestConvertAnimationsEmpty(t *testing.T) {
	asset := &ir.Asset{}
	result := convertAnimations(context.Background(), nil, asset)
	assert.Nil(t, result, "nil input should return nil")
}
