package ir

import "sync"

// Font uses composition: exactly one of Vector or Bitmap is non-nil.
type Font struct {
	Name       string
	Format     FontFormat
	Family     string
	Subfamily  string
	PostScript string
	Vector     *VectorFontData
	Bitmap     *BitmapFontData
	Metadata   map[string]string
}

// VectorFontData holds metrics and glyph data for vector fonts.
type VectorFontData struct {
	UnitsPerEm int
	Ascender   int
	Descender  int
	LineGap    int
	GlyphCount int
	Codepoints []rune
	Advances   map[rune]int
	Kerning    []KerningPair
	RawData    []byte // Raw .ttf/.otf bytes needed for rasterization

	rawDataOnce   sync.Once
	rawDataError  error
	rawDataLoader func() ([]byte, error)
}

// HasRawBytes reports whether the font has eager or lazy raw bytes.
func (f *VectorFontData) HasRawBytes() bool {
	return f != nil && (len(f.RawData) != 0 || f.rawDataLoader != nil)
}

// RawBytes returns the raw vector font bytes, materializing them on demand.
func (f *VectorFontData) RawBytes() ([]byte, error) {
	if f == nil {
		return nil, nil
	}
	if len(f.RawData) != 0 {
		return f.RawData, nil
	}
	if f.rawDataLoader == nil {
		return nil, nil
	}
	f.rawDataOnce.Do(func() {
		f.RawData, f.rawDataError = f.rawDataLoader()
	})
	return f.RawData, f.rawDataError
}

// SetRawBytes replaces the raw vector font bytes and clears any lazy loader.
func (f *VectorFontData) SetRawBytes(data []byte) {
	if f == nil {
		return
	}
	f.RawData = data
	f.rawDataLoader = nil
	f.rawDataOnce = sync.Once{}
	f.rawDataError = nil
}

// SetRawBytesLoader installs a lazy raw-byte loader.
func (f *VectorFontData) SetRawBytesLoader(loader func() ([]byte, error)) {
	if f == nil {
		return
	}
	f.RawData = nil
	f.rawDataLoader = loader
	f.rawDataOnce = sync.Once{}
	f.rawDataError = nil
}

type KerningPair struct {
	First  rune
	Second rune
	Amount int
}

// BitmapFontData holds metrics for rasterized bitmap font atlases.
type BitmapFontData struct {
	LineHeight int
	Base       int
	GlyphCount int
	AtlasPath  string
	AtlasIndex int // Index into Asset.Images
	Glyphs     map[rune]BitmapGlyph
}

type BitmapGlyph struct {
	X       int
	Y       int
	Width   int
	Height  int
	XOffset int
	YOffset int
	Advance int
}
