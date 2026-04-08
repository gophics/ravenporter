---
title: Import Workflows
description: Choose the right import entrypoint for local files, fs.FS inputs, in-memory bytes, readers, or directory trees.
---

The root package exposes several import entrypoints, but they all follow the same option model. What changes is where the bytes come from and whether RavenPorter is opening one asset or walking a tree.

## Single-Asset Entry Points

| Function | Best fit |
| --- | --- |
| `ImportPath(ctx, path, options...)` | The source file is on the local filesystem |
| `ImportFS(ctx, fsys, path, options...)` | The source file lives in an arbitrary `fs.FS` |
| `ImportReader(ctx, reader, filename, options...)` | You already have a custom or memory-backed `ReadSeekerAt` |
| `ImportBytes(ctx, data, filename, options...)` | The source bytes are already in memory |

### Import From `fs.FS`

```go
fsys := fstest.MapFS{
	"scene.obj": &fstest.MapFile{
		Data: []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"),
	},
}

result, err := ravenporter.ImportFS(context.Background(), fsys, "scene.obj")
if err != nil {
	log.Fatal(err)
}

fmt.Println(result.Report.Source.DetectedFormat, len(result.Asset.Meshes))
```

### Import From In-Memory Bytes

```go
data := []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")

result, err := ravenporter.ImportBytes(context.Background(), data, "scene.obj")
if err != nil {
	log.Fatal(err)
}

fmt.Println(result.Report.Source.DetectedFormat, len(result.Asset.Meshes))
```

## Directory Entry Points

| Function | Best fit |
| --- | --- |
| `ImportDir(ctx, dir, options...)` | You want every supported asset under a local directory tree |
| `ImportFSDir(ctx, fsys, dir, options...)` | You want recursive import from an arbitrary `fs.FS` |

Both functions walk the tree recursively and only import files whose extension is supported by the current registry.

If you need to cap directory-import parallelism, pass `WithBatchConcurrency()`. A value of `0` keeps the default worker limit based on `runtime.GOMAXPROCS(0)`.

```go
results, err := ravenporter.ImportDir(
	context.Background(),
	"assets",
	ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
	ravenporter.WithBatchConcurrency(4),
)
if err != nil {
	log.Fatal(err)
}

for _, result := range results {
	log.Printf("%s -> %s", result.Report.Source.InputPath, result.Report.Source.DetectedFormat)
}
```

## Load Masking

Use `WithLoadMask()` when you only want certain domains in the returned asset.

```go
result, err := ravenporter.ImportBytes(
	context.Background(),
	data,
	"scene.obj",
	ravenporter.WithLoadMask(ravenporter.LoadMaterials),
)
```

`WithLoadMask()` cannot be serialized into a TOML profile. It reshapes the result after processing and before semantic validation.

## Registry And Dependency Resolution

- By default, imports use the built-in decoder registry returned by `NewRegistry()`.
- Use `WithRegistry()` to replace the registry for one import.
- `ImportPath()` and `ImportFS()` also expose filesystem context to decoders so relative texture references and similar source dependencies can be resolved during decode.

## Choosing The Right Function

- `ImportPath()` is the default choice for normal on-disk build pipelines.
- `ImportFS()` and `ImportFSDir()` make more sense when content already lives behind `embed.FS`, `fstest.MapFS`, or another virtual filesystem.
- `ImportReader()` is there for the cases where you genuinely need custom reader plumbing.
- `ImportBytes()` works well for tests, small generated assets, or preloaded blobs.
