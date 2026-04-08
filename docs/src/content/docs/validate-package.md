---
title: validate Package
description: Curated reference for structural and semantic validation passes over ir.Asset, with the current checks centered on scene and model content.
---

The `validate` package runs integrity and logical checks over `ir.Asset`.

Today those checks are primarily scene and model oriented: meshes, node graphs, materials, texture references, and animations. Images, audio, and fonts mostly rely on import-time decode failures rather than dedicated `validate` rules.

## Main Functions

| Function | Purpose |
| --- | --- |
| `Structural(asset)` | Validate pre-processing structural integrity |
| `Semantic(asset)` | Validate post-processing logical correctness |
| `Asset(asset)` | Run both passes and merge the result |

## Result Type

`Result` contains:

- `Errors`
- `Warnings`

Use `Result.OK()` to check whether any errors were found.

## Typical Use

```go
result := validate.Asset(asset)
if !result.OK() {
	for _, issue := range result.Errors {
		log.Printf("%s: %s", issue.Code, issue.Message)
	}
}
```

## Common Checks

- node and texture reference bounds
- cyclic graphs
- attribute length mismatches
- invalid indices
- NaN or Inf positions
- orphan materials
- out-of-range PBR values
- animation NaNs

See [Error and Issue Reference](../error-and-issue-reference/) for the stable code list.
