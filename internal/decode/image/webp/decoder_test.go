package webp_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	imgwebp "github.com/gophics/ravenporter/internal/decode/image/webp"
	"github.com/gophics/ravenporter/ir"
)

var webpData []byte

func init() {
	b64 := "UklGRjIAAABXRUJQVlA4ICYAAACyAgCdASoBAAEALmk0mk0iIiIiIgBoSygABc6zbAAA/v56QAAAAA=="
	var err error
	webpData, err = base64.StdEncoding.DecodeString(b64)
	if err != nil {
		panic("invalid webp structure mock initialization")
	}
}

func TestWebPProbe(t *testing.T) {
	dec := &imgwebp.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(webpData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestWebPDecode(t *testing.T) {
	dec := &imgwebp.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(webpData), opts)
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	imgScene := scene.Images[0]
	assert.Equal(t, ir.ImageWebP, imgScene.Format)
	assert.Equal(t, 1, imgScene.Width)
	assert.Equal(t, 1, imgScene.Height)

	assert.Equal(t, ir.ChannelRGBA, imgScene.Channels)
	assert.Equal(t, ir.ColorSRGB, imgScene.ColorSpace)

	pb, decErr := imgScene.DecodePixels()
	require.NoError(t, decErr)
	assert.NotNil(t, pb)
	assert.Len(t, pb.Data, 1*1*4)
}

func BenchmarkDecode(b *testing.B) {
	dec := &imgwebp.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(webpData), opts)
	}
}
