# Changelog

All notable changes to RavenPorter will be documented in this file.

## [Unreleased]

Draco glTF import readiness.

Highlights:

- added glTF/GLB import support for `KHR_draco_mesh_compression` using pure-Go Draco decoding
- added low-allocation Draco primitive decode coverage with generated, fuzz, cache, benchmark, and Khronos fixture tests
- cleaned repo-wide lint debt so release checks pass cleanly

## [0.1.1] - 2026-04-12

Runtime fidelity update.

Highlights:

- broader image coverage, including BigTIFF, animated WebP, better BMP and TGA support, and improved PSD and KTX2 handling
- wider audio and animation coverage with more AIFF-C support and BVH scale channels
- better USDC import behavior and a cleaner decoder reference

## [0.1.0] - 2026-04-09

Initial public release.

Highlights:

- pure-Go asset ingest across models, images, audio, and fonts
- shared `ir.Asset` import target for runtime tooling and inspection
- JSON IR emission for debugging and handoff workflows
- cooked `cache` format with eager scene data and lazy media payloads
- CLI support for import, export, batch conversion, validation, inspection, and cache cooking
- docs site with guides, examples, format coverage, and architecture notes
