---
title: DDS Decoder
description: Specification support reference for RavenPorter's DDS importer.
---

RavenPorter imports DirectDraw Surface textures from `.dds` files. It handles the compressed texture families and container metadata used by runtime-ready GPU texture assets.

## Extensions

`.dds`

## Supported Specification Features

- `DXT1` / `BC1`, `DXT3` / `BC2`, and `DXT5` / `BC3`
- `ATI1` / `BC4U` and `ATI2` / `BC5U`
- `DX10` extended headers
- `BC6H` and `BC7` DXGI formats
- Mipmap count metadata
- 2D, 3D, cube, 2D array, and cube-array topology classification from legacy DDS headers and `DDS_HEADER_DXT10`
- Uncompressed DDS pixel formats

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

None.

## Notes

- None.

