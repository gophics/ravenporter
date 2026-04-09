---
title: OBJ Decoder
description: Specification support reference for RavenPorter's Wavefront OBJ and MTL importer.
---

RavenPorter imports Wavefront geometry and material data from `.obj` scene files and their companion `.mtl` libraries. It includes a wide span of classic OBJ surface and material statements.

## Extensions

`.obj`

## Supported Specification Features

- Core OBJ geometry records `v`, `vt`, `vn`, and `f`. It includes triangles, quads, n-gons, and negative indices
- Object/group organization, smooth shading, line elements, point elements, and line continuation
- Free-form curve and surface records including Bezier, B-spline, Cardinal, Taylor, basis-matrix, rational, trimming, knot, and parameter-vertex data
- Vertex colors and homogeneous vertex coordinates
- Material library references through `mtllib` and `usemtl`
- MTL shading/material fields including `Kd`, `Ks`, `Ka`, `Ns`, `d`, `Tr`, `illum`, and `Ni`
- PBR-style MTL fields including `Pm`, `Pr`, `Ps`, `Pc`, `Pcr`, `Tf`, `aniso`, and `anisor`
- Texture map statements including `map_Kd`, `map_Ks`, `map_Ns`, `map_d`, `map_bump`, `bump`, `norm`, `map_Ke`, `map_Pr`, `map_Pm`, `map_Ka`, `disp`, and `refl`
- Texture options such as `-s`, `-o`, and `-bm`

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- The Wavefront binary `.mod` variant mentioned in the original OBJ appendix is not supported.
- Only RGB numeric forms are imported for MTL color statements; `xyz` and spectral `.rfl` forms are not supported.
- The optional third `vt` coordinate is not preserved; RavenPorter imports texture coordinates as 2D UVs.
- Free-form `step` directives do not control tessellation density; RavenPorter uses fixed internal sampling for imported curves and surfaces.
- Only a subset of MTL texture-map options affect imported material state, primarily `-s`, `-o`, and `-bm`.

## Notes

- None.

