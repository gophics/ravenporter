---
title: Scene Graph and Indexing
description: How scenes, root nodes, parent-child links, NoIndex, NormalizeGraph, and traversal helpers work in RavenPorter.
---

RavenPorter stores scene structure in a flat node slice plus integer links.

## Core Types

| Type | Role |
| --- | --- |
| `Scene` | Names a scene entry and lists its `RootNodes` |
| `Node` | Stores transform, visibility, child links, and cross-indices to mesh, skin, camera, light, and LOD data |
| `NoIndex` | Sentinel value `-1` for optional index fields |

## Root Lists

There are two root concepts:

- `Asset.RootNodes`
- `Scene.RootNodes`

`NormalizeGraph()` keeps these primary root lists consistent. If the asset has scenes, the primary scene and `Asset.RootNodes` are synchronized. If there are no scenes, RavenPorter derives roots from nodes whose `ParentIndex` is `NoIndex`.

## Why NormalizeGraph Exists

Decoders and processors may produce partial or inconsistent parent/root information while mutating the asset. `NormalizeGraph()`:

- sanitizes node indices
- removes duplicates and invalid references
- rebuilds `ParentIndex` from child links
- preserves explicit root lists when present
- derives root nodes when they are missing

If you construct assets manually, call `asset.NormalizeGraph()` before advanced traversal or processing.

## Traversal Helpers

### Walk A Scene

```go
asset.WalkNodes(0, func(idx int, node *ir.Node) bool {
	log.Printf("node[%d] = %s", idx, node.Name)
	return true
})
```

`WalkNodes()` performs a depth-first traversal and stops early if your callback returns `false`.

### Compute World Transforms

Use `WorldMatrix(idx)` when you need a node’s accumulated transform through its parent chain.

### Find By Name

Use `FindNode()`, `FindMesh()`, `FindMaterial()`, `FindAnimation()`, `FindImage()`, and the related helpers when you need the first matching index by name.

## Graph-Related Fields On Node

`Node` includes:

- `Transform`
- `Visible`
- `Mobility`
- `ParentIndex`
- `Children`
- `MeshIndex`
- `SkinIndex`
- `CameraIndex`
- `LightIndex`
- `LODGroupIndex`
- `IsJoint`
- `IsCollision`
- `MorphWeights`
- `Extras`

See [Import Pipeline Architecture](../import-pipeline-architecture/) for where normalization happens during import.
