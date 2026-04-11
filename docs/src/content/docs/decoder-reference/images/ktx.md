---
title: KTX / KTX2 Decoder
description: Specification support reference for RavenPorter's KTX and KTX2 importer.
---

RavenPorter imports Khronos texture containers from `.ktx` and `.ktx2` files. It covers the container metadata and compressed texture format identifiers used by GPU-oriented texture workflows.

## Extensions

`.ktx`, `.ktx2`

## Supported Specification Features

- KTX1 and KTX2 container headers
- `VkFormat` identification for `BC1` through `BC7`, `ASTC 4x4`, and `ETC2 RGB8`
- GL internal format identification for S3TC, BPTC, `ETC2 RGB8` / `ETC2 RGBA8`, and `ASTC 4x4`
- Compressed vs. uncompressed payload detection
- KTX1 and KTX2 mipmap count metadata
- 2D, 3D, cube, 2D array, and cube-array topology classification from container header fields
- KTX2 `Zstd` and `ZLIB` supercompression inflation

## Unimplemented Runtime-Relevant Features

- KTX2 `BasisLZ` supercompression is not inflated.

## Out Of Scope For RavenPorter

- Data Format Descriptor details and key/value metadata are not imported.

## Notes

None.

