---
title: ravenporter Package
description: Curated reference for the root package, including import entrypoints, option helpers, profile APIs, and built-in format discovery helpers.
---

The root package is the intended public API for RavenPorter.

## Main Entry Points

| Function | Purpose |
| --- | --- |
| `ImportPath()` | Import one local filesystem asset |
| `ImportFS()` | Import one asset from `fs.FS` |
| `ImportReader()` | Import one asset from a `ReadSeekerAt` |
| `ImportBytes()` | Import one in-memory asset |
| `ImportDir()` | Import every supported asset under a local directory |
| `ImportFSDir()` | Import every supported asset under an `fs.FS` directory |

## Result Types

The package re-exports the main pipeline types:

- `Result`
- `Report`
- `SourceReport`
- `Issue`
- `Dependency`
- `AssetSummary`
- `Profile`
- `DecodeProfile`
- `ProcessProfile`

It also exposes these support types directly:

- `Option`
- `LoadMask`
- `ReadSeekerAt`

## Option Helpers

The most commonly used options are:

- `WithPreset()`
- `WithProfile()`
- `WithProfileFile()`
- `WithDecodeMaxFileSize()`
- `WithDecodeMaxVertices()`
- `WithDecodeMaxImagePixels()`
- `WithDecodeMaxAudioSamples()`
- `WithGlobalScale()`
- `WithTargetUpAxis()`
- `WithEmbedTextures()`
- `WithBatchConcurrency()`
- `WithLoadMask()`
- `WithRegistry()`
- `WithLogger()`
- `WithProcessFlags()`

## Preset And Load-Mask Constants

The root package also exports stable constants for the common string and mask values:

- `ProfileVersion`
- `BuiltInPresetFast`
- `BuiltInPresetQuality`
- `BuiltInPresetMaxQuality`
- `LoadMeshes`
- `LoadMaterials`
- `LoadTextures`
- `LoadAnimations`
- `LoadSkeletons`
- `LoadCameras`
- `LoadLights`
- `LoadAudio`
- `LoadFonts`
- `LoadImages`
- `LoadAll`

## Profile Helpers

| Function | Purpose |
| --- | --- |
| `LoadProfile()` | Read a TOML profile from disk |
| `SaveProfile()` | Write a TOML profile |
| `ParseProfileTOML()` | Parse profile bytes directly |
| `ResolveProfile()` | Convert options into the effective serializable profile |
| `BuiltInPresetNames()` | Return `fast`, `quality`, and `max-quality` |

## Built-In Format Discovery

| Function | Purpose |
| --- | --- |
| `NewRegistry()` | Return a fresh registry with all built-in decoders |
| `SupportedFormats()` | Return built-in format IDs |
| `SupportedExtensions()` | Return built-in file extensions |
| `SupportsExtension()` | Test one extension against the built-in catalog |

## Notes

- `WithLoadMask()` reshapes the returned asset after processing, so it is not part of the TOML profile schema.
- `WithProcessFlags()` replaces the process mask directly and also bypasses profile serialization.

## Example

```go
result, err := ravenporter.ImportPath(
	context.Background(),
	"assets/scene.glb",
	ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
	ravenporter.WithEmbedTextures(),
)
if err != nil {
	log.Fatal(err)
}

log.Printf("meshes=%d issues=%d", len(result.Asset.Meshes), len(result.Report.Issues))
```

If you want the shortest path into the library, start with [Quick Start](../quick-start/). If you need lower-level hooks, continue to [`detect`](../detect-package/) or [`process`](../process-package/).
