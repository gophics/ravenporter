---
title: Media Representation
description: How images, audio, and fonts are stored in the IR, including lazy compressed bytes, pixel decoding, sample decoding, and font raw-data loading.
---

RavenPorter's media types keep useful source data around without forcing an eager decode of every asset payload.

## Images

`ImageAsset` stores:

- identity fields such as `Name`, `Format`, `Width`, `Height`, `Channels`, `ColorSpace`, and `MipLevels`
- raw compressed bytes through `Compressed` or a lazy compressed loader
- optional decoded pixel data through `PixelBuffer`
- `SourceFormat`, `SourcePath`, and `CompressionFormat`

### Lazy And Eager Image Data

- `CompressedBytes()` materializes raw encoded bytes on demand.
- `Pixels()` returns any already-decoded pixel buffer.
- `DecodePixels()` runs the decoder callback once and caches the result.

GPU-compressed formats are treated as passthrough content. `DecodePixels()` returns an error for GPU-compressed images, so check `IsGPUCompressed()` first.

## Audio

`AudioClip` stores:

- container and codec identity
- sample rate, layout, bit depth, duration, loop points
- metadata such as title, artist, album, artwork, and cue points
- raw compressed bytes or a lazy loader
- optional PCM sample decode through `DecodeSamples()`

The decoded samples are cached after the first successful decode.

## Fonts

`Font` uses composition:

- `VectorFontData` for vector fonts
- `BitmapFontData` for rasterized atlases

`VectorFontData` keeps metrics, codepoints, kerning, and raw font bytes. The raw bytes can also be loaded lazily through `RawBytes()`.

`BitmapFontData` stores atlas metrics, glyph rectangles, and an atlas image reference through `AtlasIndex`.

## Why This Matters

This model lets RavenPorter:

- keep source-compressed media for runtime passthrough
- decode only when needed
- preserve media in the cooked cache without forcing full eager materialization
- restore decode behavior after reading a cache file

See [Runtime Cache](../runtime-cache/) and [Cache Format Model](../cache-format-model/) for how those lazy loaders survive cooking.
