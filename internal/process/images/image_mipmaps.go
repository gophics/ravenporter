package images

import (
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type generateMipmapsStep struct{}

func (s *generateMipmapsStep) Name() string      { return "GenerateMipmaps" }
func (s *generateMipmapsStep) Flag() core.PPFlag { return core.PPGenerateMipmaps }

func (s *generateMipmapsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Images {
		img := asset.Images[i]
		if img == nil || img.Pixels() == nil || img.Width <= 1 {
			continue
		}

		if len(img.Pixels().Mipmaps) > 0 {
			continue
		}

		mipmaps := generateLDRMipchains(img.Pixels().Data, img.Width, img.Height)
		img.Pixels().Mipmaps = mipmaps
		img.MipLevels = len(mipmaps) + 1
	}
	return asset, nil
}

func generateLDRMipchains(src []uint8, srcW, srcH int) [][]byte {
	maxDim := srcW
	if srcH > maxDim {
		maxDim = srcH
	}
	mipCount := 0
	for d := maxDim; d > 1; d /= 2 {
		mipCount++
	}
	mips := make([][]byte, 0, mipCount)
	w := srcW
	h := srcH
	curr := src

	for w > 1 || h > 1 {
		const halfScale = 2
		const pixelStride = 4

		nw := w / halfScale
		nh := h / halfScale
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}

		next := make([]byte, nw*nh*pixelStride)

		for y := range nh {
			for x := range nw {
				sx := x * halfScale
				sy := y * halfScale

				r, g, b, a := 0, 0, 0, 0
				count := 0

				for dy := range 2 {
					for dx := range 2 {
						px := sx + dx
						py := sy + dy
						if px < w && py < h {
							idx := (py*w + px) * pixelStride
							r += int(curr[idx])
							g += int(curr[idx+1])
							b += int(curr[idx+2])
							a += int(curr[idx+3])
							count++
						}
					}
				}

				if count > 0 {
					dstIdx := (y*nw + x) * pixelStride
					next[dstIdx] = uint8(r / count)   //nolint:gosec // division fits into uint8 bounds max 255
					next[dstIdx+1] = uint8(g / count) //nolint:gosec // bounds max 255
					next[dstIdx+2] = uint8(b / count) //nolint:gosec // bounds max 255
					next[dstIdx+3] = uint8(a / count) //nolint:gosec // bounds max 255
				}
			}
		}

		mips = append(mips, next)
		curr = next
		w = nw
		h = nh
	}

	return mips
}
