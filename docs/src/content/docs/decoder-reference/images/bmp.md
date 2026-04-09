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
- 16-bit bitfield or RGB555-style bitmap variants
- 24-bit RGB and 32-bit RGBA bitmap variants
- Bottom-up and top-down scanline orientation
- `BI_RGB` uncompressed pixel storage
- `BI_BITFIELDS` channel masks
- `RLE4` and `RLE8` compressed images

## Unimplemented Runtime-Relevant Features

- BMP bit depths outside the imported `1`, `4`, `8`, `16`, `24`, and `32` bits-per-pixel variants are not supported.
- Compression types beyond `BI_RGB`, `BI_BITFIELDS`, `RLE4`, and `RLE8`, such as embedded `BI_JPEG`, `BI_PNG`, and CMYK variants, are not supported.

## Out Of Scope For RavenPorter

None.

## Notes

- None.

