---
title: EXR Decoder
description: Specification support reference for RavenPorter's OpenEXR importer.
---

RavenPorter imports OpenEXR images from `.exr` files. It covers the core scanline and tiled image model, common channel metadata, and the most widely used compression schemes.

## Extensions

`.exr`

## Supported Specification Features

- Image dimensions from `dataWindow` plus general attribute parsing
- Channel metadata, channel counts, and pixel types including half-float and float32
- Scanline and tiled image layouts. It includes tile size metadata
- Compression modes `NONE`, `RLE`, `ZIPS`, `ZIP`, `PIZ`, and `B44`
- Scanline chunk iteration and tiled chunk decode paths

## Unimplemented Runtime-Relevant Features

- `PXR24`, `B44A`, `DWAA`, `DWAB`, and `HTJ2K256` / `HTJ2K32` compression are not supported.
- Multipart and deep EXR files are detected but not fully imported.

## Out Of Scope For RavenPorter

None.

## Notes

- None.

