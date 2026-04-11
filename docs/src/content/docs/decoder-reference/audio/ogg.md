---
title: Ogg Decoder
description: Specification support reference for RavenPorter's Ogg/Vorbis importer.
---

RavenPorter imports Ogg/Vorbis audio from `.ogg` and `.oga` files. It covers stream metadata, selected Vorbis comment fields, embedded album art, and both single-stream and chained Vorbis packaging when the chained streams keep the same sample rate and channel layout.

## Extensions

`.ogg`, `.oga`

## Supported Specification Features

- Ogg/Vorbis stream metadata including sample rate and channel count
- Common channel layouts inferred from the Vorbis stream channel count
- Vorbis comment fields for title, artist, album, genre, and comment
- Embedded album art
- Chained Vorbis logical bitstreams when every chained stream keeps the same sample rate and channel layout

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Ogg codecs other than Vorbis are not handled by this decoder.
- Only common Vorbis comment fields and `METADATA_BLOCK_PICTURE` album art are surfaced.

## Notes

None.

