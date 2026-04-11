---
title: OpenType (OTF) Decoder
description: Specification support reference for RavenPorter's OpenType metadata importer.
---

RavenPorter imports OpenType metadata from standalone `.otf` files and OpenType collections in `.otc` containers. It covers the SFNT header and the core tables RavenPorter reads to identify font families, metrics, glyph counts, and Unicode coverage.

## Extensions

`.otf`, `.otc`

## Supported Specification Features

- `OTTO` SFNT header parsing
- OpenType Collection (`ttcf`) member enumeration with one imported font per collection entry
- `name` table metadata including family, subfamily, and PostScript names
- `OS/2` metrics including ascender, descender, and line gap
- `head` table metrics such as units per em
- `maxp` glyph count metadata
- Unicode `cmap` table extraction for format `4` and format `12` subtables
- Additional text metadata such as copyright, trademark, manufacturer, and designer fields

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- CFF and CFF2 outline programs are not interpreted beyond raw font-data preservation.
- OpenType layout, variation, color, and SVG font tables are not surfaced.

## Notes

None.

