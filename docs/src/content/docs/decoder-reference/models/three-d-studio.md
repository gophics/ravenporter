---
title: 3D Studio (3DS) Decoder
description: Specification support reference for RavenPorter's 3DS importer.
---

RavenPorter imports Autodesk 3D Studio scene data from binary `.3ds` files. It covers geometry, materials, cameras, lights, hierarchy, and keyframe animation tracks.

## Extensions

`.3ds`

## Supported Specification Features

- Binary 3DS geometry with vertices, faces, UVs, and local 3 x 4 transforms
- Material properties for diffuse, specular, and ambient shading
- Texture map slots for diffuse, bump/normal, specular, opacity, reflection, and emissive data
- Point, directional, and spot lights plus perspective cameras
- Mesh-node keyframe animation tracks for translation, rotation, and scale
- Face-material assignments and hierarchical scene nodes

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Tension, continuity, and bias spline parameters are skipped; imported 3DS animation channels use sampled key values with linear interpolation.
- Camera and light track nodes contribute hierarchy only; animated camera and light transforms are not imported.
- Pivot data and other keyframer chunks beyond hierarchy plus translation, rotation, and scale tracks are not imported.

## Notes

None.

