---
title: IR Models and Materials Reference
description: Reference for mesh, primitive, mesh-data, morph target, material, texture, and typed material-extension exports.
---

This page covers the geometry- and material-oriented exports from [`ir`](../ir-overview/).

## Geometry Types

| Type | Notes |
| --- | --- |
| `Mesh` | Named geometry container |
| `Primitive` | Sub-mesh with topology and material binding |
| `MeshData` | Struct-of-arrays vertex data |
| `MorphTarget` | Sparse blend-shape deltas |
| `PrimitiveMode` | Triangles, strips, lines, points, and related modes |
| `VertexRemap` | Mapping from new vertices back to originals |

Useful `MeshData` helpers:

- `HasNormals()`
- `HasTangents()`
- `HasUVs()`
- `HasColors()`
- `HasBones()`
- `HasIndices()`

## Material And Texture Types

| Type | Notes |
| --- | --- |
| `Material` | Metallic-roughness PBR plus typed extensions |
| `Texture` | Sampler and image binding metadata |
| `TextureRef` | Texture reference with UV transform and channel selection |
| `TextureFilter` | Nearest or linear |
| `TextureWrap` | Repeat, clamp, or mirror |
| `AlphaMode` | Opaque, mask, or blend |

## Typed Material Extensions

- `MaterialClearcoat`
- `MaterialSheen`
- `MaterialTransmission`
- `MaterialVolume`
- `MaterialIOR`
- `MaterialSpecular`
- `MaterialAnisotropy`
- `MaterialIridescence`
- `MaterialDispersion`
- `MaterialDiffuseTransmission`
- `MaterialSpecularGlossiness`
- `MaterialEmissiveStrength`

For the conceptual model, continue to [Geometry and Materials](../geometry-and-materials/).
