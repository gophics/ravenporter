---
title: TIFF Decoder
description: Specification support reference for RavenPorter's TIFF importer.
---

RavenPorter imports TIFF still images from `.tiff` and `.tif` files. It covers the common endian variants used for raster source assets.

## Extensions

`.tiff`, `.tif`

## Supported Specification Features

- Little-endian and big-endian TIFF headers
- Standard TIFF image data

## Unimplemented Runtime-Relevant Features

- BigTIFF files using version `43` headers are not supported.

## Out Of Scope For RavenPorter

None.

## Notes

- None.

