---
title: detect Package
description: Curated reference for RavenPorter's decoder registry, detection flow, and decoder interface contracts.
---

Reach for `detect` when you need to extend RavenPorter with custom format detection or custom decoders.

## Main Types

| Type | Purpose |
| --- | --- |
| `Registry` | Holds decoders keyed by format |
| `Registration` | Declarative `Format` plus `Decoder` pair |
| `Decoder` | Interface implemented by each format decoder |
| `DecodeOptions` | Context, filesystem access, reporter, and decode limits |
| `ReadSeekerAt` | Reader shape accepted by decoders |
| `SeekableFS` | Minimal filesystem interface used for dependent-file access |
| `DecodeReporter` | Collector for dependencies and provenance notes |

## Registry Methods

- `NewRegistry()`
- `Register()`
- `RegisterAll()`
- `Lookup()`
- `Formats()`
- `Extensions()`
- `SupportsExtension()`
- `Detect()`

## Decoder Interface

```go
type Decoder interface {
	Probe(r io.ReadSeeker) bool
	Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error)
	Extensions() []string
	FormatName() string
}
```

## Common Uses

- build a registry containing only selected decoders
- add support for a private asset format
- change detection order for a conflicting file extension

## Example

```go
registry := detect.NewRegistry(
	detect.Registration{
		Format:  ir.FormatID("custom"),
		Decoder: &customDecoder{},
	},
)

result, err := ravenporter.ImportReader(
	context.Background(),
	reader,
	"asset.custom",
	ravenporter.WithRegistry(registry),
)
```

For a fuller workflow example, see [Custom Detection and Processing](../custom-detection-and-processing/).
