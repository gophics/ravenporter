---
title: FBX Decoder
description: Specification support reference for RavenPorter's FBX importer.
---

RavenPorter imports Autodesk FBX scenes from binary and ASCII `.fbx` files. It handles the mesh, material, texture, hierarchy, and animation features most DCC exports rely on.

## Extensions

`.fbx`

## Supported Specification Features

- Binary FBX (`v7400`, `v7500+`) and ASCII FBX scene files
- Mesh geometry, polygon triangulation, normals, binormals, UV sets, vertex colors, and tangents
- Materials plus diffuse, emissive, normal, ambient, and specular texture bindings
- Model hierarchies, cameras, lights, and `GlobalSettings` axis/unit metadata
- Skeletal skinning with clusters and inverse bind matrices, keyframe animation on translation/rotation/scale channels, blend shapes, and multiple animation stacks/takes
- Embedded textures and zlib-compressed array payloads
- Smoothing groups mapped into imported normal splits

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

None.

## Notes

None.

