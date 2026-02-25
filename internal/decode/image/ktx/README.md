# KTX / KTX2 Decoder

Khronos Texture container parser with GPU-compressed passthrough.

## Extensions

`.ktx`, `.ktx2`

## Supported Features

| Feature | Status |
| :--- | :---: |
| KTX1 Header Parsing | ✅ |
| KTX2 Header Parsing | ✅ |
| VkFormat Identification (BC1–BC7, ASTC, ETC2) | ✅ |
| GL Internal Format Identification (S3TC, BPTC, ETC2, ASTC) | ✅ |
| Uncompressed vs Compressed Detection (glFormat) | ✅ |
| Mipmap Count Extraction | ✅ |
| Zstd Supercompression Inflate | ✅ |
