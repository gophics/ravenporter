package decoder

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	aiffdec "github.com/gophics/ravenporter/internal/decode/audio/aiff"
	mp3dec "github.com/gophics/ravenporter/internal/decode/audio/mp3"
	oggdec "github.com/gophics/ravenporter/internal/decode/audio/ogg"
	"github.com/gophics/ravenporter/ir"
)

func TestDecodeAIFFRealFile(t *testing.T) {
	scene, err := (&aiffdec.Decoder{}).Decode(bytes.NewReader(sourceData(t, "audio", "aiff_8bit.aif")), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioAIFF, clip.Format)
	assert.Equal(t, 44100, clip.SampleRate)
	assert.Equal(t, ir.LayoutStereo, clip.Layout)
	assert.Equal(t, ir.BitDepth16, clip.BitDepth)
	samples, sampleErr := clip.DecodeSamples()
	require.NoError(t, sampleErr)
	assert.NotEmpty(t, samples)
}

func TestDecodeMP3RealFile(t *testing.T) {
	scene, err := (&mp3dec.Decoder{}).Decode(bytes.NewReader(sourceData(t, "audio", "outFoxing.mp3")), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioMP3, clip.Format)
	assert.Equal(t, 44100, clip.SampleRate)
	assert.Equal(t, ir.LayoutMono, clip.Layout)
	assert.Equal(t, ir.BitDepth16, clip.BitDepth)
	samples, sampleErr := clip.DecodeSamples()
	require.NoError(t, sampleErr)
	assert.NotEmpty(t, samples)
}

func TestDecodeOGGRealFile(t *testing.T) {
	scene, err := (&oggdec.Decoder{}).Decode(bytes.NewReader(sourceData(t, "audio", "Example.ogg")), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	clip := scene.AudioClips[0]
	assert.Equal(t, ir.AudioOGG, clip.Format)
	assert.Equal(t, 44100, clip.SampleRate)
	assert.Equal(t, ir.LayoutStereo, clip.Layout)
	assert.Equal(t, ir.BitDepth16, clip.BitDepth)
	samples, sampleErr := clip.DecodeSamples()
	require.NoError(t, sampleErr)
	assert.NotEmpty(t, samples)
}

func FuzzDecodeAIFF(f *testing.F) {
	seed, err := os.ReadFile(sourcePath("audio", "aiff_8bit.aif"))
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("FORM\x00\x00\x00\x00AIFF"))
	f.Add([]byte("FORM\x00\x00\x00\x00AIFC"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = (&aiffdec.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
