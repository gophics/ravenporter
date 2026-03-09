# WOFF Decoder

Web Open Font Format 1.0 decoder with zlib decompression and SFNT metadata extraction.

## Extensions

`.woff`

## Supported Features

| Feature | Status |
| :--- | :---: |
| WOFF Header Parsing (44 bytes) | ✅ |
| Table Directory Parsing | ✅ |
| Zlib Table Decompression | ✅ |
| Uncompressed Table Passthrough | ✅ |
| SFNT Reconstruction | ✅ |
| `name` Table Extraction | ✅ |
| `OS/2` Table Extraction | ✅ |
| `head` Table Extraction | ✅ |
| `maxp` Table Extraction | ✅ |
