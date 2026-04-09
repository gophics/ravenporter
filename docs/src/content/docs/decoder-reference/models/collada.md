---
title: COLLADA Decoder
description: Specification support reference for RavenPorter's COLLADA importer.
---

RavenPorter imports COLLADA scene documents from `.dae` files. It covers the core geometry, material, transform, camera, light, and animation features used in COLLADA 1.4.1 assets.

## Extensions

`.dae`

## Supported Specification Features

- Geometry primitives from `triangles`, `polylist`, `polygons`, `lines`, `linestrips`, `tristrips`, and `trifans`
- Material models based on `phong`, `lambert`, and `blinn`
- Image libraries and texture references
- Visual scene hierarchies and 4 x 4 node transforms
- Skeletal animation, matrix animation decomposition, joint weights, and morph targets
- Cameras, lights, and up-axis metadata for `Y_UP` and `Z_UP`
- Multiple UV sets via the `set` attribute

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- COLLADA physics and kinematics libraries are not imported.
- Only `profile_COMMON` effects are mapped; other COLLADA effect and shader profiles are not supported.

## Notes

- None.

