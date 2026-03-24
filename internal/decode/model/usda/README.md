# USDA / USDC / USDZ Decoder

**Format:** Universal Scene Description (ASCII, Binary, Archive)
**Extensions:** `.usda`, `.usd`, `.usdc`, `.usdz`

## Supported Features

| Feature | USDA | USDC |
|---|---|---|
| Mesh prims (point3f[] points) | ✅ | ✅ |
| Face vertex indices | ✅ | ✅ |
| Face triangulation (fan, via faceVertexCounts) | ✅ | ✅ |
| Normals (normal3f[]) | ✅ | ✅ |
| Texture coordinates (texCoord2f[] primvars:st) | ✅ | ✅ |
| Vertex colors (color3f[] primvars:displayColor) | ✅ | ✅ |
| Vertex opacity (float[] primvars:displayOpacity) | ✅ | ✅ |
| DoubleSided meshes (uniform bool doubleSided) | ✅ | ✅ |
| Material binding on mesh (rel material:binding) | ✅ | ✅ |
| Orientation (leftHanded winding flip) | ✅ | ✅ |
| Xform prims (translate, rotateXYZ, scale, matrix) | ✅ | ✅ |
| Scope prims (organizational container) | ✅ | ✅ |
| Parent-child hierarchy | ✅ | ✅ |
| upAxis / metersPerUnit metadata | ✅ | ✅ |
| defaultPrim metadata | ✅ | ✅ |
| Camera prims (perspective) | ✅ | ✅ |
| Camera prims (orthographic) | ✅ | ✅ |
| Light prims (Distant, Sphere, Disk, Rect, Cylinder) | ✅ | ✅ |
| Material/Shader prims (UsdPreviewSurface PBR) | ✅ | ✅ |
| Material opacity (inputs:opacity) | ✅ | ✅ |
| Material emissive (inputs:emissiveColor) | ✅ | ✅ |
| Material opacityThreshold → AlphaCutoff | ✅ | ✅ |
| Material clearcoat/clearcoatRoughness/ior | ✅ | ✅ |
| Material texture connections (UsdUVTexture) | ✅ | ✅ |
| Material texture wrap modes (wrapS/wrapT) | ✅ | ✅ |
| Material multi-slot textures (metallic/normal/etc) | ✅ | ✅ |
| Skeleton prims (joints, bindTransforms) | ✅ | ✅ |
| Skeleton joint hierarchy (path-based Children) | ✅ | ✅ |
| Skeleton joint indices/weights on mesh | ✅ | ✅ |
| SkelAnimation (static + timeSamples) | ✅ | ✅ |
| Procedural Cube mesh | ✅ | ✅ |
| Procedural Sphere mesh | ✅ | ✅ |
| Procedural Cylinder mesh | ✅ | ✅ |
| Procedural Cone mesh | ✅ | ✅ |
| Procedural Capsule mesh | ✅ | ✅ |
| BasisCurves → LineStrip/Lines | ✅ | ✅ |
| Points prim → Points mode | ✅ | ✅ |
| NurbsCurves → LineStrip/Lines | ✅ | ✅ |
| GeomSubset (face-level material splitting) | ✅ | ✅ |
| BlendShape (morph targets) | ✅ | ✅ |
| USDC binary format (crate) | - | ✅ |
| USDC double precision values (float64, vec3d) | - | ✅ |
| USDC matrix4d transforms | - | ✅ |
| USDC matrix4d[] arrays (bindTransforms) | - | ✅ |
| USDC token arrays | - | ✅ |
| USDC path hierarchy (parent-child nodes) | - | ✅ |
| USDZ zip archive format | ✅ | ✅ |
| USDZ embedded textures (PNG, JPEG, EXR) | ✅ | ✅ |
| USDZ references/payload (intra-archive) | ✅ | - |
| Composition arcs (sublayers/inherits/specializes) | ✅ | - |
| VariantSets metadata extraction | ✅ | ✅ |
| Animated mesh (points.timeSamples → morph targets) | ✅ | - |
