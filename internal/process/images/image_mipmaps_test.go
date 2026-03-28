package images_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:unparam // test helper
func makeImageWithPixels(name string, w, h int, px []byte) *ir.ImageAsset {
	pb := &ir.PixelBuffer{Data: px, BitDepth: ir.BitDepth8}
	img := &ir.ImageAsset{
		Name:       name,
		Width:      w,
		Height:     h,
		Compressed: []byte{0}, // placeholder so DecodePixels doesn't short-circuit
		PixelDecode: func(_ *ir.ImageAsset) (*ir.PixelBuffer, error) {
			return pb, nil
		},
	}
	_, _ = img.DecodePixels()
	return img
}

func TestGenerateMipmaps(t *testing.T) {
	px := make([]byte, 4*4*4) // 4x4 RGBA
	for i := range px {
		px[i] = 128
	}

	asset := &ir.Asset{
		Images: []*ir.ImageAsset{makeImageWithPixels("test", 4, 4, px)},
	}

	require.NoError(t, process.Apply(asset, process.PPGenerateMipmaps, process.Options{}))
	assert.Greater(t, len(asset.Images[0].Pixels().Mipmaps), 0)
	assert.Equal(t, len(asset.Images[0].Pixels().Mipmaps)+1, asset.Images[0].MipLevels)

	t.Run("nil_image", func(t *testing.T) {
		s := &ir.Asset{Images: []*ir.ImageAsset{nil}}
		require.NoError(t, process.Apply(s, process.PPGenerateMipmaps, process.Options{}))
	})

	t.Run("skip_existing_mipmaps", func(t *testing.T) {
		img := makeImageWithPixels("test", 4, 4, px)
		img.Pixels().Mipmaps = [][]byte{{1, 2, 3, 4}}
		s := &ir.Asset{Images: []*ir.ImageAsset{img}}
		require.NoError(t, process.Apply(s, process.PPGenerateMipmaps, process.Options{}))
		assert.Len(t, s.Images[0].Pixels().Mipmaps, 1)
	})

	t.Run("width_1_skip", func(t *testing.T) {
		img := makeImageWithPixels("test", 1, 1, []byte{255, 0, 0, 255})
		s := &ir.Asset{Images: []*ir.ImageAsset{img}}
		require.NoError(t, process.Apply(s, process.PPGenerateMipmaps, process.Options{}))
		assert.Empty(t, s.Images[0].Pixels().Mipmaps)
	})
}
