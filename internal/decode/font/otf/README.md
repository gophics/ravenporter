# OTF Decoder

OpenType Font decoder with full SFNT table parsing.

## Extensions

`.otf`

## Supported Features

| Feature | Status |
| :--- | :---: |
| SFNT Header Parsing (OTTO magic) | ✅ |
| `name` Table (Family, Subfamily, PostScript) | ✅ |
| `OS/2` Table (Ascender, Descender, LineGap) | ✅ |
| `head` Table (UnitsPerEm) | ✅ |
| `maxp` Table (GlyphCount) | ✅ |
| Metadata Extraction (Copyright, Trademark, Manufacturer, Designer) | ✅ |
