---
title: STL Decoder
description: Specification support reference for RavenPorter's STL importer.
---

RavenPorter imports STL mesh data from both ASCII and binary files. It also recognizes the color conventions used by some extended binary STL exporters.

## Extensions

`.stl`

## Supported Specification Features

- ASCII and binary STL files
- Facet normals and vertex positions
- `solid` name parsing
- Binary per-facet RGB555 color attributes
- Header-level `COLOR=` part color tags

## Unimplemented Runtime-Relevant Features

None.

## Out Of Scope For RavenPorter

- STL color support is non-standard; RavenPorter recognizes per-facet RGB555 attribute colors and Magics-style `COLOR=` header tags, but other binary color conventions may import incorrectly or be ignored.

## Notes

None.

