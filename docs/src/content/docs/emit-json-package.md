---
title: emit/json Package
description: Curated reference for RavenPorter's JSON IR emitter.
---

The `emit/json` package provides a JSON emitter for the RavenPorter IR.

## Main API

| Symbol | Purpose |
| --- | --- |
| `WriteTo(asset, writer, pretty)` | Write JSON directly to any `io.Writer` |
| `Emitter` | `emit.Emitter` implementation used by the CLI export path |

## Example

```go
var out bytes.Buffer

if err := jsonir.WriteTo(result.Asset, &out, true); err != nil {
	log.Fatal(err)
}
```

## Notes

- The emitter writes the full `ir.Asset` structure.
- Pretty printing is optional.
- This is an inspection and tooling format, not RavenPorter's runtime cache format.

See [JSON IR and Emitters](../json-ir-and-emitters/) for the workflow context and tradeoffs.
