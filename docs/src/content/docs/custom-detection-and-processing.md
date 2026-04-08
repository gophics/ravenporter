---
title: Custom Detection and Processing
description: Extend RavenPorter with a custom detect.Registry or apply the built-in processing catalog directly.
---

Most users can stay on the root package. If you need to extend or compose the import pipeline more directly, the two public hooks are [`detect`](../detect-package/) and [`process`](../process-package/).

## Custom Decoder Registry

The root package lets you replace the default decoder catalog with `WithRegistry()`.

```go
package main

import (
	"bytes"
	"context"
	"io"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

type customDecoder struct{}

func (d *customDecoder) Probe(io.ReadSeeker) bool { return true }
func (d *customDecoder) Extensions() []string     { return []string{".mesh"} }
func (d *customDecoder) FormatName() string       { return "custom-mesh" }

func (d *customDecoder) Decode(_ detect.ReadSeekerAt, _ detect.DecodeOptions) (*ir.Asset, error) {
	asset, _ := ir.NewAssetWithScene(ir.FormatID("custom-mesh"), "Root")
	return asset, nil
}

func main() {
	registry := detect.NewRegistry(
		detect.Registration{
			Format:  ir.FormatID("custom-mesh"),
			Decoder: &customDecoder{},
		},
	)

	_, _ = ravenporter.ImportReader(
		context.Background(),
		bytes.NewReader([]byte("placeholder")),
		"asset.mesh",
		ravenporter.WithRegistry(registry),
	)
}
```

## Decoder Contracts

Your decoder implements:

- `Probe(io.ReadSeeker) bool`
- `Decode(ReadSeekerAt, DecodeOptions) (*ir.Asset, error)`
- `Extensions() []string`
- `FormatName() string`

`DecodeOptions` also carries:

- the `Context`
- a filesystem adapter for dependent files
- a provenance/dependency reporter
- decode safeguards such as max file size and vertex/pixel/sample limits

## Applying Process Steps Directly

The processing catalog is public and can be applied independently of the import pipeline.

```go
result, err := ravenporter.ImportPath(context.Background(), "assets/scene.glb")
if err != nil {
	log.Fatal(err)
}

err = process.Apply(
	result.Asset,
	process.PPEmbedTextures|process.PPGenBoundingBoxes,
	process.Options{},
)
if err != nil {
	log.Fatal(err)
}
```

`process.Apply()` normalizes the graph before and after each flagged step, so parent links and root lists stay coherent.

## Presets And Canonical Step Names

- The root package presets map to `process.PresetFast`, `process.PresetQuality`, and `process.PresetMaxQuality`.
- The canonical step names used by CLI flags and TOML profiles are the built-in RavenPorter pipeline names listed by `ravenporter steps`.

## Practical Guidance

- `WithRegistry()` is the right hook when you need new source formats or different detection behavior.
- `process.Apply()` makes sense when you are constructing or mutating `ir.Asset` yourself.
- Stay on the root package if you only need the normal import workflows, profiles, and cache cooking.
