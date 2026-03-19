# FBX Decoder

Autodesk FBX binary and ASCII decoder.

## Supported Features

| Feature | Status |
|---|---|
| Binary FBX (v7400, v7500+) | ✅ |
| ASCII FBX | ✅ |
| Geometry (meshes, triangulation, fan polygons) | ✅ |
| Normals (by-polygon-vertex, by-vertex, indexed) | ✅ |
| UV coordinates (by-polygon-vertex, indexed) | ✅ |
| Vertex colors | ✅ |
| Materials (diffuse, emissive, specular, ambient) | ✅ |
| Textures (diffuse, emissive, normal map) | ✅ |
| Scene hierarchy (model parent/child) | ✅ |
| Cameras (perspective, FOV, near/far) | ✅ |
| Lights (point, directional, spot) | ✅ |
| Skeletal animation (skinning, clusters, IBM) | ✅ |
| Keyframe animation (T/R/S channels) | ✅ |
| Morph targets (blend shapes) | ✅ |
| Zlib-compressed arrays (pool.ZlibReader) | ✅ |
| GlobalSettings (up-axis, unit scale) | ✅ |
| Embedded textures (Video Content) | ✅ |
| Multiple animation takes/stacks | ✅ |
| Tangents / Binormals | ✅ |
| Smoothing groups → normal splitting | ✅ |

## Performance

| Path | allocs/op | MB/s |
|---|---|---|
| Binary (core) | 52 | 227 |
| ASCII (core) | 197 | 429 |
