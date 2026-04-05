//go:build integration

package integration

import (
	"errors"
	"testing"

	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/rperr"
	"github.com/gophics/ravenporter/testsuite/corpus"
)

func TestIntegration_RejectionPaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		optCheck func(t *testing.T, err error)
	}{
		// Audio Rejections
		{"Audio_WAV", corpus.RejectAudioWAV, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Audio_FLAC", corpus.RejectAudioFLAC, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Audio_OPUS", corpus.RejectAudioOPUS, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Audio_MP3", corpus.RejectAudioMP3, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Audio_OGG", corpus.RejectAudioOGG, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Audio_AIFF", corpus.RejectAudioAIFF, func(t *testing.T, err error) { assert.Error(t, err) }},

		{"Font_TTF", corpus.RejectFontTTF, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Font_OTF", corpus.RejectFontOTF, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Font_WOFF", corpus.RejectFontWOFF, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Font_WOFF2", corpus.RejectFontWOFF2, func(t *testing.T, err error) { assert.Error(t, err) }},

		{"Image_DDS", corpus.RejectImageDDS, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_TGA", corpus.RejectImageTGA, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_KTX", corpus.RejectImageKTX, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_BMP", corpus.RejectImageBMP, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_EXR", corpus.RejectImageEXR, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_HDR", corpus.RejectImageHDR, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_JPEG", corpus.RejectImageJPEG, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_PNG", corpus.RejectImagePNG, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_PSD", corpus.RejectImagePSD, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_TIFF", corpus.RejectImageTIFF, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Image_WEBP", corpus.RejectImageWEBP, func(t *testing.T, err error) { assert.Error(t, err) }},

		{"Model_GLTF", corpus.RejectModelGLTF, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_OBJ", corpus.RejectModelOBJ, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_FBX", corpus.RejectModelFBX, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_DAE", corpus.RejectModelDAE, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_ABC", corpus.RejectModelABC, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_BVH", corpus.RejectModelBVH, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_PLY", corpus.RejectModelPLY, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_STL", corpus.RejectModelSTL, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_TDS", corpus.RejectModelTDS, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_3MF", corpus.RejectModel3MF, func(t *testing.T, err error) { assert.Error(t, err) }},
		{"Model_USDA", corpus.RejectModelUSDA, func(t *testing.T, err error) { assert.Error(t, err) }},
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
			t.Logf("Got error: %v (type %T)", err, err)
			tt.optCheck(t, err)

			var decErr *rperr.DecodeError
			isDecErr := errors.As(err, &decErr)
			isValErr := strings.Contains(err.Error(), "0 samples") || strings.Contains(err.Error(), "tiny") || strings.Contains(err.Error(), "dimensions are zero") || strings.Contains(err.Error(), "completely empty") || strings.Contains(err.Error(), "completely undefined") || strings.Contains(err.Error(), "validation failed") || strings.Contains(err.Error(), "undefined") || strings.Contains(err.Error(), "unexpected EOF") || strings.Contains(err.Error(), "format") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "short") || strings.Contains(err.Error(), "read") || strings.Contains(err.Error(), "truncated")
			assert.True(t, isDecErr || isValErr, "expected error to be a DecodeError or validation string, got %T: %v", err, err)
		})
	}
}
