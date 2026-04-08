---
title: Animation and Skeletons
description: How RavenPorter stores animation channels, targets, interpolation, skeletons, and node-level joint relationships.
---

Animation data in RavenPorter is node-oriented and index-based.

## Animation

An `Animation` contains:

- `Name`
- `Channels`
- `Duration`

Each `AnimationChannel` targets one property on one node.

## Channel Fields

| Field | Purpose |
| --- | --- |
| `NodeIndex` | Target node in `Asset.Nodes` |
| `Target` | Translation, rotation, scale, morph weights, pointer-based target, material target, camera FOV, or light target |
| `Interpolation` | `linear`, `step`, or `cubic-spline` |
| `Times` | Keyframe timestamps in seconds |
| `Pointer` | JSON Pointer path for `TargetPointer` channels |
| `MaterialIndex` | Target material index for material-color or material-scalar channels |
| `Values` | Generic float payload for pointer-, material-, camera-, or light-style targets |

Specialized fields store strongly typed keyframe payloads when applicable:

- `Translations`
- `Rotations`
- `Scales`
- `Weights`

## Skeletons

A `Skeleton` stores:

- `Name`
- `Joints` as indices into `Asset.Nodes`
- `RootIdx` as the index inside `Joints` for the root joint
- `InverseBindMatrices`

Nodes participating in a skeleton can also carry:

- `IsJoint`
- `SkinIndex`

## Practical Model

- Scene nodes remain the shared target for transforms and skeletal binding.
- Animation channels reference nodes rather than embedding a separate graph structure.
- Skeletons point back into the same node array, keeping the IR flat and serializable.

Pair this page with [Scene Graph and Indexing](../scene-graph-and-indexing/) when you need to reconstruct a full animated hierarchy from the IR.
