---
title: TrueType (TTF) Decoder
description: Specification support reference for RavenPorter's TrueType metadata importer.
---

RavenPorter imports TrueType metadata from standalone `.ttf` files and TrueType collections in `.ttc` containers. It covers the SFNT header and the core tables RavenPorter reads to identify font families, metrics, glyph counts, and Unicode coverage.

## Extensions

`.ttf`, `.ttc`

## Supported Specification Features

- SFNT header parsing
- TrueType Collection (`ttcf`) member enumeration with one imported font per collection entry
- `name` table metadata including family, subfamily, and PostScript names
- `OS/2` metrics including ascender, descender, and line gap
- `head` table metrics such as units per em
- `maxp` glyph count metadata
- Unicode `cmap` table extraction for format `4` and format `12` subtables
- Additional text metadata such as copyright, trademark, manufacturer, and designer fields

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- TrueType glyph outlines, hinting instructions, and rasterization programs are not interpreted.
- OpenType layout, variation, color, and SVG font tables are not surfaced.

## Notes

None.

