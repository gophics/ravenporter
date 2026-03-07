# Opus Decoder

## Format

Opus audio inside OGG container.

## Extensions

`.opus`

## Supported Features

| Feature | Status |
|---------|:------:|
| Channel count | ✅ |
| Duration (with pre-skip correction) | ✅ |
| Mono / Stereo | ✅ |
| Metadata (OpusTags / Vorbis comment) | ✅ |
| Sample decoding (pion/opus native SILK) | ✅ |
| R128 gain (header + track/album tag) | ✅ |
| Chained Ogg streams | ✅ |
| Embedded album art (METADATA_BLOCK_PICTURE) | ✅ |

## Not Supported

- CELT mode and Hybrid samples (Unsupported by Pion)
