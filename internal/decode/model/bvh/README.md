# BVH Decoder

Biovision Hierarchy (`.bvh`) motion capture file decoder.

## Supported Features

| Feature | Status |
|:---|:---:|
| HIERARCHY section parsing | ✅ |
| ROOT joint definition | ✅ |
| Nested JOINT hierarchy | ✅ |
| End Site terminals | ✅ |
| OFFSET (parent-relative translation) | ✅ |
| CHANNELS (per-joint channel layout) | ✅ |
| 6-DOF channels (XYZ position + XYZ rotation) | ✅ |
| 3-DOF channels (rotation only) | ✅ |
| Arbitrary rotation order (ZXY, XYZ, YZX, etc.) | ✅ |
| MOTION section parsing | ✅ |
| Frame count (`Frames:`) | ✅ |
| Frame time (`Frame Time:`) | ✅ |
| Per-frame channel data parsing | ✅ |
| Euler angles → Quaternion conversion | ✅ |
