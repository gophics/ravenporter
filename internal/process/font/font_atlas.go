package font

import (
	"image"
	"image/draw"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	defaultAtlasSize = 1024
	paddingPixels    = 2
	asciiStart       = 32
	asciiEnd         = 126
	maxDPI           = 72.0
)

type generateFontAtlasStep struct{}

func (s *generateFontAtlasStep) Name() string      { return "GenerateFontAtlas" }
func (s *generateFontAtlasStep) Flag() core.PPFlag { return core.PPGenerateFontAtlas }

func (s *generateFontAtlasStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	if opts.AtlasFontSize <= 0 {
		return asset, nil
	}

	for i := range asset.Fonts {
		f := asset.Fonts[i]
		if f == nil || f.Vector == nil || !f.Vector.HasRawBytes() {
			continue
		}
		if f.Bitmap != nil && len(f.Bitmap.Glyphs) > 0 {
			continue
		}

		rawData, err := f.Vector.RawBytes()
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.Warn("failed to load font bytes", "font", f.Name, "err", err)
			}
			continue
		}
		ttf, err := opentype.Parse(rawData)
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.Warn("failed to parse font TTF", "font", f.Name, "err", err)
			}
			continue
		}

		face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
			Size:    float64(opts.AtlasFontSize),
			DPI:     maxDPI,
			Hinting: font.HintingFull,
		})
		if err != nil {
			continue
		}

		atlasImg, glyphs, lineHeight := generateAtlas(face)
		if err := face.Close(); err != nil {
			if opts.Logger != nil {
				opts.Logger.Warn("failed to close font face", "err", err)
			}
		}

		texIdx := len(asset.Images)
		atlasPB := &ir.PixelBuffer{
			Data:     atlasImg.Pix,
			BitDepth: ir.BitDepth8,
		}
		asset.Images = append(asset.Images, &ir.ImageAsset{
			Name:   f.Name + "_atlas",
			Format: ir.ImagePNG,
			Width:  atlasImg.Bounds().Dx(),
			Height: atlasImg.Bounds().Dy(),
			PixelDecode: func(_ *ir.ImageAsset) (*ir.PixelBuffer, error) {
				return atlasPB, nil
			},
		})

		f.Bitmap = &ir.BitmapFontData{
			AtlasIndex: texIdx,
			LineHeight: lineHeight,
			GlyphCount: len(glyphs),
			Glyphs:     glyphs,
		}
	}

	return asset, nil
}

func generateAtlas(face font.Face) (outAtlas *image.NRGBA, outGlyphs map[rune]ir.BitmapGlyph, outLineHeight int) {
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	ascent := metrics.Ascent.Ceil()

	atlas := image.NewNRGBA(image.Rect(0, 0, defaultAtlasSize, defaultAtlasSize))
	glyphs := make(map[rune]ir.BitmapGlyph, asciiEnd-asciiStart+1)

	var cx, cy, rowHeight int

	for r := rune(asciiStart); r <= rune(asciiEnd); r++ {
		bounds, advance, ok := face.GlyphBounds(r)
		if !ok {
			continue
		}

		w := bounds.Max.X.Ceil() - bounds.Min.X.Floor()
		h := bounds.Max.Y.Ceil() - bounds.Min.Y.Floor()
		offX := bounds.Min.X.Floor()
		offY := bounds.Min.Y.Floor()

		if cx+w+paddingPixels > defaultAtlasSize {
			cx = 0
			cy += rowHeight + paddingPixels
			rowHeight = 0
		}
		if h > rowHeight {
			rowHeight = h
		}

		dot := fixed.P(cx-offX, cy-offY+ascent)
		dr, mask, maskp, _, _ := face.Glyph(dot, r)
		draw.Draw(atlas, dr, mask, maskp, draw.Src)

		glyphs[r] = ir.BitmapGlyph{
			X:       cx,
			Y:       cy,
			Width:   w,
			Height:  h,
			XOffset: offX,
			YOffset: offY,
			Advance: advance.Floor(),
		}

		cx += w + paddingPixels
	}

	return atlas, glyphs, lineHeight
}
