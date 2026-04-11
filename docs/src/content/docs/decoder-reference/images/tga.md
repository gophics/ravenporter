---
title: TGA Decoder
description: Specification support reference for RavenPorter's TGA importer.
---

RavenPorter imports Truevision TGA images from `.tga` files. It handles the truecolor, grayscale, indexed, orientation, and RLE-compressed variants that show up in older texture pipelines.

## Extensions

`.tga`

## Supported Specification Features

- Image types `1`, `2`, `3`, `9`, `10`, and `11`
- 8-bit indexed pixels
- 8-bit grayscale pixels and 16-bit grayscale-plus-alpha pixels
- 16-bit, 24-bit, and 32-bit truecolor pixels
- Vertical and right-to-left scanline origin handling
- Uncompressed RGB and grayscale images
- RLE-compressed RGB, grayscale, and indexed images
- Indexed and color-mapped images with 16-bit, 24-bit, and 32-bit palette entries

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Legacy TGA image types `32` and `33` that use Huffman, delta, and quadtree-style compression are not supported.
- Indexed TGA variants with pixel indices wider than 8 bits are not imported.

## Notes

None.

