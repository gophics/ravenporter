---
title: cache Package
description: Curated reference for writing, validating, and reading RavenPorter cooked runtime assets.
---

The [`cache`](../runtime-cache/) package turns a `*ravenporter.Result` into a cooked runtime asset and can reopen that asset later.

## Main Functions

| Function | Purpose |
| --- | --- |
| `Write(w, result, options...)` | Serialize a cooked asset |
| `Validate(result)` | Check whether a result is cache-safe before writing |
| `ParseImagePixelsMode(value)` | Parse the supported pixel-persistence mode strings |
| `Open(path, options...)` | Open a cooked asset from disk |
| `OpenFS(fsys, path, options...)` | Open from `fs.FS` |
| `Read(readerAt, size, options...)` | Open from an existing `io.ReaderAt` |

## Main Types

| Type | Purpose |
| --- | --- |
| `Asset` | Cooked runtime asset containing `Manifest` and embedded `*ir.Asset` |
| `Manifest` | Cache metadata, source profile, dependencies, notes, and summary |
| `ImagePixelsMode` | Controls decoded image pixel persistence during write |

## Write Options

- `WithImagePixels(mode)`
- `WithMaxEmbeddedMediaBytes(limit)`

`ImagePixelsMode` values:

- `ImagePixelsNever`
- `ImagePixelsIfPresent`
- `ImagePixelsAlways`

## Read Options

- `WithEagerMedia()`

## Example

```go
var buf bytes.Buffer

if err := cache.Write(
	&buf,
	result,
	cache.WithImagePixels(cache.ImagePixelsIfPresent),
); err != nil {
	log.Fatal(err)
}

pkg, err := cache.Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
if err != nil {
	log.Fatal(err)
}
defer pkg.Close()
```

## Important Constraints

- `Write()` rejects external texture file references.
- Material property maps are validated against a narrow supported type set.
- Lazy media means `Close()` matters when you open cooked assets from reader-backed storage.

See [Cache Format Model](../cache-format-model/) for the internal chunk layout.
