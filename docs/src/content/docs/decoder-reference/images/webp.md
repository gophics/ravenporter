---
title: WebP Decoder
description: Specification support reference for RavenPorter's WebP importer.
---

RavenPorter imports WebP images from `.webp` files. It handles the standard RIFF-based WebP container for both still-image payloads and composed animation frames.

## Extensions

`.webp`

## Supported Specification Features

- RIFF / `WEBP` container identification
- Standard still-image WebP payloads
- Animated WebP carried by `ANIM` and `ANMF` chunks, imported as composed RGBA frame images with per-frame delay metadata

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- ICC profile, Exif, and XMP container metadata are not surfaced in the imported asset.
- Original WebP frame disposal and blend operations are flattened into composed RGBA frames rather than preserved as authored animation state.

## Notes

None.

