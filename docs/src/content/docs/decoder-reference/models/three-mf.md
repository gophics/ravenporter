---
title: 3MF Decoder
description: Specification support reference for RavenPorter's 3MF importer.
---

RavenPorter imports 3D Manufacturing Format packages from `.3mf` files. It covers mesh geometry, material/color assignments, textured surfaces, units, and multi-object scenes.

## Extensions

`.3mf`

## Supported Specification Features

- Mesh geometry with vertex and triangle sets
- `BaseMaterials` color assignments
- Vertex color groups
- UV-mapped `Texture2D` textures
- Triangle and object property IDs for materials, colors, and texture groups
- Scene units including microns, millimeters, centimeters, inches, feet, and meters
- Multi-object scenes

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- 3MF extension content such as Beam Lattice, Production, Slice, Volumetric, and Secure Content is not imported.
- Materials and Properties extension features beyond `BaseMaterials`, color groups, and `Texture2D` / `Texture2DGroup` resources are not imported.

## Notes

- None.

