---
title: USD Decoder
description: Specification support reference for RavenPorter's USDA, USDC, and USDZ importer.
---

RavenPorter imports multiple Universal Scene Description variants from USDA, USDC, and USDZ files. It covers core scene, material, lighting, animation, and package features across both ASCII and binary representations.

## Extensions

`.usda`, `.usd`, `.usdc`, `.usdz`

## Supported Specification Features

- Supported in both USDA and USDC: mesh prims, face indices and triangulation, normals, `primvars:st`, `displayColor`, `displayOpacity`, and `doubleSided`
- Supported in both USDA and USDC: material binding, `Xform` and `Scope` prims, parent-child hierarchy, `upAxis`, `metersPerUnit`, and `defaultPrim` metadata
- Supported in both USDA and USDC: perspective and orthographic cameras
- Supported in both USDA and USDC: `DistantLight`, `SphereLight`, `DiskLight`, `RectLight`, and `CylinderLight` prims
- Supported in both USDA and USDC: `UsdPreviewSurface` materials with opacity, emissive, clearcoat, ior, texture connections, wrap modes, and multi-slot textures
- Supported in both USDA and USDC: skeletons, joint hierarchies, skinned meshes, bind transforms, and `SkelAnimation`
- Supported in both USDA and USDC: procedural `Cube`, `Sphere`, `Cylinder`, `Cone`, and `Capsule` prims
- Supported in both USDA and USDC: `BasisCurves`, `NurbsCurves`, and `Points` prims
- Supported in both USDA and USDC: `GeomSubset` face material splits, `BlendShape` morph targets, and variant set metadata extraction
- Supported in both USDA and USDC: composition arcs `subLayers`, `references`, `payload`, `inherits`, and `specializes`
- Supported in both USDA and USDC: animated mesh import from `points.timeSamples`
- Supported in both USDA and USDC: USDZ packaged image assets in PNG, JPEG, WebP, KTX and KTX2, DDS, BMP, TGA, HDR, PSD, TIFF, and EXR
- Supported in USDC only: crate binary encoding, double-precision values, `matrix4d` data, token arrays, and path hierarchies

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

None.

## Notes

- `.usd` files are imported according to whether their contents resolve to USDA or USDC data.
- External composition dependencies are always reported. External USDA or USDC scenes are merged only while decoding from a USDZ archive; standalone `.usdc` files do not add new filesystem resolution behavior.

