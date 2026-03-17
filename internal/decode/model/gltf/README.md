# glTF 2.0 / GLB Decoder

Extensions: `.gltf`, `.glb`

glTF 2.0 and GLB binary decoder using `valyala/fastjson` with zero-copy buffer sharing.

## Supported Features

| Feature | Status |
|---|---|
| GLB binary container | ✅ |
| JSON glTF | ✅ |
| External `.bin` buffers | ✅ |
| Meshes (positions, normals, tangents) | ✅ |
| TexCoord 0-3 | ✅ |
| Vertex colors (float + ubyte) | ✅ |
| Indices (ubyte / ushort / uint) | ✅ |
| Morph targets (pos / norm / tan) | ✅ |
| Morph weights | ✅ |
| Primitive modes (all 7) | ✅ |
| PBR metallic-roughness | ✅ |
| Normal / occlusion textures | ✅ |
| Emissive factor + texture | ✅ |
| Alpha modes (opaque/mask/blend) | ✅ |
| Embedded textures (`bufferView`) | ✅ |
| External texture URIs | ✅ |
| Sampler wrap + filter modes | ✅ |
| Scene hierarchy (nodes) | ✅ |
| Matrix transforms | ✅ |
| TRS transforms | ✅ |
| Perspective cameras | ✅ |
| Orthographic cameras | ✅ |
| Skeletal animation (skins) | ✅ |
| Joints 0-1 / Weights 0-1 | ✅ |
| Inverse bind matrices | ✅ |
| Keyframe animation (T/R/S/W) | ✅ |
| Interpolation (linear/step/cubic spline) | ✅ |
| KHR_lights_punctual | ✅ |
| KHR_materials_unlit | ✅ |
| KHR_mesh_quantization | ✅ |
| EXT_meshopt_compression | ✅ |
| KHR_texture_basisu | ✅ |
| KHR_texture_transform | ✅ |
| KHR_materials_emissive_strength | ✅ |
| KHR_materials_clearcoat | ✅ |
| KHR_materials_sheen | ✅ |
| KHR_materials_transmission | ✅ |
| KHR_materials_volume | ✅ |
| KHR_materials_ior | ✅ |
| KHR_materials_specular | ✅ |
| KHR_materials_anisotropy | ✅ |
| Sparse accessors (detection) | ✅ |
| Sparse accessors (data application) | ✅ |
| Required extensions check | ✅ |
| KHR_materials_pbrSpecularGlossiness | ✅ |
| KHR_materials_dispersion | ✅ |
| KHR_materials_diffuse_transmission | ✅ |
| KHR_animation_pointer | ✅ |
| KHR_materials_iridescence | ✅ |
| KHR_materials_variants | ✅ |
| KHR_draco_mesh_compression | ❌ |

`EXT_meshopt_compression` support includes `ATTRIBUTES`, `TRIANGLES`, and `INDICES` modes with `NONE`, `OCTAHEDRAL`, `QUATERNION`, and `EXPONENTIAL` filters.

## Benchmarks

The package includes:

- `BenchmarkDecodeGLTF`
- `BenchmarkProbeGLTF`
- `BenchmarkDecodeGLTF_1K`
- `BenchmarkDecodeGLTF_Allocs`
- `BenchmarkDecodeMeshoptAttributes`
- `BenchmarkDecodeMeshoptTriangles`
- `BenchmarkDecodeMeshoptIndices`
- `BenchmarkDecodeMeshoptFilteredAttributes`
- `BenchmarkDecodeMeshoptScene`
