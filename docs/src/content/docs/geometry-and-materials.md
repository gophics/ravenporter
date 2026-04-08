---
title: Geometry and Materials
description: How RavenPorter stores meshes, primitives, struct-of-arrays mesh data, morph targets, materials, textures, and typed PBR extensions.
---

Geometry and materials are represented with flat slices plus per-primitive and per-texture indices.

## Meshes And Primitives

Each `Mesh` contains one or more `Primitive` entries.

| Type | Meaning |
| --- | --- |
| `Mesh` | Named geometry container with `Primitives`, `MorphWeights`, and `BoundingBox` |
| `Primitive` | One topology and one material binding |
| `Primitive.Mode` | Topology such as `Triangles`, `Lines`, or `Points` |
| `Primitive.MaterialIndex` | Index into `Asset.Materials` or `NoIndex` |

## Struct-Of-Arrays MeshData

`MeshData` stores vertex attributes in a struct-of-arrays layout.

Always-present fields:

- `VertexCount`
- `Positions`
- `Indices`

Optional fields include:

- `Normals`
- `Tangents`
- `TexCoord0` through `TexCoord3`
- `Colors0`
- `Joints0`, `Joints1`
- `Weights0`, `Weights1`
- `SmoothGroups`

This layout keeps the representation predictable for validation, processing, JSON emission, and cache serialization.

## Morph Targets

`Primitive.MorphTargets` stores sparse per-vertex deltas:

- `Indices` identifies the affected base vertices
- `Positions`, `Normals`, and `Tangents` store the target deltas

`Mesh.MorphWeights` and `Node.MorphWeights` hold default or instance-level morph weights.

## Materials

`Material` stores metallic-roughness PBR properties plus optional typed extensions.

Base material fields include:

- `BaseColorFactor`
- `MetallicFactor`
- `RoughnessFactor`
- `NormalTexture`
- `OcclusionTexture`
- `EmissiveFactor`
- `AlphaMode`
- `DoubleSided`
- `Unlit`

## Textures And Images

`Texture` is metadata and sampling behavior. `ImageAsset` is the underlying image payload. The two are linked through `Texture.ImageIndex`.

`TextureRef` is how materials bind a texture:

- `TextureIndex`
- `UVSet`
- `Offset`
- `Tiling`
- `Rotation`
- `Channel`

## Typed Material Extensions

RavenPorter exposes typed fields for standard material extensions instead of leaving everything in an untyped property map. Examples include:

- `Clearcoat`
- `Sheen`
- `Transmission`
- `Volume`
- `IOR`
- `Specular`
- `Anisotropy`
- `Iridescence`
- `Dispersion`
- `DiffuseTransmission`
- `SpecularGlossiness`
- `EmissiveStrength`

For the concrete exported type list, open [IR Models and Materials Reference](../ir-models-materials/).
