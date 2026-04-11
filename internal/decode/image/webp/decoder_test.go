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

func TestWebPDecodeAnimated(t *testing.T) {
	dec := &imgwebp.Decoder{}

	stillScene, err := dec.Decode(bytes.NewReader(webpData), detect.DecodeOptions{})
	require.NoError(t, err)
	stillPixels, decErr := stillScene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.Len(t, stillPixels.Data, 4)
	frameColor := append([]byte(nil), stillPixels.Data[:4]...)

	animData := buildAnimatedWebP(5, 1, []animatedFrame{
		{x: 0, y: 0, w: 1, h: 1, durationMS: 10, doNotBlend: true, frameData: webpData[12:]},
		{x: 2, y: 0, w: 1, h: 1, durationMS: 20, doNotBlend: true, disposeToBackground: true, frameData: webpData[12:]},
		{x: 4, y: 0, w: 1, h: 1, durationMS: 30, doNotBlend: true, frameData: webpData[12:]},
	})

	scene, err := dec.Decode(bytes.NewReader(animData), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 3)

	assert.Equal(t, "10", scene.Images[0].Metadata["DelayNum"])
	assert.Equal(t, "1000", scene.Images[0].Metadata["DelayDen"])
	assert.Equal(t, "20", scene.Images[1].Metadata["DelayNum"])
	assert.Equal(t, "30", scene.Images[2].Metadata["DelayNum"])

	frame0, err := scene.Images[0].DecodePixels()
	require.NoError(t, err)
	frame1, err := scene.Images[1].DecodePixels()
	require.NoError(t, err)
	frame2, err := scene.Images[2].DecodePixels()
	require.NoError(t, err)

	assertPixelAt(t, frame0.Data, 0, frameColor)
	assertPixelAt(t, frame0.Data, 2, []byte{0, 0, 0, 0})
	assertPixelAt(t, frame0.Data, 4, []byte{0, 0, 0, 0})

	assertPixelAt(t, frame1.Data, 0, frameColor)
	assertPixelAt(t, frame1.Data, 2, frameColor)
	assertPixelAt(t, frame1.Data, 4, []byte{0, 0, 0, 0})

	assertPixelAt(t, frame2.Data, 0, frameColor)
	assertPixelAt(t, frame2.Data, 2, []byte{0, 0, 0, 0})
	assertPixelAt(t, frame2.Data, 4, frameColor)
}

func BenchmarkDecode(b *testing.B) {
	dec := &imgwebp.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(webpData), opts)
	}
}

func BenchmarkDecodeAnimated(b *testing.B) {
	dec := &imgwebp.Decoder{}
	opts := detect.DecodeOptions{}
	animData := buildAnimatedWebP(5, 1, []animatedFrame{
		{x: 0, y: 0, w: 1, h: 1, durationMS: 10, doNotBlend: true, frameData: webpData[12:]},
		{x: 2, y: 0, w: 1, h: 1, durationMS: 20, doNotBlend: true, disposeToBackground: true, frameData: webpData[12:]},
		{x: 4, y: 0, w: 1, h: 1, durationMS: 30, doNotBlend: true, frameData: webpData[12:]},
	})
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(animData), opts)
	}
}

type animatedFrame struct {
	x                   int
	y                   int
	w                   int
	h                   int
	durationMS          int
	doNotBlend          bool
	disposeToBackground bool
	frameData           []byte
}

func buildAnimatedWebP(canvasW, canvasH int, frames []animatedFrame) []byte {
	chunks := [][]byte{
		buildChunk("VP8X", buildVP8X(canvasW, canvasH)),
		buildChunk("ANIM", []byte{0, 0, 0, 0, 0, 0}),
	}
	for i := range frames {
		chunks = append(chunks, buildChunk("ANMF", buildANMF(frames[i])))
	}

	riffSize := 4
	for i := range chunks {
		riffSize += len(chunks[i])
	}

	out := make([]byte, 0, riffSize+8)
	out = append(out, 'R', 'I', 'F', 'F')
	out = appendU32(out, uint32(riffSize))
	out = append(out, 'W', 'E', 'B', 'P')
	for i := range chunks {
		out = append(out, chunks[i]...)
	}
	return out
}

func buildVP8X(width, height int) []byte {
	payload := make([]byte, 10)
	payload[0] = 1 << 1
	writeU24(payload[4:], width-1)
	writeU24(payload[7:], height-1)
	return payload
}

func buildANMF(frame animatedFrame) []byte {
	flags := byte(0)
	if frame.doNotBlend {
		flags |= 1 << 1
	}
	if frame.disposeToBackground {
		flags |= 1
	}

	payload := make([]byte, 16+len(frame.frameData))
	writeU24(payload[0:], frame.x/2)
	writeU24(payload[3:], frame.y/2)
	writeU24(payload[6:], frame.w-1)
	writeU24(payload[9:], frame.h-1)
	writeU24(payload[12:], frame.durationMS)
	payload[15] = flags
	copy(payload[16:], frame.frameData)
	return payload
}

func buildChunk(id string, payload []byte) []byte {
	out := make([]byte, 0, 8+len(payload)+len(payload)%2)
	out = append(out, id...)
	out = appendU32(out, uint32(len(payload)))
	out = append(out, payload...)
	if len(payload)%2 != 0 {
		out = append(out, 0)
	}
	return out
}

func appendU32(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func writeU24(dst []byte, v int) {
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
	dst[2] = byte(v >> 16)
}

func assertPixelAt(t *testing.T, data []byte, x int, want []byte) {
	t.Helper()
	off := x * 4
	require.Len(t, want, 4)
	require.GreaterOrEqual(t, len(data), off+4)
	assert.Equal(t, want, data[off:off+4])
}
