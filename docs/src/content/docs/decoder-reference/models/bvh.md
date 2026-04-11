---
title: BVH Decoder
description: Specification support reference for RavenPorter's BVH importer.
---

RavenPorter imports Biovision Hierarchy motion-capture data from `.bvh` files. It includes skeletal hierarchy layout and frame-based animation data.

## Extensions

`.bvh`

## Supported Specification Features

- `HIERARCHY` sections with `ROOT`, nested `JOINT` nodes, and `End Site` terminals
- Parent-relative `OFFSET` transforms and per-joint `CHANNELS` layouts
- 6-DOF roots and rotation-only joints
- Scale channels when they are present as `Xscale`, `Yscale`, and `Zscale`
- Arbitrary Euler rotation orders such as `ZXY`, `XYZ`, and `YZX`
- `MOTION` sections with frame counts, frame time, and per-frame channel samples

## Unimplemented Runtime-Relevant Features

- Custom channel types beyond translation, rotation, and scale are not supported.

## Out Of Scope For RavenPorter

- Data outside the hierarchy and motion sections is not part of this importer.

## Notes

None.

