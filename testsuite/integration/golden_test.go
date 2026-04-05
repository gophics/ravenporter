//go:build integration

package integration

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	jsonir "github.com/gophics/ravenporter/emit/json"
	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/process"
	"github.com/gophics/ravenporter/testsuite/corpus"
	"go.uber.org/goleak"
)

func TestRoundTrip_Golden(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name   string
		path   string
		preset process.PPFlag
	}{
		// ── Model: Quality ──
		{"OBJ_Quality", corpus.ModelOBJCube, process.PresetQuality},
		{"GLTF_Quality", corpus.ModelGLTF2BoxTextured, process.PresetQuality},
		{"STL_Quality", corpus.ModelSTLBinary, process.PresetQuality},
		{"PLY_Quality", corpus.ModelPLYCubeBinary, process.PresetQuality},
		{"DAE_Quality", corpus.ModelDAEDuck, process.PresetQuality},
		{"BVH_Quality", corpus.ModelBVHMoCap, process.PresetQuality},
		{"FBX_Quality", corpus.ModelFBXBox, process.PresetQuality},
		{"3DS_Quality", corpus.Model3DSCube, process.PresetQuality},
		{"3MF_Quality", corpus.Model3MFBox, process.PresetQuality},
		{"USDA_Quality", corpus.ModelUSDAComprehensive, process.PresetQuality},
		{"USDC_Quality", corpus.ModelUSDCComprehensive, process.PresetQuality},
		{"ABC_Quality", corpus.ModelABCCube, process.PresetQuality},

		// ── Model: Max Quality stress ──
		{"GLTF_Max", corpus.ModelGLTF2BoxTextured, process.PresetMaxQuality},
		{"OBJ_Max", corpus.ModelOBJCube, process.PresetMaxQuality},

		// ── Audio ──
		{"Audio_WAV", corpus.AudioWAV, 0},
		{"Audio_OGG", corpus.AudioOGG, 0},
		{"Audio_MP3", corpus.AudioMP3, 0},
		{"Audio_FLAC", corpus.AudioFLAC, 0},
		{"Audio_Opus", corpus.AudioOpus, 0},
		{"Audio_AIFF", corpus.AudioAIFF, 0},

		// ── Image ──
		{"Image_PNG", corpus.ImagePNGRed, 0},
		{"Image_JPEG", corpus.ImageJPEGRed, 0},
		{"Image_BMP", corpus.ImageBMPRed, 0},
		{"Image_HDR", corpus.ImageHDRTest, 0},
		{"Image_TGA", corpus.ImageTGABlue, 0},
		{"Image_EXR", corpus.ImageEXRTest, 0},
		{"Image_WebP", corpus.ImageWebPGallery, 0},
		{"Image_TIFF", corpus.ImageTIFFTest, 0},
		{"Image_PSD", corpus.ImagePSDMinimal, 0},
		{"Image_DDS", corpus.ImageDDSMinimal, 0},
		{"Image_KTX", corpus.ImageKTXMinimal, 0},

		// ── Font ──
		{"Font_TTF", corpus.FontRoboto, 0},
		{"Font_OTF", corpus.FontOTFMinimal, 0},
		{"Font_WOFF", corpus.FontWOFFMinimal, 0},
		{"Font_WOFF2", corpus.FontWOFF2Minimal, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := runPipeline(t, tc.path, pipeline.WithProcessFlags(tc.preset))

			var buf bytes.Buffer
			require.NoError(t, jsonir.WriteTo(asset, &buf, true), "json emission should never fail logically")
			require.True(t, buf.Len() > 50, "JSON output should be non-trivial: %d bytes", buf.Len())

			goldenCompare(t, asset, filepath.Join("..", "testdata", "golden", tc.name+".json"))
		})
	}
}
