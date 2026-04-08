---
title: IR Overview
description: Understand ir.Asset as RavenPorter's flat, runtime-oriented container for scenes, geometry, materials, media, animation, and metadata.
---

`ir.Asset` is RavenPorter's central internal representation. Every built-in decoder and every public import entrypoint ends up here.

## Design Goals

- Keep imported content in one flat, inspectable container.
- Use integer indices instead of pointer graphs for most cross-references.
- Make post-processing and serialization predictable.
- Avoid holding OS resources after import completes.

## Top-Level Shape

An `ir.Asset` contains slices for each domain:

- `Scenes`
- `Nodes`
- `Meshes`
- `Materials`
- `Textures`
- `Images`
- `Animations`
- `Skeletons`
- `Cameras`
- `Lights`
- `AudioClips`
- `Fonts`
- `LODGroups`
- `CollisionMeshes`

It also stores:

- `Name`
- `UpAxis`
- `Unit`
- `DefaultScene`
- `RootNodes`
- `Metadata`

## Flat Container, Indexed Relationships

The IR favors cross-indices such as:

- `Node.MeshIndex`
- `Node.SkinIndex`
- `Primitive.MaterialIndex`
- `Texture.ImageIndex`
- `AnimationChannel.NodeIndex`
- `BitmapFontData.AtlasIndex`

That makes the IR easier to serialize, cache, diff, and traverse in tooling.

## Lifecycle

- `NewAsset(format)` creates an empty asset with source-format metadata initialized.
- `NewAssetWithScene(format, name)` creates an asset plus a primary scene entry.
- `Asset.Close()` is a no-op because the in-memory IR itself owns no external resources.

After import returns, the asset is just Go data on the heap. Decoder-side memory mappings are released during decode.

## Common Helpers

`ir.Asset` also provides convenience helpers such as:

- `PrimaryScene()`
- `PrimaryRootNodes()`
- `NormalizeGraph()`
- `WorldMatrix()`
- `WalkNodes()`
- `FindNode()`, `FindMesh()`, `FindMaterial()`, `FindAnimation()`, and related lookup helpers
- `TotalVertexCount()`
- `TotalTriangleCount()`
- `SceneBoundingBox()`

If you want the graph model, continue to [Scene Graph and Indexing](../scene-graph-and-indexing/). If you want the concrete exported types, open [IR Asset and Graph Reference](../ir-asset-and-graph/).
