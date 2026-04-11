# RavenPorter

RavenPorter is a pure-Go asset ingest and runtime-cooking library for games,
tools, and asset pipelines.

It loads source assets into a single runtime-oriented IR, [`ir.Asset`](./ir/asset.go), and it can also write and read a cooked cache format through [`cache`](./cache/). The goal is straightforward: one import surface for models, images, audio, and fonts without dragging engine-specific assumptions into the library.

> Development status: RavenPorter is still under active development. Public APIs, decoder coverage, cache details, and documentation may change before the first stable release.

## Requirements

- Go `1.25.7` or newer
- Node.js `22.12.0` or newer if you want to build the docs site

## What It Is Good At

- importing mixed asset types through one API
- staying pure Go
- preserving useful provenance and dependency data during import
- cooking runtime-ready assets for faster load paths
- keeping the public API small while leaving advanced hooks available

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/gophics/ravenporter"
)

func main() {
	result, err := ravenporter.ImportPath(
		context.Background(),
		"assets/scene.glb",
		ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"format=%s scenes=%d meshes=%d",
		result.Report.Source.DetectedFormat,
		len(result.Asset.Scenes),
		len(result.Asset.Meshes),
	)
}
```

## Loading A Folder

Use `ImportDir` for local directories and `ImportFSDir` for arbitrary
filesystems. These calls walk the tree recursively and import every supported
asset they find. By default, batch imports use a worker limit based on
`runtime.GOMAXPROCS(0)`. Use `WithBatchConcurrency` if you want to cap it.

```go
package main

import (
	"context"
	"log"

	"github.com/gophics/ravenporter"
)

func main() {
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
		log.Printf(
			"%s: format=%s scenes=%d meshes=%d",
			result.Report.Source.InputPath,
			result.Report.Source.DetectedFormat,
			len(result.Asset.Scenes),
			len(result.Asset.Meshes),
		)
	}
}
```

## Runtime Cache

If you want a faster runtime load path than reparsing source assets every time,
use the cooked cache format.

```go
package main

import (
	"bytes"
	"context"
	"log"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/cache"
)

func main() {
	result, err := ravenporter.ImportPath(
		context.Background(),
		"assets/scene.glb",
		ravenporter.WithEmbedTextures(),
	)
	if err != nil {
		log.Fatal(err)
	}

	var cooked bytes.Buffer
	if err := cache.Write(&cooked, result); err != nil {
		log.Fatal(err)
	}

	pkg, err := cache.Read(bytes.NewReader(cooked.Bytes()), int64(cooked.Len()))
	if err != nil {
		log.Fatal(err)
	}
	defer pkg.Close()

	log.Printf("runtime meshes=%d", len(pkg.Asset.Meshes))
}
```

`cache.Write` is intentionally strict. If runtime-critical texture dependencies
are still external, it fails instead of writing a half-self-contained package.
Use `WithEmbedTextures()` when you want a single-file runtime asset.

By default, cooked assets keep scene data eager and media blobs lazy. That keeps
startup memory lower for large textures, audio, and font payloads. Use:

- `cache.WithImagePixels(...)` to control decoded image pixel persistence
- `cache.WithMaxEmbeddedMediaBytes(...)` to reject oversized cooked assets
- `cache.WithEagerMedia()` when you explicitly want media bytes materialized at open time

## Emitting JSON IR

RavenPorter also ships with a JSON emitter for inspection, debugging, and
tooling handoff. It is not meant to be a final runtime format, but it is useful
when you want to look at the imported asset shape directly.

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

## Built-In Decoder Support

These are the built-in source formats RavenPorter can detect and import into
`ir.Asset`.

### Models And Scene Assets

| Format | Extensions |
| --- | --- |
| Alembic | `.abc` |
| BVH | `.bvh` |
| COLLADA | `.dae` |
| FBX | `.fbx` |
| glTF 2.0 | `.gltf`, `.glb` |
| OBJ | `.obj` |
| PLY | `.ply` |
| STL | `.stl` |
| 3D Studio | `.3ds` |
| 3MF | `.3mf` |
| USD | `.usda`, `.usd`, `.usdc`, `.usdz` |

### Images

| Format | Extensions |
| --- | --- |
| BMP | `.bmp` |
| DDS | `.dds` |
| EXR | `.exr` |
| Radiance HDR | `.hdr` |
| JPEG | `.jpeg`, `.jpg` |
| KTX | `.ktx`, `.ktx2` |
| PNG | `.png` |
| Photoshop | `.psd`, `.psb` |
| TGA | `.tga` |
| TIFF | `.tiff`, `.tif` |
| WebP | `.webp` |

### Audio

| Format | Extensions |
| --- | --- |
| AIFF | `.aiff`, `.aif` |
| FLAC | `.flac` |
| MP3 | `.mp3` |
| Ogg | `.ogg`, `.oga` |
| Opus | `.opus` |
| WAV | `.wav` |

### Fonts

| Format | Extensions |
| --- | --- |
| OpenType | `.otf` |
| TrueType | `.ttf` |
| WOFF | `.woff` |
| WOFF2 | `.woff2` |

If you need to inspect the built-in catalog at runtime, use:

- `SupportedFormats()`
- `SupportedExtensions()`
- `SupportsExtension(ext)`

## Main API

The root package is the intended entry point.

- `ImportPath`, `ImportFS`, `ImportReader`, `ImportBytes`
- `ImportDir`, `ImportFSDir`
- `WithPreset`, `WithProfile`, `WithProfileFile`
- `WithDecodeMaxFileSize`, `WithDecodeMaxVertices`
- `WithDecodeMaxImagePixels`, `WithDecodeMaxAudioSamples`
- `WithGlobalScale`, `WithTargetUpAxis`, `WithEmbedTextures`
- `WithBatchConcurrency`
- `WithLoadMask`
- `WithRegistry`, `WithLogger`, `WithProcessFlags`
- `NewRegistry`
- `SupportedFormats`, `SupportedExtensions`, `SupportsExtension`

Profiles are TOML-serializable. You can load them with `LoadProfile`, save them
with `SaveProfile`, parse bytes with `ParseProfileTOML`, and derive them from
options with `ResolveProfile`. Use `BuiltInPresetNames()` when you want the
canonical preset strings programmatically.

## Runtime Model

RavenPorter is structured around a few clear layers:

- `ravenporter`: import source assets into `ir.Asset`
- `cache`: cook and load runtime-ready RavenPorter assets
- `process`: optional post-import conditioning
- `detect`: registry and decoder extension point
- `ir`: the runtime-oriented intermediate representation

The import pipeline and the built-in decoder catalog are internal
implementation details behind the root package.

If you build assets manually instead of importing them, call
`asset.NormalizeGraph()` before advanced traversal or direct processing.

If you prefer a command-line workflow, see the CLI guide in
[`cmd/ravenporter/README.md`](./cmd/ravenporter/README.md).

## Repository Test Layout

The root module stays lean and only ships the library plus small package-local
fixtures.

Heavy integration fixtures, goldens, and real-file decoder coverage live in
[`testsuite`](./testsuite/).

Use:

- `make test` for fast root-module validation
- `make test-integration` for corpus-backed integration coverage
- `make test-all` or `make release-check` for the full local gate

## Project Docs

- [`CHANGELOG.md`](./CHANGELOG.md) for release notes
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) for local setup and contribution expectations

## License

RavenPorter-authored source code in this repository is licensed under the
Apache License, Version 2.0. See [`LICENSE`](./LICENSE).

## Unsupported

- `KHR_draco_mesh_compression` remains intentionally unsupported.
