package images

import (
	"image"
	"image/draw"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
	xdraw "golang.org/x/image/draw"
)

type resizeImagesStep struct{}

func (s *resizeImagesStep) Name() string      { return "ResizeImages" }
func (s *resizeImagesStep) Flag() core.PPFlag { return core.PPResizeImages }

func (s *resizeImagesStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	if opts.MaxTextureSize <= 0 {
		return asset, nil
	}

	maxSize := opts.MaxTextureSize

	for i := range asset.Images {
		img := asset.Images[i]
		if img == nil || img.Pixels() == nil {
			continue
		}

		w := img.Width
		h := img.Height

		if w <= maxSize && h <= maxSize {
			continue
		}

		ratio := float64(maxSize) / float64(w)
		if hRatio := float64(maxSize) / float64(h); hRatio < ratio {
			ratio = hRatio
		}

		newW := int(float64(w) * ratio)
		newH := int(float64(h) * ratio)
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}

		if len(img.Pixels().Data) > 0 {
			srcImg := &image.RGBA{
				Pix:    img.Pixels().Data,
				Stride: img.Width * 4,
				Rect:   image.Rect(0, 0, img.Width, img.Height),
			}
			dstImg := image.NewRGBA(image.Rect(0, 0, newW, newH))
			xdraw.BiLinear.Scale(dstImg, dstImg.Rect, srcImg, srcImg.Bounds(), draw.Over, nil)
			img.Pixels().Data = dstImg.Pix
			img.Width = newW
			img.Height = newH
		}
	}
	return asset, nil
}
