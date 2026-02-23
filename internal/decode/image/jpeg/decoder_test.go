package jpeg_test

import (
	"bytes"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/jpeg"
	"github.com/gophics/ravenporter/ir"
)

var jpegData []byte

func init() {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	err := stdjpeg.Encode(&buf, img, &stdjpeg.Options{Quality: 100})
	if err != nil {
		panic("Failed to compile synthetic JPEG payload")
	}
	jpegData = buf.Bytes()
}

func TestJPEGProbe(t *testing.T) {
	dec := &jpeg.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(jpegData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestJPEGDecode(t *testing.T) {
	dec := &jpeg.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(jpegData), opts)

	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	imgScene := scene.Images[0]
	assert.Equal(t, ir.ImageJPEG, imgScene.Format)
	assert.Equal(t, 1, imgScene.Width)
	assert.Equal(t, 1, imgScene.Height)

	assert.Equal(t, ir.ChannelRGBA, imgScene.Channels)
	assert.Equal(t, ir.ColorSRGB, imgScene.ColorSpace)

	pb, decErr := imgScene.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 1*1*4) // LDR array inflates to flat RGBA slice representation
}

func TestJPEGDecodeWithICC(t *testing.T) {
	targetData := []byte{0xFF, 0xD8} // SOI

	iccPayload := append([]byte("ICC_PROFILE\x00"), []byte{1, 1}...)
	iccPayload = append(iccPayload, []byte("fake_icc_content_block")...)

	chunkLen := len(iccPayload) + 2
	targetData = append(targetData, 0xFF, 0xE2, byte(chunkLen>>8), byte(chunkLen&0xFF))
	targetData = append(targetData, iccPayload...)

	targetData = append(targetData, jpegData[2:]...)

	dec := &jpeg.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(targetData), opts)

	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	imgScene := scene.Images[0]
	assert.Equal(t, ir.ImageJPEG, imgScene.Format)
	require.NotNil(t, imgScene.Metadata)
	assert.Equal(t, "fake_icc_content_block", imgScene.Metadata["ICCProfile"])
}

func TestJPEGExtensions(t *testing.T) {
	dec := &jpeg.Decoder{}
	exts := dec.Extensions()
	assert.Contains(t, exts, ".jpeg")
	assert.Contains(t, exts, ".jpg")
}

func TestJPEGFormatName(t *testing.T) {
	dec := &jpeg.Decoder{}
	assert.Equal(t, "JPEG", dec.FormatName())
}

func BenchmarkDecode(b *testing.B) {
	dec := &jpeg.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(jpegData), opts)
	}
}
