---
title: USD Decoder
description: Specification support reference for RavenPorter's USDA, USDC, and USDZ importer.
---

RavenPorter imports multiple Universal Scene Description variants from USDA, USDC, and USDZ files. The current importer covers core scene, material, lighting, animation, and package features across both ASCII and binary representations, with a few variant-specific gaps noted below.

## Extensions

`.usda`, `.usd`, `.usdc`, `.usdz`

## Supported Specification Features

| Feature | USDA | USDC |
| --- | --- | --- |
| Mesh prims, face indices/triangulation, normals, `primvars:st`, `displayColor`, `displayOpacity`, and `doubleSided` | Supported | Supported |
| Material binding, `Xform` / `Scope` prims, parent-child hierarchy, `upAxis`, `metersPerUnit`, and `defaultPrim` metadata | Supported | Supported |
| Perspective and orthographic cameras | Supported | Supported |
| `DistantLight`, `SphereLight`, `DiskLight`, `RectLight`, and `CylinderLight` prims | Supported | Supported |
| `UsdPreviewSurface` materials with opacity, emissive, clearcoat, ior, texture connections, wrap modes, and multi-slot textures | Supported | Supported |
| Skeletons, joint hierarchies, skinned meshes, bind transforms, and `SkelAnimation` | Supported | Supported |
| Procedural `Cube`, `Sphere`, `Cylinder`, `Cone`, and `Capsule` prims | Supported | Supported |
| `BasisCurves`, `NurbsCurves`, and `Points` prims | Supported | Supported |
| `GeomSubset` face material splits and `BlendShape` morph targets | Supported | Supported |
| Variant set metadata extraction | Supported | Supported |
| Crate binary encoding, double-precision values, `matrix4d` data, token arrays, and path hierarchies | - | Supported |
| USDZ packaged image assets recognized by RavenPorter filename extension. It includes PNG, JPEG, WebP, KTX / KTX2, DDS, BMP, TGA, HDR, PSD, TIFF, and EXR | Supported | Supported |

## Unimplemented Runtime-Relevant Features

- USDA imports `subLayers`, `references`, `payload`, `inherits`, and `specializes`; the current USDC crate conversion does not import those composition arcs.
- Animated mesh import from `points.timeSamples` is available on the USDA path only; the current USDC crate conversion does not import time-sampled mesh points.

## Out Of Scope For RavenPorter

None.

## Notes

- `.usd` files are imported according to whether their contents resolve to USDA or USDC data.

