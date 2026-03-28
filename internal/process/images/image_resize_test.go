package images_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResizeImages(t *testing.T) {
	px := make([]byte, 8*8*4) // 8x8 RGBA
	for i := range px {
		px[i] = 200
	}

	tests := []struct {
		name       string
		maxSize    int
		wantResize bool
	}{
		{name: "downscale", maxSize: 4, wantResize: true},
		{name: "zero_no_op", maxSize: 0, wantResize: false},
		{name: "larger_no_op", maxSize: 16, wantResize: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{Images: []*ir.ImageAsset{
				makeImageWithPixels("test", 8, 8, append([]byte{}, px...)),
			}}
			require.NoError(t, process.Apply(asset, process.PPResizeImages, process.Options{MaxTextureSize: tt.maxSize}))
			if tt.wantResize {
				assert.LessOrEqual(t, asset.Images[0].Width, tt.maxSize)
				assert.LessOrEqual(t, asset.Images[0].Height, tt.maxSize)
			} else {
				assert.Equal(t, 8, asset.Images[0].Width)
			}
		})
	}

	t.Run("nil_image", func(t *testing.T) {
		s := &ir.Asset{Images: []*ir.ImageAsset{nil}}
		require.NoError(t, process.Apply(s, process.PPResizeImages, process.Options{MaxTextureSize: 4}))
	})
}
