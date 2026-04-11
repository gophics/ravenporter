---
title: Photoshop Decoder
description: Specification support reference for RavenPorter's PSD and PSB importer.
---

RavenPorter imports Adobe Photoshop documents from `.psd` and `.psb` files. It covers composite-image decode, basic layer metadata, common color modes, and the standard compression variants used for flattened image data.

## Extensions

`.psd`, `.psb`

## Supported Specification Features

- File headers plus document dimensions, channel counts, bit depth, and color mode metadata
- PSD and PSB container variants for the flattened composite-image path
- Layer and mask section metadata. It includes layer counts
- Composite image decode for raw, PackBits RLE, ZIP, and ZIP-with-prediction data
- Bitmap, grayscale, indexed, duotone, RGB, CMYK, Lab, and multichannel source documents on the flattened composite path
- 8-bit, 16-bit, and 32-bit source depths

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Layer pixel content, adjustment and effect data, vector data, and image-resource blocks are not preserved as editable document structure; RavenPorter imports the flattened composite image plus basic layer-count metadata.

## Notes

None.

