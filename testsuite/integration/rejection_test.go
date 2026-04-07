//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/testsuite/corpus"
)

func TestIntegration_RejectionPaths(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErrPart string
	}{
		// Audio Rejections
		{"Audio_WAV", corpus.RejectAudioWAV, "not a RIFF file"},
		{"Audio_FLAC", corpus.RejectAudioFLAC, "invalid magic bytes"},
		{"Audio_OPUS", corpus.RejectAudioOPUS, "failed to read id header"},
		{"Audio_MP3", corpus.RejectAudioMP3, "audio has 0 samples"},
		{"Audio_OGG", corpus.RejectAudioOGG, "failed to read id header"},
		{"Audio_AIFF", corpus.RejectAudioAIFF, "missing COMM chunk"},

		{"Font_TTF", corpus.RejectFontTTF, "invalid sfnt header"},
		{"Font_OTF", corpus.RejectFontOTF, "font data is suspiciously tiny"},
		{"Font_WOFF", corpus.RejectFontWOFF, "truncated WOFF data"},
		{"Font_WOFF2", corpus.RejectFontWOFF2, "truncated WOFF2 data"},

		{"Image_DDS", corpus.RejectImageDDS, "invalid or truncated header"},
		{"Image_TGA", corpus.RejectImageTGA, "malformed header"},
		{"Image_KTX", corpus.RejectImageKTX, "invalid ktx magic bytes"},
		{"Image_BMP", corpus.RejectImageBMP, "truncated BMP data"},
		{"Image_EXR", corpus.RejectImageEXR, "image dimensions are zero"},
		{"Image_HDR", corpus.RejectImageHDR, "not a Radiance HDR file"},
		{"Image_JPEG", corpus.RejectImageJPEG, "unexpected EOF"},
		{"Image_PNG", corpus.RejectImagePNG, "image dimensions are zero"},
		{"Image_PSD", corpus.RejectImagePSD, "image dimensions are zero"},
		{"Image_TIFF", corpus.RejectImageTIFF, "unexpected EOF"},
		{"Image_WEBP", corpus.RejectImageWEBP, "unknown format"},

		{"Model_GLTF", corpus.RejectModelGLTF, "structural validation failed"},
		{"Model_OBJ", corpus.RejectModelOBJ, "failed to read face data"},
		{"Model_FBX", corpus.RejectModelFBX, "ascii: no geometry found"},
		{"Model_DAE", corpus.RejectModelDAE, "XML syntax error"},
		{"Model_ABC", corpus.RejectModelABC, "truncated file"},
		{"Model_BVH", corpus.RejectModelBVH, "missing MOTION section"},
		{"Model_PLY", corpus.RejectModelPLY, "invalid PLY header"},
		{"Model_STL", corpus.RejectModelSTL, "failed to read triangle count"},
		{"Model_TDS", corpus.RejectModelTDS, "no decoder registered for format"},
		{"Model_3MF", corpus.RejectModel3MF, "not a valid zip file"},
		{"Model_USDA", corpus.RejectModelUSDA, "asset was completely empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("..", "corpus", tt.path)
			_, statErr := os.Stat(path)
			require.NoError(t, statErr, "test fixture must exist: %s", path)

			result, err := pipeline.ImportPath(context.Background(), path)

			if err == nil && result != nil && result.Asset != nil {
				asset := result.Asset
				// Exert pressure on lazy-evaluated endpoints if header parsing passed
				for _, img := range asset.Images {
					if img.PixelDecode != nil {
						_, pxErr := img.PixelDecode(img)
						if pxErr != nil {
							err = pxErr
						}
					}
					if img.Width == 0 || img.Height == 0 {
						err = errors.New("structural validation passed but image dimensions are zero")
					}
				}
				for _, aud := range asset.AudioClips {
					audSamples, _ := aud.DecodeSamples()
					if len(audSamples) == 0 {
						err = errors.New("structural validation passed but audio has 0 samples")
					}
				}
				for _, font := range asset.Fonts {
					if font.Vector != nil && len(font.Vector.RawData) < 20 {
						err = errors.New("structural validation passed but font data is suspiciously tiny")
					}
				}
				for range asset.Meshes {
					if err == nil {
						err = errors.New("structural validation passed but geometry was completely undefined")
					}
				}
				if err == nil && len(asset.Meshes) == 0 && len(asset.Images) == 0 && len(asset.Fonts) == 0 && len(asset.AudioClips) == 0 {
					err = errors.New("structural validation passed but asset was completely empty")
				}
			}

			require.Error(t, err, "expected properly formatted malformed file to safely reject")
			assert.Contains(t, err.Error(), tt.wantErrPart)
		})
	}
}
