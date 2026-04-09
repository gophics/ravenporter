---
title: PNG Decoder
description: Specification support reference for RavenPorter's PNG importer.
---

RavenPorter imports PNG images from `.png` files. It covers standard raster PNG data plus animated PNG chunks used in APNG assets.

## Extensions

`.png`

## Supported Specification Features

- `IHDR` dimension parsing
- Standard PNG image data
- APNG animation chunks `acTL`, `fcTL`, and `fdAT`

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Ancillary PNG metadata such as ICC profiles, text chunks, physical-resolution data, and Exif is not surfaced in the imported asset.
- APNG frame-control operations are flattened into composed RGBA frames rather than preserved as original dispose and blend operations.

## Notes

- None.

