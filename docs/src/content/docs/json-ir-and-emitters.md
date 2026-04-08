---
title: JSON IR and Emitters
description: RavenPorter's JSON emitter is best used for inspection, debugging, and tooling handoff, not for runtime packaging.
---

RavenPorter ships a JSON emitter in [`emit/json`](../emit-json-package/). It writes the full in-memory IR in a form that is easy to inspect, diff, and hand to external tooling.

## When JSON Is The Right Tool

- You want to inspect the imported asset shape directly.
- You are debugging decoder or processing behavior.
- You need a simple interchange artifact for tooling that understands the RavenPorter IR model.

## When It Is Not

- You need a runtime format with lazy media and strict validation.
- You want a compact shipping artifact.
- You need behavior beyond what JSON can represent efficiently.

For those jobs, [`cache`](../runtime-cache/) is the better fit.

## Write JSON To Any Writer

```go
package main

import (
	"bytes"
	"context"
	"log"

	"github.com/gophics/ravenporter"
	jsonir "github.com/gophics/ravenporter/emit/json"
)

func main() {
	result, err := ravenporter.ImportPath(context.Background(), "assets/scene.glb")
	if err != nil {
		log.Fatal(err)
	}

	var out bytes.Buffer
	if err := jsonir.WriteTo(result.Asset, &out, true); err != nil {
		log.Fatal(err)
	}

	log.Print(out.String())
}
```

## The CLI Uses The Same Emitter

The CLI commands `import`, `export`, `batch`, and `convert` all go through the same `Emitter` type.

```go
emitter := &jsonir.Emitter{}
err := emitter.Emit(asset, outFS, emit.Options{
	BaseName:    "scene",
	PrettyPrint: true,
})
```

## Output Characteristics

- The emitter writes the full `ir.Asset` as JSON.
- Pretty printing is optional.
- It does not apply cache-specific validation or lazy media behavior.
- The JSON shape mirrors the Go IR closely, so it works best as an inspection format rather than as a stable public interchange contract.
