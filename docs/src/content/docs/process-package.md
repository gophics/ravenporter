---
title: process Package
description: Curated reference for RavenPorter's built-in post-processing catalog, presets, flags, and process options.
---

The `process` package exposes RavenPorter's built-in post-import conditioning catalog.

## Main API

| Function | Purpose |
| --- | --- |
| `Apply(asset, flags, opts)` | Run flagged steps on an `ir.Asset` |
| `BuiltInSteps()` | Return the built-in step catalog |
| `NewRegistry(steps...)` | Create a custom ordered step registry |
| `(*Registry).Register(step)` | Add one step to a registry |
| `(*Registry).RegisterAll(steps...)` | Add multiple steps to a registry |
| `(*Registry).Steps()` | Return the ordered registered step list |

## Core Types

| Type | Purpose |
| --- | --- |
| `PPFlag` | Bitmask of enabled process steps |
| `Options` | Per-step configuration such as scale, target axis, and limits |
| `Step` | Individual processing step interface |
| `Registry` | Ordered set of steps |
| `ComponentFlag` | Vertex-attribute removal flags |
| `DegenerateMode` | Behavior for degenerate-triangle handling |

## Presets

- `PresetFast`
- `PresetQuality`
- `PresetMaxQuality`

The root-package preset names map to these bitmasks.

## Common Flags

- `PPTriangulate`
- `PPGenNormals`
- `PPGenSmoothNormals`
- `PPCalcTangentSpace`
- `PPJoinIdenticalVertices`
- `PPRemoveDegenerates`
- `PPRemoveComponent`
- `PPFindInstances`
- `PPOptimizeMeshes`
- `PPLimitBoneWeights`
- `PPSplitLargeMeshes`
- `PPSplitByBoneCount`
- `PPGlobalScale`
- `PPFixUpAxis`
- `PPEmbedTextures`
- `PPDebone`
- `PPGenBoundingBoxes`
- `PPDecodePixels`
- `PPDecodeSamples`
- `PPResizeImages`
- `PPGenerateMipmaps`
- `PPResampleAudio`
- `PPMixdownAudio`
- `PPNormalizeAudio`
- `PPTrimAudio`
- `PPGenerateFontAtlas`

## Important Options Fields

`Options` includes:

- `Logger`
- `SmoothNormalAngle`
- `MaxBoneWeights`
- `MaxVerticesPerMesh`
- `MaxBonesPerMesh`
- `MaxTextureSize`
- `AtlasFontSize`
- `GlobalScale`
- `TargetUpAxis`
- `RemoveFlags`
- `AssetDir`
- `AssetFS`
- `TargetSampleRate`
- `TargetChannels`
- `DegenerateMode`
- `DeboneThreshold`

Some flags only become meaningful when the matching option field is populated, such as `PPRemoveComponent` with `RemoveFlags` or `PPSplitLargeMeshes` with `MaxVerticesPerMesh`.

## Example

```go
err := process.Apply(
	asset,
	process.PPGlobalScale|process.PPFixUpAxis,
	process.Options{
		GlobalScale:  0.01,
		TargetUpAxis: ir.YUp,
	},
)
```

The registry normalizes the graph before and after each applied step. That makes direct `process.Apply()` safe for manually assembled assets too.
