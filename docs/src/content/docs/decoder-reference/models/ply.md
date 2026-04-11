---
title: PLY Decoder
description: Specification support reference for RavenPorter's PLY importer.
---

RavenPorter imports Stanford PLY geometry from ASCII and binary files. It covers the common vertex, face, and edge properties used in polygon and point-cloud assets.

## Extensions

`.ply`

## Supported Specification Features

- ASCII, binary little-endian, and binary big-endian PLY files
- Vertex positions, normals, RGBA colors, and texture coordinates (`s` / `t`, `texture_u` / `texture_v`)
- Face index list properties with n-gon triangulation during import
- Edge elements and point-cloud-only files
- Numeric property types `char`, `uchar`, `short`, `ushort`, `int`, `uint`, `float`, and `double`

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Header `comment` and `obj_info` records are not imported into RavenPorter metadata.
- Custom PLY elements beyond `vertex`, `face`, and `edge` are not interpreted.

## Notes

None.

