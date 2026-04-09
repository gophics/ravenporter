---
title: WAV Decoder
description: Specification support reference for RavenPorter's WAV importer.
---

RavenPorter imports RIFF/WAVE audio from `.wav` files. It covers classic `RIFF`, `RF64`, and `BW64` containers, uncompressed PCM and floating-point data, several common compressed encodings, and the subset of RIFF chunks RavenPorter maps into cue, loop, and descriptive audio metadata.

## Extensions

`.wav`

## Supported Specification Features

- PCM integer audio, IEEE 754 floating-point audio, and `WAVEFORMATEXTENSIBLE`
- Classic RIFF, RF64, and BW64 container variants
- `A-law` and `mu-law` G.711 audio
- 8-bit, 16-bit, 24-bit, and 32-bit sample depths
- Mono, stereo, and common multichannel layouts, including preserved `WAVEFORMATEXTENSIBLE` speaker masks when present
- `MS-ADPCM`, `IMA ADPCM` (DVI), and `MP3-in-WAV` (`0x0055`) payloads
- `LIST/INFO` metadata tags, `cue ` markers with `LIST/adtl` cue labels, and `smpl` loop points

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- Broadcast and authoring metadata chunks such as `bext`, `iXML`, and `axml` are not surfaced.

## Notes

- None.

