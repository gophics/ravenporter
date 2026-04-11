---
title: BMP Decoder
description: Specification support reference for RavenPorter's BMP importer.
---

RavenPorter imports Windows Bitmap images from `.bmp` files. It covers the common truecolor, indexed, bitfield, and RLE-compressed variants used in legacy asset pipelines.

## Extensions

`.bmp`

## Supported Specification Features

- `BITMAPCOREHEADER` and modern DIB header variants
- 1-bit, 4-bit, and 8-bit indexed palette images
- 15-bit RGB555-style and 16-bit bitfield bitmap variants
- 24-bit RGB, 32-bit RGBA, and 64-bit RGBA bitmap variants
- Bottom-up and top-down scanline orientation
- `BI_RGB` uncompressed pixel storage
- `BI_BITFIELDS` and `BI_ALPHABITFIELDS` channel masks
- `RLE4` and `RLE8` compressed images

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Embedded `BI_JPEG` and `BI_PNG` payloads are not supported.
- CMYK-family BMP compression modes are not supported.
- Obscure legacy bit depths outside the imported `1`, `4`, `8`, `15`, `16`, `24`, `32`, and `64` bits-per-pixel variants are not a target for this runtime importer.

## Notes

- 64-bit BMP pixels are converted into 8-bit RGBA images during import.

