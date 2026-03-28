package font_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/font/gofont/goregular"
)

func TestGenerateFontAtlas(t *testing.T) {
	tests := []struct {
		name         string
		fonts        []*ir.Font
		opts         process.Options
		wantAtlas    bool
		wantExisting bool
	}{
		{
			name:      "Valid TTF (GoRegular)",
			fonts:     []*ir.Font{{Vector: &ir.VectorFontData{RawData: goregular.TTF}}},
			opts:      process.Options{AtlasFontSize: 16},
			wantAtlas: true,
		},
		{
			name:      "Valid TTF Large Wrap",
			fonts:     []*ir.Font{{Vector: &ir.VectorFontData{RawData: goregular.TTF}}},
			opts:      process.Options{AtlasFontSize: 250},
			wantAtlas: true,
		},
		{
			name:      "Invalid TTF Data",
			fonts:     []*ir.Font{{Vector: &ir.VectorFontData{RawData: []byte("invalid data")}}},
			opts:      process.Options{AtlasFontSize: 16},
			wantAtlas: false,
		},
		{
			name:      "No-Op (FontSize 0)",
			fonts:     []*ir.Font{{Name: "test"}},
			opts:      process.Options{AtlasFontSize: 0},
			wantAtlas: false,
		},
		{
			name:      "Nil Font Guards",
			fonts:     []*ir.Font{nil, {Vector: nil}},
			opts:      process.Options{AtlasFontSize: 24},
			wantAtlas: false,
		},
		{
			name: "Skips Existing Atlas",
			fonts: []*ir.Font{{
				Name:   "existing",
				Vector: &ir.VectorFontData{RawData: []byte{0}},
				Bitmap: &ir.BitmapFontData{GlyphCount: 5, Glyphs: map[rune]ir.BitmapGlyph{'A': {}}},
			}},
			opts:         process.Options{AtlasFontSize: 24},
			wantAtlas:    false,
			wantExisting: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{Fonts: tt.fonts}
			require.NoError(t, process.Apply(asset, process.PPGenerateFontAtlas, tt.opts))

			if len(asset.Fonts) == 0 || asset.Fonts[0] == nil {
				return
			}

			if tt.wantAtlas {
				require.NotNil(t, asset.Fonts[0].Bitmap)
				idx := asset.Fonts[0].Bitmap.AtlasIndex
				assert.Greater(t, asset.Images[idx].Width, 0)
				assert.Greater(t, asset.Fonts[0].Bitmap.GlyphCount, 50)
			} else if tt.wantExisting {
				require.NotNil(t, asset.Fonts[0].Bitmap)
				assert.Equal(t, 5, asset.Fonts[0].Bitmap.GlyphCount)
			} else {
				assert.Nil(t, asset.Fonts[0].Bitmap)
			}
		})
	}
}
