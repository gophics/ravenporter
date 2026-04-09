---
title: AIFF Decoder
description: Specification support reference for RavenPorter's AIFF and AIFF-C importer.
---

RavenPorter imports AIFF and AIFF-C audio from `.aiff` and `.aif` files. It covers the common PCM, floating-point, companded, and ADPCM variants RavenPorter maps from interchange audio assets.

## Extensions

`.aiff`, `.aif`

## Supported Specification Features

- AIFF uncompressed audio and AIFF-C compression tags `NONE`, `sowt`, `fl32`, `alaw`, `ulaw`, and `ima4`
- 8-bit, 16-bit, 24-bit, and 32-bit sample depths
- Mono, stereo, and multichannel layouts
- Sustain and release loop points
- Text metadata chunks such as `NAME`, `AUTH`, and `ANNO`

## Unimplemented Runtime-Relevant Features

- AIFF-C codec tags beyond `NONE`, `sowt`, `fl32`, `alaw`, `ulaw`, and `ima4` are not supported.

## Out Of Scope For RavenPorter

- Optional AIFF chunks beyond the current `NAME`, `AUTH`, `ANNO`, `MARK`, and `INST` subset are not imported.

## Notes

- None.

