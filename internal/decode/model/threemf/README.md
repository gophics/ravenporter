# 3MF Decoder

**Format:** 3D Manufacturing Format (3MF)
**Extension:** `.3mf`

## Supported Features

| Feature | Status |
|---|---|
| Mesh geometry (vertices, triangles) | ✅ |
| BaseMaterials (color → PBR) | ✅ |
| Vertex colors (ColorGroup) | ✅ |
| Object-level PID fallback | ✅ |
| Unit scale (mm, in, ft, m) | ✅ |
| Multi-object scenes | ✅ |
| Texture2D UV-mapped textures | ✅ |

> Parsing delegated to `go3mf` library; this decoder converts `go3mf.Model` → `ir.Scene`.
