---
title: Supported Formats
description: Built-in model, image, audio, and font formats that RavenPorter can detect and import.
---

If you need the exact built-in catalog programmatically, call [`SupportedFormats()`](../ravenporter-package/) or [`SupportedExtensions()`](../ravenporter-package/), or run `ravenporter formats`. The tables below group that registry into broader format families, so the exact decoder labels you see in code or CLI output can be a little narrower, such as `OTF`, `TTF`, or `USDA`.

Use the linked format names to open the per-decoder specification reference pages when you need format-level coverage details.

## Models And Scene Assets

| Format | Extensions |
| --- | --- |
| [Alembic](../decoder-reference/models/alembic/) | `.abc` |
| [BVH](../decoder-reference/models/bvh/) | `.bvh` |
| [COLLADA](../decoder-reference/models/collada/) | `.dae` |
| [FBX](../decoder-reference/models/fbx/) | `.fbx` |
| [glTF 2.0](../decoder-reference/models/gltf/) | `.gltf`, `.glb` |
| [OBJ](../decoder-reference/models/obj/) | `.obj` |
| [PLY](../decoder-reference/models/ply/) | `.ply` |
| [STL](../decoder-reference/models/stl/) | `.stl` |
| [3D Studio](../decoder-reference/models/three-d-studio/) | `.3ds` |
| [3MF](../decoder-reference/models/three-mf/) | `.3mf` |
| [USD](../decoder-reference/models/usd/) | `.usda`, `.usd`, `.usdc`, `.usdz` |

## Images

| Format | Extensions |
| --- | --- |
| [BMP](../decoder-reference/images/bmp/) | `.bmp` |
| [DDS](../decoder-reference/images/dds/) | `.dds` |
| [EXR](../decoder-reference/images/exr/) | `.exr` |
| [Radiance HDR](../decoder-reference/images/hdr/) | `.hdr` |
| [JPEG](../decoder-reference/images/jpeg/) | `.jpeg`, `.jpg` |
| [KTX](../decoder-reference/images/ktx/) | `.ktx`, `.ktx2` |
| [PNG](../decoder-reference/images/png/) | `.png` |
| [Photoshop](../decoder-reference/images/psd/) | `.psd`, `.psb` |
| [TGA](../decoder-reference/images/tga/) | `.tga` |
| [TIFF](../decoder-reference/images/tiff/) | `.tiff`, `.tif` |
| [WebP](../decoder-reference/images/webp/) | `.webp` |

## Audio

| Format | Extensions |
| --- | --- |
| [AIFF](../decoder-reference/audio/aiff/) | `.aiff`, `.aif` |
| [FLAC](../decoder-reference/audio/flac/) | `.flac` |
| [MP3](../decoder-reference/audio/mp3/) | `.mp3` |
| [Ogg](../decoder-reference/audio/ogg/) | `.ogg`, `.oga` |
| [Opus](../decoder-reference/audio/opus/) | `.opus` |
| [WAV](../decoder-reference/audio/wav/) | `.wav` |

## Fonts

| Format | Extensions |
| --- | --- |
| [OpenType](../decoder-reference/fonts/otf/) | `.otf` |
| [TrueType](../decoder-reference/fonts/ttf/) | `.ttf` |
| [WOFF](../decoder-reference/fonts/woff/) | `.woff` |
| [WOFF2](../decoder-reference/fonts/woff2/) | `.woff2` |

## Notes

- Detection uses a mix of file signatures, decoder probes, and filename extensions through the [`detect.Registry`](../detect-package/).
- GPU-compressed image payloads such as DDS and KTX can stay in compressed passthrough form inside the IR and cache model.
- Support here means RavenPorter can detect and import the source into [`ir.Asset`](../ir-overview/), not that every format feature is preserved one-to-one.
- `KHR_draco_mesh_compression` is still unsupported.
