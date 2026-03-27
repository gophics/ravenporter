package models

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"math"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type findInstancesStep struct{}

func (s *findInstancesStep) Name() string      { return "FindInstances" }
func (s *findInstancesStep) Flag() core.PPFlag { return core.PPFindInstances }

func (s *findInstancesStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	meshHashes := make(map[[32]byte]int, len(asset.Meshes))
	remap := make(map[int]int, len(asset.Meshes))
	h := sha256.New()

	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			remap[i] = ir.NoIndex
			continue
		}

		digest := computeMeshHash(mesh, h)
		if originalIdx, exists := meshHashes[digest]; exists {
			remap[i] = originalIdx
		} else {
			meshHashes[digest] = i
			remap[i] = i
		}
	}

	for i := range asset.Nodes {
		n := &asset.Nodes[i]
		if n.MeshIndex != ir.NoIndex {
			if target, ok := remap[n.MeshIndex]; ok && target != ir.NoIndex {
				n.MeshIndex = target
			}
		}
	}

	newMeshes := make([]*ir.Mesh, 0, len(asset.Meshes))
	consolidateMap := make(map[int]int, len(asset.Meshes))
	for i := range asset.Meshes {
		if target := remap[i]; target == i {
			consolidateMap[i] = len(newMeshes)
			newMeshes = append(newMeshes, asset.Meshes[i])
		}
	}

	if len(newMeshes) < len(asset.Meshes) {
		for i := range asset.Nodes {
			n := &asset.Nodes[i]
			if n.MeshIndex != ir.NoIndex {
				if mapped, ok := consolidateMap[n.MeshIndex]; ok {
					n.MeshIndex = mapped
				}
			}
		}
		asset.Meshes = newMeshes
	}

	return asset, nil
}

func computeMeshHash(m *ir.Mesh, h hash.Hash) [32]byte {
	h.Reset()
	var buf [16]byte
	for i := range m.Primitives {
		p := &m.Primitives[i]
		binary.LittleEndian.PutUint32(buf[:], uint32(p.Mode))           //nolint:gosec // enum fits
		binary.LittleEndian.PutUint32(buf[4:], uint32(p.MaterialIndex)) //nolint:gosec // index fits
		h.Write(buf[:8])
		for _, pos := range p.Data.Positions {
			writeVec3(h, buf[:], pos)
		}
		for _, norm := range p.Data.Normals {
			writeVec3(h, buf[:], norm)
		}
		for _, id := range p.Data.Indices {
			binary.LittleEndian.PutUint32(buf[:], id)
			h.Write(buf[:4])
		}
		for _, uv := range p.Data.TexCoord0 {
			writeVec2(h, buf[:], uv)
		}
	}
	var res [32]byte
	copy(res[:], h.Sum(nil))
	return res
}

func writeVec3(h hash.Hash, buf []byte, v [3]float32) {
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(v[0]))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(v[1]))
	binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(v[2]))
	h.Write(buf[:12])
}

func writeVec2(h hash.Hash, buf []byte, v [2]float32) {
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(v[0]))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(v[1]))
	h.Write(buf[:8])
}
