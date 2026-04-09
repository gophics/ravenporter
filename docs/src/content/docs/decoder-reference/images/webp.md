---
title: WebP Decoder
description: Specification support reference for RavenPorter's WebP importer.
---

RavenPorter imports WebP still images from `.webp` files. It handles the standard RIFF-based WebP container used for compressed still images.

## Extensions

`.webp`

## Supported Specification Features

- RIFF / `WEBP` container identification
- Standard still-image WebP payloads

## Unimplemented Runtime-Relevant Features

- Animated WebP carried by `ANIM` and `ANMF` chunks is not imported as animation.

## Out Of Scope For RavenPorter

- ICC profile, Exif, and XMP container metadata are not surfaced in the imported asset.

## Notes

- None.

