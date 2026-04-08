---
title: IR Asset and Graph Reference
description: Reference for asset, scene, node, transform, graph helpers, and related scene-level IR types.
---

This page covers the scene- and graph-oriented exports from [`ir`](../ir-overview/).

## Core Asset Types

| Type | Notes |
| --- | --- |
| `Asset` | Top-level container for every imported domain |
| `AssetMetadata` | Source format, generator, version, creation time, and extra properties |
| `Scene` | Named scene with `RootNodes` |
| `Node` | Graph node with transform, child indices, and domain references |
| `Transform` | TRS or raw matrix local transform |
| `NoIndex` | Sentinel value `-1` |

## Asset Constructors And Helpers

- `NewAsset(format)`
- `NewAssetWithScene(format, name)`
- `PrimaryScene()`
- `PrimaryRootNodes()`
- `NormalizeGraph()`
- `WorldMatrix()`
- `WalkNodes()`
- `FindNode()`
- `SceneBoundingBox()`
- `TotalVertexCount()`
- `TotalTriangleCount()`

## Related Scene-Level Types

| Type | Notes |
| --- | --- |
| `Camera` | Camera entry referenced from nodes |
| `Light` | Light entry referenced from nodes |
| `LODGroup` / `LODLevel` | LOD metadata referencing scene nodes |
| `CollisionMesh` | Collision representation linked to meshes or nodes |
| `MobilityState` | Static, stationary, or movable |
| `Axis` | `YUp` or `ZUp` |

For the behavioral overview, continue to [Scene Graph and Indexing](../scene-graph-and-indexing/).
