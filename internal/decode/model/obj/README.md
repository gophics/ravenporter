# OBJ Decoder

Wavefront OBJ + MTL decoder with zero-alloc byte scanning.

## Supported Features

| Feature | Status |
|---|---|
| Vertex positions (`v`) | ✅ |
| Texture coordinates (`vt`) | ✅ |
| Vertex normals (`vn`) | ✅ |
| Faces (`f`) - triangles, quads, n-gons | ✅ |
| Negative indices | ✅ |
| Groups (`g`) / Objects (`o`) | ✅ |
| Smooth shading (`s`) | ✅ |
| Material library (`mtllib`) | ✅ |
| Material assignment (`usemtl`) | ✅ |
| Fan triangulation | ✅ |
| Vertex deduplication | ✅ |
| Line elements (`l`) | ✅ |
| Point elements (`p`) | ✅ |
| Free-form: Bezier curves/surfaces | ✅ |
| Free-form: B-spline curves/surfaces | ✅ |
| Free-form: Cardinal curves | ✅ |
| Free-form: Taylor curves | ✅ |
| Free-form: Basis matrix curves/surfaces | ✅ |
| Free-form: Rational forms (`rat`) | ✅ |
| Free-form: Parameter vertices (`vp`) | ✅ |
| Free-form: Trimming loops (`trim`, `hole`) | ✅ |
| Free-form: Parameter knots (`parm`) | ✅ |
| Free-form: Line continuation (`\`) | ✅ |
| Vertex colors (`v x y z r g b`) | ✅ |
| Homogeneous coordinates (`v x y z w`) | ✅ |
| MTL: Diffuse color (`Kd`) | ✅ |
| MTL: Specular color (`Ks`) | ✅ |
| MTL: Ambient color (`Ka`) | ✅ |
| MTL: Shininess (`Ns`) | ✅ |
| MTL: Dissolve (`d`) / Transparency (`Tr`) | ✅ |
| MTL: PBR metallic (`Pm`) | ✅ |
| MTL: PBR roughness (`Pr`) | ✅ |
| MTL: Diffuse texture (`map_Kd`) | ✅ |
| MTL: Specular texture (`map_Ks`) | ✅ |
| MTL: Roughness texture (`map_Ns`) | ✅ |
| MTL: Alpha texture (`map_d`) | ✅ |
| MTL: Normal/bump map (`map_bump`, `bump`, `norm`) | ✅ |
| MTL: Emissive color (`Ke`) | ✅ |
| MTL: Emissive texture (`map_Ke`) | ✅ |
| MTL: Illumination model (`illum`) | ✅ |
| MTL: Optical density / IOR (`Ni`) | ✅ |
| MTL: PBR roughness texture (`map_Pr`) | ✅ |
| MTL: PBR metallic texture (`map_Pm`) | ✅ |
| MTL: Sheen (`Ps`, `map_Ps`) | ✅ |
| MTL: Clearcoat thickness (`Pc`) | ✅ |
| MTL: Clearcoat roughness (`Pcr`) | ✅ |
| MTL: Transmittance filter (`Tf`) | ✅ |
| MTL: Anisotropy (`aniso`, `anisor`) | ✅ |
| MTL: Ambient texture (`map_Ka`) | ✅ |
| MTL: Displacement map (`disp`) | ✅ |
| MTL: Reflection map (`refl`) | ✅ |
| MTL: Texture options (`-s`, `-o`, `-bm`, `-t`) | ✅ |

## Performance

| Benchmark | ns/op | MB/s | B/op | allocs/op |
|---|---|---|---|---|
| Decode100 | 154,695 | 74.13 | 105,937 | 41 |
