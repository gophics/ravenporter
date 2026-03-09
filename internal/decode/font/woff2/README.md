# WOFF2 Decoder

Web Open Font Format 2.0 decoder with Brotli decompression and SFNT metadata extraction.

## Extensions

`.woff2`

## Supported Features

| Feature | Status |
| :--- | :---: |
| WOFF2 Header Parsing (48 bytes) | ✅ |
| Signature Validation | ✅ |
| Known-Tag Table Directory Parsing | ✅ |
| Custom Tag Support | ✅ |
| UIntBase128 Decoding | ✅ |
| Brotli Decompression (pooled reader) | ✅ |
| SFNT Reconstruction | ✅ |
| `name` Table Extraction | ✅ |
| `OS/2` Table Extraction | ✅ |
| `head` Table Extraction | ✅ |
| `maxp` Table Extraction | ✅ |
