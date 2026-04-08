---
title: IR Media and Animation Reference
description: Reference for image, audio, font, animation, and skeleton exports in the ir package.
---

This page covers the media- and animation-oriented exports from [`ir`](../ir-overview/).

## Image Types

| Type | Notes |
| --- | --- |
| `ImageAsset` | Image payload plus lazy compressed/pixel access |
| `PixelBuffer` | Decoded pixel data and mipmaps |
| `PixelDecodeFunc` | Image decode callback type |
| `ImageFormat` | PNG, JPEG, DDS, KTX, EXR, and others |
| `ColorSpace` | `sRGB` or `linear` |
| `ChannelCount` | Gray, gray-alpha, RGB, RGBA |
| `BitDepth` | 8, 16, 24, or 32 |
| `GPUCompression` | GPU-native block compression marker |
| `DataType` | Uint8 or float32 pixel payload |

## Audio Types

| Type | Notes |
| --- | --- |
| `AudioClip` | Audio payload plus lazy compressed/sample access |
| `SampleDecodeFunc` | Sample decode callback type |
| `AudioMetadata` | Title, artist, album, artwork, cue points |
| `CuePoint` | Named audio sample marker |
| `AudioFormat` | WAV, OGG, MP3, FLAC, AIFF, Opus |
| `ChannelLayout` | Mono, stereo, 5.1, 7.1 |

## Font Types

| Type | Notes |
| --- | --- |
| `Font` | Wrapper around vector or bitmap font data |
| `VectorFontData` | Metrics, codepoints, kerning, and raw bytes |
| `BitmapFontData` | Atlas metrics and glyph rectangles |
| `BitmapGlyph` | Single atlas glyph placement |
| `FontFormat` | TTF, OTF, WOFF, WOFF2, BMFont |

## Animation Types

| Type | Notes |
| --- | --- |
| `Animation` | Named animation clip |
| `AnimationChannel` | One target property on one node |
| `Skeleton` | Joint set and inverse bind matrices |
| `ChannelTarget` | Translation, rotation, scale, morph weights, pointer, material, camera, or light targets |
| `Interpolation` | Linear, step, or cubic spline |

For the narrative explanation, continue to [Media Representation](../media-representation/) and [Animation and Skeletons](../animation-and-skeletons/).
