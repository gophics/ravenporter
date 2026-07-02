# RavenPorter CLI

The `ravenporter` CLI is the practical companion to the library.

It is useful when you want to:

- inspect an asset quickly
- import source assets to JSON IR
- cook runtime cache files
- batch-convert a directory
- validate inputs before they enter a build pipeline
- export or save reusable import profiles

The CLI stays close to the library API. Presets, profile files, decode limits,
load behavior, and process-step overrides follow the same concepts as the root
package.

## Install

Requires Go `1.25.7` or newer.

```bash
go install github.com/gophics/ravenporter/cmd/ravenporter@v0.1.0
```

## Common Commands

### Import A Source Asset To JSON IR

```bash
ravenporter import assets/scene.glb --out out --pretty
```

This writes JSON IR to `out/scene.json`.

### Cook A Runtime Cache

```bash
ravenporter cook assets/scene.glb --out build/scene.rpcache --embed-textures
```

This imports the source asset, resolves the configured processing steps, and
writes a cooked RavenPorter cache file.

If you want stricter cache policy during the cook step:

```bash
ravenporter cook assets/scene.glb \
  --cache-image-pixels if-present \
  --cache-max-embedded-media-bytes 268435456
```

Use `--cache-image-pixels` to decide whether decoded image pixels are baked
into the cache, and `--cache-max-embedded-media-bytes` to reject oversized
runtime packages.

### Inspect An Asset

```bash
ravenporter info assets/scene.glb
```

For machine-readable output:

```bash
ravenporter info assets/scene.glb --json
```

`info` also works on cooked `.rpcache` files.

### Validate An Asset

```bash
ravenporter validate assets/scene.glb
```

JSON output is available too:

```bash
ravenporter validate assets/scene.glb --json
```

### Export JSON IR Explicitly

```bash
ravenporter export assets/scene.glb --format json --out out/scene.json --pretty
```

Today, `json` is the only emitter format exposed by the CLI.

### Batch Import A Directory

```bash
ravenporter batch assets --out out --pretty --recursive
```

This writes JSON IR for every supported asset it finds. Add `--recursive` when
you want to walk subdirectories too; without it, `batch` stays at the top level
of the input directory.

### Convert A Source Asset

```bash
ravenporter convert assets/scene.glb --out out/scene.json --pretty
```

Use `convert` when you want the shorter import-plus-export workflow and you are
happy with the emitter implied by `--out`. Today that still means JSON, so point
`--out` at a `.json` file.

## Profiles

Profiles let you pin import behavior as TOML instead of repeating flags.

Export the effective profile you want:

```bash
ravenporter profile export --preset quality --embed-textures --scale 0.01 --out profiles/game.toml
```

Then reuse it:

```bash
ravenporter import assets/scene.fbx --profile-file profiles/game.toml --out out
```

Explicit CLI flags still win over the profile file.

## Process Steps

If you need more control than presets provide, you can override individual
processing steps.

List the canonical step names:

```bash
ravenporter steps
```

Or get structured output:

```bash
ravenporter steps --json
```

Then enable or disable steps directly:

```bash
ravenporter cook assets/scene.glb --enable-step embed-textures --disable-step find-instances
```

These step names are the same names used in profile files.

## Useful Flags

Common import-style flags include:

- `--preset fast|quality|max-quality`
- `--profile-file path/to/profile.toml`
- `--embed-textures`
- `--scale 0.01`
- `--up-axis Y|Z`
- `--decode-max-file-size`
- `--decode-max-vertices`
- `--enable-step`
- `--disable-step`
- `--report-file`
- `--cache-image-pixels never|if-present|always`
- `--cache-max-embedded-media-bytes`

## Discovery

See which built-in formats are available:

```bash
ravenporter formats
```

## Practical Flow

For an offline game-asset workflow, the usual sequence is:

1. `ravenporter info` or `ravenporter validate` while iterating on source files
2. `ravenporter profile export` once the import behavior is right
3. `ravenporter cook` during the asset build step
4. load the resulting `.rpcache` files at runtime through the library

## Current Limits

- Draco- and meshopt-compressed glTF meshes are decoded during import before validation, reporting, or cooking.
- The only public CLI emitter format today is JSON.
