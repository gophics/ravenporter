# EXR Decoder

OpenEXR image decoder with scanline and tiled pixel decompression.

## Extensions

`.exr`

## Supported Features

| Feature | Status |
| :--- | :---: |
| Dimensions Extraction (`dataWindow`) | ✅ |
| Attribute Parsing | ✅ |
| Channel Count Detection | ✅ |
| Compression Type Detection | ✅ |
| Pixel Type Detection (Half / Float32) | ✅ |
| Tiled Flag Detection | ✅ |
| Tile Size Extraction | ✅ |
| Multi-Part Flag Detection | ✅ |
| Deep Data Flag Detection | ✅ |
| **Compression: None** | ✅ |
| **Compression: RLE** | ✅ |
| **Compression: ZIPS** (1 scanline/chunk) | ✅ |
| **Compression: ZIP** (16 scanlines/chunk) | ✅ |
| **Compression: PIZ** (wavelet + Huffman) | ✅ |
| **Compression: B44** (4×4 half-float blocks) | ✅ |
| Compression: B44A (flat-block optimization) | ✅ |
| Compression: DWAA / DWAB (DCT-based) | ❌ |
| Scanline Multi-Line Chunk Iteration | ✅ |
| Tiled Decompression | ✅ |
| Predictor Reconstruction | ✅ |
