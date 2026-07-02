//go:build integration

package integration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/testsuite/corpus"
)

// Benchmark_Memory_Allocations tracks precise allocs/op across core decoders.
// CI will fail if regressions occur against these baseline profiles.
func Benchmark_Memory_Allocations(b *testing.B) {
	benchmarks := []struct {
		name string
		path string
	}{
		{"Audio_WAV", corpus.AudioWAV},
		{"Audio_MP3", corpus.AudioMP3},
		{"Audio_Opus", corpus.AudioOpus},
		{"Audio_FLAC", corpus.AudioFLAC},
		{"Audio_OGG", corpus.AudioOGG},
		{"Audio_AIFF", corpus.AudioAIFF},

		{"Image_PNG", corpus.ImagePNGRed},
		{"Image_JPEG", corpus.ImageJPEGRed},
		{"Image_BMP", corpus.ImageBMPRed},
		{"Image_WebP", corpus.ImageWebPGallery},
		{"Image_TGA", corpus.ImageTGABlue},
		{"Image_HDR", corpus.ImageHDRTest},
		{"Image_TIFF", corpus.ImageTIFFTest},
		{"Image_EXR", corpus.ImageEXRTest},
		{"Image_DDS", corpus.ImageDDSMinimal},
		{"Image_KTX", corpus.ImageKTXMinimal},
		{"Image_PSD", corpus.ImagePSDMinimal},

		{"Font_TTF", corpus.FontRoboto},
		{"Font_OTF", corpus.FontOTFMinimal},
		{"Font_WOFF", corpus.FontWOFFMinimal},
		{"Font_WOFF2", corpus.FontWOFF2Minimal},

		{"Model_OBJ", corpus.ModelOBJCube},
		{"Model_GLTF2", corpus.ModelGLTF2BoxTextured},
		{"Model_GLTF2_Draco", corpus.IsoModelDracoGLTF},
		{"Model_FBX", corpus.ModelFBXBox},
		{"Model_STL", corpus.ModelSTLBinary},
		{"Model_PLY", corpus.ModelPLYCubeBinary},
		{"Model_DAE", corpus.ModelDAEDuck},
		{"Model_BVH", corpus.ModelBVHMoCap},
		{"Model_3DS", corpus.Model3DSCube},
		{"Model_3MF", corpus.Model3MFBox},
		{"Model_USDA", corpus.ModelUSDAComprehensive},
		{"Model_USDC", corpus.ModelUSDCComprehensive},
		{"Model_ABC", corpus.ModelABCCube},
	}

	for _, bc := range benchmarks {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			ctx := context.Background()
			path := filepath.Join(corpusDir(b, bc.path), filepath.FromSlash(bc.path))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := pipeline.ImportPath(ctx, path)
				if err != nil {
					b.Fatalf("benchmark failed to process %s: %v", bc.path, err)
				}
			}
		})
	}
}
