---
title: Opus Decoder
description: Specification support reference for RavenPorter's Ogg Opus importer.
---

RavenPorter imports Opus audio from `.opus` files carried inside the Ogg container. It covers stream metadata, gain metadata, album art metadata, and mono/stereo Ogg Opus decode.

## Extensions

`.opus`

## Supported Specification Features

- Ogg Opus stream metadata including channel counts, granule positions, and pre-skip handling
- Common mono and stereo stream layouts
- `OpusTags` / Vorbis comment metadata
- R128 gain from the stream header plus track/album tags
- Embedded album art via `METADATA_BLOCK_PICTURE`
- Chained Ogg streams

## Unimplemented Runtime-Relevant Features

- Only channel mapping family `0` mono/stereo streams are supported for sample decode.
- Multistream channel mappings such as family `1` surround layouts and ambisonic families `2` and `3` are not supported.

## Out Of Scope For RavenPorter

None.

## Notes

None.

