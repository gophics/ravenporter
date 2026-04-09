# RavenPorter Cache

RavenPorter Cache is the cooked runtime asset format used by the `cache` package.

## Format

- Format name: RavenPorter Cache
- File extension: `.rpcache`

## Feature Matrix

| Feature | Status | Notes |
|---|---|---|
| Container magic and versioning | supported | Fixed magic with explicit format version |
| Chunk table layout | supported | `MANF`, `SCEN`, `BLOB` chunks |
| Bounded reads | supported | Chunk offsets, sizes, lengths, and element counts are validated before allocation |
| Lazy media loads | supported | `MANF` and `SCEN` load eagerly; `BLOB` payloads stay reader-backed by default |
| Manifest JSON | supported | Provenance, dependencies, notes, and summary |
| Typed asset serialization | supported | Flat IR arrays and cross-index references |
| Meshes and materials | supported | Includes typed material extensions |
| Textures and embedded payloads | supported | `data:` URIs are normalized to embedded bytes during cooking |
| Images, audio, and fonts | supported | Compressed and raw payloads are preserved |
| LOD groups and collision meshes | supported | Authored runtime-relevant structure is preserved |
| Material property whitelist | supported | `float32`, `int`, `bool`, `string`, `[3]float32`, `[4]float32` |
| Decoder callback restoration | supported | `DecodePixels()` and `DecodeSamples()` work after `cache.Open` |
| Image pixel persistence control | supported | `WithImagePixels` controls whether decoded pixel buffers are baked |
| Embedded media size budget | supported | `WithMaxEmbeddedMediaBytes` rejects oversized cache writes |
| Corruption rejection | supported | Invalid counts, bounds, booleans, and trailing scene data are rejected |
| Node extras preservation | unsupported | Dropped intentionally in cache v1 |
| External texture file references | unsupported | Rejected by `cache.Write` |
| Whole-file compression | unsupported | Not used in cache v1 |

## Notes

- The cache format is RavenPorter-owned, not an external interchange standard.
- Existing cooked caches from older commits should be treated as disposable and fully rebuilt after serializer layout changes; this branch now uses container version `1`.
- Call `(*cache.Asset).Close()` when you open a reader-backed cache through `Open` or lazy `Read`.
- Use `WithEagerMedia()` when you want the old all-bytes-materialized behavior during `Read`/`Open`.
- Decoded image pixels are not serialized by default when compressed bytes are enough to reconstruct them.
- `KHR_draco_mesh_compression` remains unsupported in the source glTF importer and is therefore unsupported in cooked runtime assets.
