//go:build integration

package integration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/testsuite/corpus"
)

const (
	msgReportGPUFmt    = "DDS/KTX must report GPU compression format"
	msgBlockHeuristics = "compressed payload must meet minimum block size heuristics"
	msgRLESuccess      = "RLE decode should succeed"
	msgRLEMatch        = "decoded pixel buffer should match width*height*rgba"

	errStrMaxFileSize = "MaxFileSize"
	errStrLimitViol   = "error should denote file size limit"
	errStrPipelineIl  = "Pipeline illegally returned success. Images=%d"
	errStrPipelineErr = "Pipeline should error due to MaxFileSize limit"
)

func TestIntegration_Image(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		format       ir.ImageFormat
		sourceFormat ir.FormatID
		width        int
		height       int
		ch           ir.ChannelCount
		space        ir.ColorSpace
		gpu          bool // DDS/KTX skip mip level check
		verifyFn     func(t *testing.T, img *ir.ImageAsset)
	}{
		// Core formats
		{"PNG_Red", corpus.ImagePNGRed, ir.ImagePNG, ir.FormatPNG, 4, 4, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"JPEG_Red", corpus.ImageJPEGRed, ir.ImageJPEG, ir.FormatJPEG, 4, 4, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"BMP_Red", corpus.ImageBMPRed, ir.ImageBMP, ir.FormatBMP, 4, 4, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"WebP_Gallery", corpus.ImageWebPGallery, ir.ImageWebP, ir.FormatWebP, 550, 368, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"TGA", corpus.ImageTGABlue, ir.ImageTGA, ir.FormatTGA, 2, 2, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"TIFF", corpus.ImageTIFFTest, ir.ImageTIFF, ir.FormatTIFF, 640, 480, 1, ir.ColorSRGB, false, nil},
		{"PSD", corpus.ImagePSDMinimal, ir.ImagePSD, ir.FormatPSD, 100, 200, ir.ChannelRGBA, ir.ColorSRGB, false, nil},

		// HDR / Linear
		{"HDR", corpus.ImageHDRTest, ir.ImageHDR, ir.FormatHDR, 1024, 512, ir.ChannelRGB, ir.ColorLinear, false, nil},
		{"EXR", corpus.ImageEXRTest, ir.ImageEXR, ir.FormatEXR, 800, 800, 1, ir.ColorLinear, false, nil},

		// GPU-compressed passthrough
		{"DDS", corpus.ImageDDSMinimal, ir.ImageDDS, ir.FormatDDS, 64, 32, ir.ChannelRGBA, ir.ColorSRGB, true, func(t *testing.T, img *ir.ImageAsset) {
			assert.NotEqual(t, ir.GPUCompressionNone, img.CompressionFormat, msgReportGPUFmt)
			minSize := 16
			assert.True(t, len(img.Compressed) >= minSize, msgBlockHeuristics)
		}},
		{"KTX", corpus.ImageKTXMinimal, ir.ImageKTX, ir.FormatKTX, 128, 64, ir.ChannelRGBA, ir.ColorSRGB, true, func(t *testing.T, img *ir.ImageAsset) {
			assert.NotEqual(t, ir.GPUCompressionNone, img.CompressionFormat, msgReportGPUFmt)
			minSize := 16
			assert.True(t, len(img.Compressed) >= minSize, msgBlockHeuristics)
		}},

		// Variant files
		{"HDR_Outdoor", corpus.ImageHDROutdoor, ir.ImageHDR, ir.FormatHDR, 1024, 512, ir.ChannelRGB, ir.ColorLinear, false, nil},
		{"PNG_Photo", corpus.ImagePNGPhoto, ir.ImagePNG, ir.FormatPNG, 640, 426, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"JPEG_Photo", corpus.ImageJPEGPhoto, ir.ImageJPEG, ir.FormatJPEG, 640, 426, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"BMP_Photo", corpus.ImageBMPPhoto, ir.ImageBMP, ir.FormatBMP, 640, 426, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"WebP_Small", corpus.ImageWebPSmall, ir.ImageWebP, ir.FormatWebP, 550, 368, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"TGA_32bit", corpus.ImageTGA32bit, ir.ImageTGA, ir.FormatTGA, 640, 426, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"TIFF_Gray", corpus.ImageTIFFGray, ir.ImageTIFF, ir.FormatTIFF, 640, 426, ir.ChannelRGBA, ir.ColorSRGB, false, nil},

		// Exhaustive Isolation
		{"TGA_NPOT", corpus.IsoImageNPOT, ir.ImageTGA, ir.FormatTGA, 1, 8192, ir.ChannelRGBA, ir.ColorSRGB, false, nil},
		{"TGA_RLE", corpus.IsoImageRLE, ir.ImageTGA, ir.FormatTGA, 10, 1, ir.ChannelRGBA, ir.ColorSRGB, false, func(t *testing.T, img *ir.ImageAsset) {
			pb, err := img.DecodePixels()
			require.NoError(t, err, msgRLESuccess)
			require.Len(t, pb.Data, 10*1*4, msgRLEMatch)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := runPipeline(t, tc.path)
			require.Len(t, asset.Images, 1, "expected exactly 1 image")
			img := asset.Images[0]

			assert.Equal(t, tc.sourceFormat, asset.Metadata.SourceFormat)

			// Shared image invariants.
			assert.NotEmpty(t, img.Name)
			assert.Equal(t, tc.format, img.Format)
			assert.Equal(t, tc.width, img.Width, "width mismatch")
			assert.Equal(t, tc.height, img.Height, "height mismatch")
			assert.Equal(t, tc.ch, img.Channels, "channel count mismatch")
			assert.Equal(t, tc.space, img.ColorSpace, "colorspace mismatch")
			assert.True(t, len(img.Compressed) > 0, "raw bytes must be retained")

			if !tc.gpu {
				assert.True(t, img.MipLevels >= 1, "mip levels must be >= 1")
			}

			if tc.verifyFn != nil {
				tc.verifyFn(t, img)
			}

			t.Logf("%s: %dx%d ch=%v space=%v mips=%d comp=%q compressed=%d",
				tc.name, img.Width, img.Height, img.Channels, img.ColorSpace, img.MipLevels, img.CompressionFormat, len(img.Compressed))
		})
	}
}

// TestIntegration_Image_DecodePixels verifies the lazy pixel decode path
// for formats that support it. ImageAsset.DecodePixels() decodes Compressed
// bytes using standard library or custom parsers.
func TestIntegration_Image_DecodePixels(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantFloat bool
	}{
		{"PNG_LDR", corpus.ImagePNGRed, false},
		{"JPEG_LDR", corpus.ImageJPEGRed, false},
		{"BMP_LDR", corpus.ImageBMPRed, false},
		{"TGA_LDR", corpus.ImageTGABlue, false},
		{"WebP_LDR", corpus.ImageWebPGallery, false},
		{"TIFF_LDR", corpus.ImageTIFFTest, false},
		{"PSD_LDR", corpus.ImagePSDMinimal, false},
		{"HDR_Float", corpus.ImageHDRTest, true},
		{"EXR_Float", corpus.ImageEXRTest, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := runPipeline(t, tc.path)
			require.Len(t, asset.Images, 1)
			img := asset.Images[0]

			require.False(t, img.IsGPUCompressed(), "test files must not be GPU compressed")
			pb, err := img.DecodePixels()
			require.NoError(t, err, "DecodePixels must succeed")
			require.NotNil(t, pb, "pixels must be decoded after DecodePixels()")

			expectedCh := 4
			if img.Format == ir.ImageHDR {
				expectedCh = 3
			}
			expectedBytesPerCh := 1
			if pb.DataType == ir.DataTypeFloat32 {
				expectedBytesPerCh = 4
			}
			expectedLen := img.Width * img.Height * expectedCh * expectedBytesPerCh
			assert.Len(t, pb.Data, expectedLen, "pixel buffer length must match decoded layout")

			if tc.wantFloat {
				assert.Equal(t, ir.DataTypeFloat32, pb.DataType, "HDR pixel data must be Float32")
			} else {
				assert.Equal(t, ir.DataTypeUint8, pb.DataType, "LDR pixel data must be Uint8")
				assert.NotZero(t, pb.BitDepth, "LDR bit depth must be set")
			}
		})
	}
}

func TestIntegration_Image_GPUCompressedError(t *testing.T) {
	paths := []string{corpus.ImageDDSMinimal, corpus.ImageKTXMinimal}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			asset := runPipeline(t, p)
			require.Len(t, asset.Images, 1)
			img := asset.Images[0]

			require.True(t, img.IsGPUCompressed(), "file must be GPU compressed")
			pb, err := img.DecodePixels()
			require.Error(t, err, "GPU compressed images should error on DecodePixels()")
			assert.Nil(t, pb, "pixel buffer should be nil for GPU compressed images")
		})
	}
}

func TestIntegration_Image_Metadata(t *testing.T) {
	asset := runPipeline(t, corpus.ImagePSDMinimal)
	require.Len(t, asset.Images, 1)
	meta := asset.Images[0].Metadata
	require.NotNil(t, meta)
	assert.Len(t, meta, 2)
	assert.Equal(t, "8", meta["BitDepth"])
	assert.Equal(t, "3", meta["ColorMode"])
}

func TestIntegration_Image_MemoryClamps(t *testing.T) {
	paths := []string{corpus.ImageEXRTest, corpus.ImageTIFFTest}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			path := filepath.Join(corpusDir(t, p), filepath.FromSlash(p))
			result, err := pipeline.ImportPath(context.Background(), path, pipeline.WithDecodeMaxFileSize(100))
			if err == nil {
				t.Logf(errStrPipelineIl, len(result.Asset.Images))
			}
			require.Error(t, err, errStrPipelineErr)
			assert.Contains(t, err.Error(), errStrMaxFileSize, errStrLimitViol)
		})
	}
}
