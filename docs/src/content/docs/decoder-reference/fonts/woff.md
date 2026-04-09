---
title: WOFF Decoder
description: Specification support reference for RavenPorter's WOFF importer.
---

RavenPorter imports WOFF 1.0 web fonts from `.woff` files. It covers the table-directory model, compressed font tables, and SFNT reconstruction needed to read the common metadata tables.

## Extensions

`.woff`

## Supported Specification Features

- WOFF 1.0 headers and table directory entries
- Zlib-compressed and uncompressed font table storage
- SFNT reconstruction from WOFF payloads
- `name`, `OS/2`, `head`, and `maxp` table extraction

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Extended metadata and private data blocks are not imported.

## Notes

- None.

