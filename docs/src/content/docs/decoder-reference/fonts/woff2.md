---
title: WOFF2 Decoder
description: Specification support reference for RavenPorter's WOFF2 importer.
---

RavenPorter imports WOFF 2.0 web fonts from `.woff2` files. It covers the compact table-directory model, custom and known table tags, Brotli-compressed payloads, and SFNT reconstruction.

## Extensions

`.woff2`

## Supported Specification Features

- WOFF2 headers and signature validation
- Known-tag and custom-tag table directory entries
- `UIntBase128`-encoded values
- Brotli-compressed table payloads
- SFNT reconstruction from WOFF2 payloads
- WOFF2 collection-directory wrappers with one imported font per collection member
- `name`, `OS/2`, `head`, and `maxp` table extraction

## Unimplemented Runtime-Relevant Features

- WOFF2 files that rely on transformed `glyf` and `loca` table reconstruction are not fully supported.

## Out Of Scope For RavenPorter

- Extended metadata and private data blocks are not imported.

## Notes

- None.

