---
title: glTF 2.0 / GLB Decoder
description: Specification support reference for RavenPorter's glTF 2.0 and GLB importer.
---

RavenPorter imports glTF 2.0 assets from JSON `.gltf` files and binary `.glb` containers. It handles the core scene model along with many Khronos material, animation, and compression extensions.

## Extensions

`.gltf`, `.glb`

## Supported Specification Features

- JSON glTF and binary GLB containers. It includes external `.bin` buffers
- Mesh primitives with positions, normals, tangents, texture coordinate sets 0-3, vertex colors, multiple index sizes, morph targets, morph weights, and primitive modes `POINTS`, `LINES`, `LINE_LOOP`, `LINE_STRIP`, `TRIANGLES`, `TRIANGLE_STRIP`, and `TRIANGLE_FAN`
- PBR metallic-roughness materials with alpha modes, normal, occlusion, and emissive textures
- Embedded texture sources from `bufferView`, external texture URIs, and sampler wrap/filter state
- `KHR_texture_transform` on the imported texture references RavenPorter already maps
- Scene hierarchies with node transforms in matrix or TRS form, plus perspective and orthographic cameras
- Skinning with joints and weights sets 0-1, inverse bind matrices, and keyframe animation for translation, rotation, scale, and morph weights
- Interpolation modes `LINEAR`, `STEP`, and `CUBICSPLINE`
- `KHR_lights_punctual`, `KHR_materials_unlit`, `KHR_mesh_quantization`, `KHR_texture_basisu`, `KHR_materials_emissive_strength`, `KHR_materials_clearcoat`, `KHR_materials_sheen`, `KHR_materials_transmission`, `KHR_materials_volume`, `KHR_materials_ior`, `KHR_materials_specular`, `KHR_materials_anisotropy`, `KHR_materials_pbrSpecularGlossiness`, `KHR_materials_dispersion`, `KHR_materials_diffuse_transmission`, `KHR_animation_pointer`, `KHR_materials_iridescence`, `EXT_mesh_gpu_instancing`, `EXT_texture_webp`, and `EXT_meshopt_compression`
- `KHR_materials_variants` variant-name extraction plus per-primitive variant-mapping metadata
- Sparse accessors and required-extension validation

## Unimplemented Runtime-Relevant Features

- `KHR_draco_mesh_compression` is not supported.

## Out Of Scope For RavenPorter

- `KHR_materials_variants` is imported as metadata only; RavenPorter does not automatically resolve or switch the active material variant.

## Notes

- `EXT_meshopt_compression` support covers `ATTRIBUTES`, `TRIANGLES`, and `INDICES` modes with `NONE`, `OCTAHEDRAL`, `QUATERNION`, and `EXPONENTIAL` filters.

