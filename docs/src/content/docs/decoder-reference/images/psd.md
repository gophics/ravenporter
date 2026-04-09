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
- RGB, grayscale, and CMYK source documents
- 8-bit, 16-bit, and 32-bit source depths

## Unimplemented Runtime-Relevant Features

- Bitmap, Indexed, Duotone, Lab, and Multichannel composite color modes are not supported.
- 16-bit and 32-bit composite PSD/PSB image data do not preserve their original source precision end to end.

## Out Of Scope For RavenPorter

- Layer pixel content, adjustment and effect data, vector data, and image-resource blocks are not preserved as editable document structure; RavenPorter imports the flattened composite image plus basic layer-count metadata.

## Notes

- None.

