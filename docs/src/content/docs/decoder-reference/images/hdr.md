---
title: Radiance HDR Decoder
description: Specification support reference for RavenPorter's Radiance HDR importer.
---

RavenPorter imports Radiance HDR images from `.hdr` files. It covers the standard header layout and the scanline encodings commonly found in HDR environment maps and light probes.

## Extensions

`.hdr`

## Supported Specification Features

- `#?RADIANCE` and `#?RGBE` header probing
- Resolution lines such as `-Y <height> +X <width>`
- Old-style scanline data
- New-style adaptive RLE scanlines
- RGBE and XYZE pixel expansion into linear float RGB data

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Header variables such as `FORMAT=`, `EXPOSURE=`, `GAMMA`, and view or color-correction lines are not surfaced or applied.

## Notes

- None.

