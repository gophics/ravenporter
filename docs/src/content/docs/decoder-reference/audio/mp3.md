---
title: MP3 Decoder
description: Specification support reference for RavenPorter's MP3 importer.
---

RavenPorter imports MPEG Layer III audio from `.mp3` files. It covers frame parsing, stereo modes, common MPEG variants, and the basic ID3 metadata fields typically carried in source music and effects assets.

## Extensions

`.mp3`

## Supported Specification Features

- MPEG-1 Layer III plus MPEG-2 and MPEG-2.5 low-sampling-frequency variants
- Frame headers with sample-rate, bitrate, and channel-mode metadata
- Basic ID3v2 metadata for title, artist, album, and genre
- VBR headers such as `Xing` and `VBRI`
- Side-information parsing, scalefactors, Huffman decoding, requantization, alias reduction, IMDCT, and synthesis filterbank stages
- Mid/side stereo and intensity stereo
- Free-format bitrate decoding and CRC-16 checking

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- ID3 metadata beyond title, artist, album, and genre is not surfaced, including comments, attached pictures, lyrics, chapters, custom text frames, and URL frames.
- ID3v1 and APE metadata are not imported.

## Notes

None.

