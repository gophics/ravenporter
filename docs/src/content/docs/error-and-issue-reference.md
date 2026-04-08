---
title: Error and Issue Reference
description: Stable report stages, severities, validation codes, and the public error types used by RavenPorter.
---

RavenPorter surfaces errors in two different ways:

- direct Go errors such as `rperr.DecodeError`
- structured report issues and validation findings with stable `stage`, `severity`, and `code` fields

## Report Stages

| Stage | Meaning |
| --- | --- |
| `detect` | Detection or decoder lookup |
| `decode` | Decoder execution |
| `validate-structural` | Integrity checks before processing |
| `process` | Post-import processing |
| `validate-semantic` | Logical checks after processing |

## Severities

| Severity | Meaning |
| --- | --- |
| `error` | Fatal or serious issue |
| `warning` | Non-fatal issue |
| `info` | Informational note |

## Validation Codes

### Structural

| Code | Meaning |
| --- | --- |
| `NIL_ASSET` | Asset pointer is nil |
| `NIL_MESH` | Mesh entry is nil |
| `EMPTY_MESH` | Primitive has zero vertices |
| `ATTR_LENGTH_MISMATCH` | Optional attribute length does not match `VertexCount` |
| `INDEX_OUT_OF_BOUNDS` | Index buffer references a vertex outside the primitive |
| `NAN_INF_POSITION` | Position data contains NaN or Inf |
| `CYCLIC_GRAPH` | Node graph contains a cycle |
| `NODE_REF_BOUNDS` | Node, scene, or collision references point outside valid ranges |

### Semantic

| Code | Meaning |
| --- | --- |
| `DEGENERATE_TRIANGLE` | Triangle contains duplicate indices |
| `ORPHAN_MATERIAL` | Material is not referenced by any primitive |
| `PBR_OUT_OF_RANGE` | PBR or material-extension scalar values are outside expected ranges |
| `TEXTURE_REF_BOUNDS` | Material or texture references point outside valid ranges |
| `ANIMATION_NAN` | Animation timestamps contain NaN or Inf |

## Public Error Types

### `rperr.DecodeError`

Used to report a format-specific decode failure. It includes:

- `Format`
- `Offset`
- `Message`
- `Cause`

### `rperr.ValidationError`

Used by [`validate`](../validate-package/) to report structural or semantic findings. It includes:

- `Severity`
- `Code`
- `Message`

## Practical Reading Order

1. Check the returned Go error first.
2. If a `Result` exists, inspect `Report.Issues`.
3. Use the stage to locate where the problem happened.
4. Use the code for stable automation and the message for the human explanation.
