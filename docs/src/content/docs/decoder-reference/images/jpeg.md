---
title: JPEG Decoder
description: Specification support reference for RavenPorter's JPEG importer.
---

RavenPorter imports JPEG still images from `.jpeg` and `.jpg` files. It handles baseline and progressive JPEG files used for source textures and reference images.

## Extensions

`.jpeg`, `.jpg`

## Supported Specification Features

- Baseline and progressive JPEG images
- Standard baseline and progressive JPEG still-image data
- ICC profiles stored in `APP2` segments

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- JPEG application metadata other than ICC profiles, such as JFIF, Exif, XMP, and IPTC blocks, is not imported.

## Notes

- None.

