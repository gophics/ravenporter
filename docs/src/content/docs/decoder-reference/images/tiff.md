---
title: TIFF Decoder
description: Specification support reference for RavenPorter's TIFF importer.
---

RavenPorter imports TIFF still images from `.tiff` and `.tif` files. It covers classic TIFF and BigTIFF containers for raster image assets.

## Extensions

`.tiff`, `.tif`

## Supported Specification Features

- Little-endian and big-endian TIFF headers
- BigTIFF files using version `43` headers when their offsets and byte counts fit within RavenPorter's stream limits
- Standard TIFF image data

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

None.

## Notes

None.

