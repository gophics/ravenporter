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

func TestIntegration_Font(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectedFmt  ir.FontFormat
		sourceFormat ir.FormatID
		verifyFn     func(t *testing.T, f *ir.Font)
	}{
		{"Roboto", corpus.FontRoboto, ir.FontTTF, ir.FormatTTF, func(t *testing.T, f *ir.Font) {
			assert.Equal(t, "Roboto", f.Name)
			assert.Equal(t, "Roboto", f.Family)
			assert.Equal(t, "Regular", f.Subfamily)
			assert.Equal(t, "Roboto-Regular", f.PostScript)
			require.NotNil(t, f.Vector)
			assert.Equal(t, 1326, f.Vector.GlyphCount)
			assert.Equal(t, 2048, f.Vector.UnitsPerEm)
			assert.Equal(t, 1536, f.Vector.Ascender)
			assert.Equal(t, -512, f.Vector.Descender)
			assert.Equal(t, 102, f.Vector.LineGap)
			assert.True(t, len(f.Vector.RawData) > 0, "raw font bytes must be retained")
			assert.True(t, len(f.Metadata) > 0, "metadata (copyright/trademark) must be parsed")
			assert.True(t, len(f.Vector.Codepoints) > 0, "codepoints must be populated")
			assert.True(t, len(f.Vector.Advances) > 0, "advance widths must be populated")
			t.Logf("Roboto: codepoints=%d advances=%d kerning=%d", len(f.Vector.Codepoints), len(f.Vector.Advances), len(f.Vector.Kerning))
		}},
		{"OpenSans", corpus.FontOpenSans, ir.FontTTF, ir.FormatTTF, func(t *testing.T, f *ir.Font) {
			assert.Contains(t, f.Family, "Open Sans")
			assert.NotEmpty(t, f.Subfamily)
			assert.NotEmpty(t, f.PostScript, "PostScript name must be parsed")
			require.NotNil(t, f.Vector)
			assert.True(t, f.Vector.GlyphCount > 0, "must have glyphs")
			assert.True(t, f.Vector.UnitsPerEm > 0, "units per em must be set")
			assert.NotZero(t, f.Vector.Ascender, "ascender must be set")
			assert.NotZero(t, f.Vector.Descender, "descender must be set")
			assert.True(t, len(f.Vector.RawData) > 0, "raw font bytes must be retained")
			assert.True(t, len(f.Metadata) > 0, "metadata (copyright/trademark) must be parsed")
			assert.True(t, len(f.Vector.Codepoints) > 0, "codepoints must be populated")
			assert.True(t, len(f.Vector.Advances) > 0, "advance widths must be populated")
			t.Logf("OpenSans: codepoints=%d advances=%d kerning=%d", len(f.Vector.Codepoints), len(f.Vector.Advances), len(f.Vector.Kerning))
		}},
		{"OTF_Minimal", corpus.FontOTFMinimal, ir.FontOTF, ir.FormatOTF, func(t *testing.T, f *ir.Font) {
			assert.Equal(t, "OTF", f.Name)
			assert.Empty(t, f.Family)
			assert.Empty(t, f.Subfamily)
			assert.Empty(t, f.PostScript)
			require.NotNil(t, f.Vector)
			assert.Equal(t, 0, f.Vector.UnitsPerEm)
			assert.Equal(t, 0, f.Vector.GlyphCount)
			assert.Zero(t, f.Vector.Ascender)
			assert.Zero(t, f.Vector.Descender)
			assert.Zero(t, f.Vector.LineGap)
			assert.True(t, len(f.Vector.RawData) > 0, "raw font bytes must be retained")
			assert.Empty(t, f.Metadata)
		}},
		{"WOFF_Minimal", corpus.FontWOFFMinimal, ir.FontWOFF, ir.FormatWOFF, func(t *testing.T, f *ir.Font) {
			assert.Equal(t, "TestFont", f.Name)
			assert.Equal(t, "TestFont", f.Family)
			assert.Empty(t, f.Subfamily)
			assert.Empty(t, f.PostScript)
			require.NotNil(t, f.Vector)
			assert.Equal(t, 0, f.Vector.UnitsPerEm)
			assert.Equal(t, 0, f.Vector.GlyphCount)
			assert.Equal(t, 800, f.Vector.Ascender)
			assert.Equal(t, -200, f.Vector.Descender)
			assert.Equal(t, 90, f.Vector.LineGap)
			assert.True(t, len(f.Vector.RawData) > 0, "raw font bytes must be retained")
			assert.Empty(t, f.Metadata)
		}},
		{"WOFF2_Minimal", corpus.FontWOFF2Minimal, ir.FontWOFF2, ir.FormatWOFF2, func(t *testing.T, f *ir.Font) {
			assert.Equal(t, "TestFont", f.Name)
			assert.Equal(t, "TestFont", f.Family)
			assert.Empty(t, f.Subfamily)
			assert.Empty(t, f.PostScript)
			require.NotNil(t, f.Vector)
			assert.Equal(t, 0, f.Vector.UnitsPerEm)
			assert.Equal(t, 0, f.Vector.GlyphCount)
			assert.Equal(t, 800, f.Vector.Ascender)
			assert.Equal(t, -200, f.Vector.Descender)
			assert.Equal(t, 90, f.Vector.LineGap)
			assert.True(t, len(f.Vector.RawData) > 0, "raw font bytes must be retained")
			assert.Empty(t, f.Metadata)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asset := runPipeline(t, tc.path)
			require.Len(t, asset.Fonts, 1, "expected exactly 1 font")
			f := asset.Fonts[0]

			assert.Equal(t, tc.sourceFormat, asset.Metadata.SourceFormat)

			// Shared Font invariants.
			assert.NotEmpty(t, f.Name)
			assert.Equal(t, tc.expectedFmt, f.Format)

			tc.verifyFn(t, f)

			subfamily := f.Subfamily
			family := f.Family
			glyphs, upm, rawLen, ascender := 0, 0, 0, 0
			if f.Vector != nil {
				glyphs = f.Vector.GlyphCount
				upm = f.Vector.UnitsPerEm
				rawLen = len(f.Vector.RawData)
				ascender = f.Vector.Ascender
			}
			t.Logf("%s: family=%q sub=%q glyphs=%d upm=%d asc=%d raw=%d",
				tc.name, family, subfamily, glyphs, upm, ascender, rawLen)
		})
	}
}

func TestIntegration_Font_MemoryClamps(t *testing.T) {
	paths := []string{corpus.FontOpenSans, corpus.FontWOFF2Minimal}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			path := filepath.Join(corpusDir(t, p), filepath.FromSlash(p))
			result, err := pipeline.ImportPath(context.Background(), path, pipeline.WithDecodeMaxFileSize(10))
			if err == nil {
				asset := result.Asset
				t.Logf("Pipeline illegally returned success. Meshes=%d Fonts=%d NodeCount=%d", len(asset.Meshes), len(asset.Fonts), len(asset.Nodes))
			}
			require.Error(t, err, "Pipeline should error due to MaxFileSize limit")
			assert.Contains(t, err.Error(), "size", "error should denote file size limit")
		})
	}
}
