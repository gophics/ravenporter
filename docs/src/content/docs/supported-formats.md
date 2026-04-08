---
title: Supported Formats
description: Built-in model, image, audio, and font formats that RavenPorter can detect and import.
---

If you need the exact built-in catalog programmatically, call [`SupportedFormats()`](../ravenporter-package/) or [`SupportedExtensions()`](../ravenporter-package/), or run `ravenporter formats`. The tables below group that registry into broader format families, so the exact decoder labels you see in code or CLI output can be a little narrower, such as `OTF`, `TTF`, or `USDA`.

## Models And Scene Assets

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

## Images

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

## Audio

| Format | Extensions |
| --- | --- |
| AIFF | `.aiff`, `.aif` |
| FLAC | `.flac` |
| MP3 | `.mp3` |
| Ogg | `.ogg`, `.oga` |
| Opus | `.opus` |
| WAV | `.wav` |

## Fonts

| Format | Extensions |
| --- | --- |
| OpenType | `.otf` |
| TrueType | `.ttf` |
| WOFF | `.woff` |
| WOFF2 | `.woff2` |

## Notes

- Detection uses a mix of file signatures, decoder probes, and filename extensions through the [`detect.Registry`](../detect-package/).
- GPU-compressed image payloads such as DDS and KTX can stay in compressed passthrough form inside the IR and cache model.
- Support here means RavenPorter can detect and import the source into [`ir.Asset`](../ir-overview/), not that every format feature is preserved one-to-one.
- `KHR_draco_mesh_compression` is still unsupported.
