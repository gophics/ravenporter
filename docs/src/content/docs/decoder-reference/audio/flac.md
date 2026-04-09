---
title: FLAC Decoder
description: Specification support reference for RavenPorter's FLAC importer.
---

RavenPorter imports FLAC audio from `.flac` files. It covers the standard frame coding modes, channel decorrelation schemes, and metadata blocks used in lossless source audio.

## Extensions

`.flac`

## Supported Specification Features

- `CONSTANT`, `VERBATIM`, `FIXED`, and `LPC` subframe modes
- Rice entropy coding with `RICE` and `RICE2`
- Left-side, right-side, and mid-side channel decorrelation
- 8-bit, 16-bit, 24-bit, and 32-bit sample depths
- Mono, stereo, and multichannel layouts
- Vorbis comment metadata and `PICTURE` blocks

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- FLAC metadata blocks beyond `VORBIS_COMMENT` and `PICTURE` are not surfaced.

## Notes

- None.

