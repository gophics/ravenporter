---
title: Alembic Decoder
description: Specification support reference for RavenPorter's Alembic importer.
---

RavenPorter imports Alembic scene archives from `.abc` files. It handles the mesh, transform, camera, animation, and material-reference features most Alembic exchange files use.

## Extensions

`.abc`

## Supported Specification Features

- Ogawa-based Alembic archives
- Mesh geometry with vertex positions, face indices, normals, UV sets (`uv` and `st`), and vertex colors (`Cs`)
- Polygon face triangulation during import
- `Xform` nodes with 4 x 4 transforms, time sampling, and animated samples
- Camera prims and archive metadata/version fields
- Material references carried through geometry parameters

## Unimplemented Runtime-Relevant Features

- Legacy HDF5-based Alembic archives are not supported.

## Out Of Scope For RavenPorter

None.

## Notes

None.

