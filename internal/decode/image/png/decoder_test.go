package png_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	stdpng "image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/png"
	"github.com/gophics/ravenporter/ir"
)

var pngData []byte
var magicPNG = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

func init() {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	err := stdpng.Encode(&buf, img)
	if err != nil {
		panic("Failed to initialize PNG mocked test stream")
	}
	pngData = buf.Bytes()
}

func appendChunk(target []byte, chunkType string, payload []byte) []byte {
	length := len(payload)
	buf := make([]byte, 12+length)
	binary.BigEndian.PutUint32(buf[0:], uint32(length))
	copy(buf[4:], chunkType)
	if length > 0 {
		copy(buf[8:], payload)
	}
	crc := crc32.ChecksumIEEE(buf[4 : 8+length])
	binary.BigEndian.PutUint32(buf[8+length:], crc)
	return append(target, buf...)
}

func TestPNGProbe(t *testing.T) {
	dec := &png.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(pngData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestPNGDecode(t *testing.T) {
	dec := &png.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(pngData), opts)
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	imgScene := scene.Images[0]
	assert.Equal(t, ir.ImagePNG, imgScene.Format)
	assert.Equal(t, 1, imgScene.Width)
	assert.Equal(t, 1, imgScene.Height)
	assert.Equal(t, ir.ChannelRGBA, imgScene.Channels)
	assert.Equal(t, ir.ColorSRGB, imgScene.ColorSpace)

	pb, decErr := imgScene.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 1*1*4)
}

func TestPNGDecodeAPNG(t *testing.T) {
	targetData := append([]byte(nil), magicPNG...)

	ihdrData := []byte{0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0}
	targetData = appendChunk(targetData, "IHDR", ihdrData)

	// Create acTL chunk
	actlData := []byte{0, 0, 0, 2, 0, 0, 0, 0} // 2 frames
	targetData = appendChunk(targetData, "acTL", actlData)

	fctl0Data := make([]byte, 26)
	binary.BigEndian.PutUint32(fctl0Data[4:], 1) // width
	binary.BigEndian.PutUint32(fctl0Data[8:], 1) // height
	targetData = appendChunk(targetData, "fcTL", fctl0Data)

	idatStart := 8 + 25 // magic + IHDR overhead
	idatLen := int(binary.BigEndian.Uint32(pngData[idatStart:]))
	idatPayload := pngData[idatStart+8 : idatStart+8+idatLen]

	targetData = appendChunk(targetData, "IDAT", idatPayload)

	fctl1Data := make([]byte, 26)
	binary.BigEndian.PutUint32(fctl1Data[0:], 1) // seq
	binary.BigEndian.PutUint32(fctl1Data[4:], 1) // width
	binary.BigEndian.PutUint32(fctl1Data[8:], 1) // height
	targetData = appendChunk(targetData, "fcTL", fctl1Data)

	seqPayload := append([]byte{0, 0, 0, 2}, idatPayload...)
	targetData = appendChunk(targetData, "fdAT", seqPayload)

	targetData = appendChunk(targetData, "IEND", nil)

	dec := &png.Decoder{}
	opts := detect.DecodeOptions{}
	scene, err := dec.Decode(bytes.NewReader(targetData), opts)
	require.NoError(t, err)

	require.Len(t, scene.Images, 2)
	assert.Equal(t, ir.ImagePNG, scene.Images[0].Format)
	assert.Equal(t, "png_0", scene.Images[0].Name)
	assert.Equal(t, "png_1", scene.Images[1].Name)
}

func TestPNGExtensions(t *testing.T) {
	dec := &png.Decoder{}
	assert.Contains(t, dec.Extensions(), ".png")
}

func TestPNGFormatName(t *testing.T) {
	dec := &png.Decoder{}
	assert.Equal(t, "PNG", dec.FormatName())
}

func BenchmarkDecode(b *testing.B) {
	dec := &png.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(pngData), opts)
	}
}
