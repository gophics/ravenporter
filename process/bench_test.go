package process_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

const benchVertexCount = 3000

func buildBenchScene() *ir.Asset {
	positions := make([][3]float32, benchVertexCount)
	indices := make([]uint32, benchVertexCount)
	uvs := make([][2]float32, benchVertexCount)
	for i := range benchVertexCount {
		fi := float32(i)
		positions[i] = [3]float32{fi, fi + 1, fi + 2}
		indices[i] = uint32(i)
		uvs[i] = [2]float32{fi * 0.01, fi * 0.01}
	}
	return &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					VertexCount: benchVertexCount,
					Positions:   positions,
					Indices:     indices,
					TexCoord0:   uvs,
				},
			}},
		}},
		Nodes:     []ir.Node{{Name: "root", MeshIndex: 0}},
		RootNodes: []int{0},
	}
}

func BenchmarkTriangulate(b *testing.B) {
	scene := buildBenchScene()
	b.ReportAllocs()
	for b.Loop() {
		_ = process.Apply(scene, process.PPTriangulate, process.Options{})
	}
}

func BenchmarkGenNormals(b *testing.B) {
	scene := buildBenchScene()
	b.ReportAllocs()
	for b.Loop() {
		scene.Meshes[0].Primitives[0].Data.Normals = nil
		_ = process.Apply(scene, process.PPGenNormals, process.Options{})
	}
}

func BenchmarkCalcTangentSpace(b *testing.B) {
	scene := buildBenchScene()
	_ = process.Apply(scene, process.PPGenNormals, process.Options{})
	b.ReportAllocs()
	for b.Loop() {
		scene.Meshes[0].Primitives[0].Data.Tangents = nil
		_ = process.Apply(scene, process.PPCalcTangentSpace, process.Options{})
	}
}

func BenchmarkWeld(b *testing.B) {
	scene := buildBenchScene()
	b.ReportAllocs()
	for b.Loop() {
		_ = process.Apply(scene, process.PPJoinIdenticalVertices, process.Options{})
	}
}

func BenchmarkRemoveDegenerates(b *testing.B) {
	scene := buildBenchScene()
	b.ReportAllocs()
	for b.Loop() {
		_ = process.Apply(scene, process.PPRemoveDegenerates, process.Options{})
	}
}

func BenchmarkPresetQuality(b *testing.B) {
	scene := buildBenchScene()
	b.ReportAllocs()
	for b.Loop() {
		scene.Meshes[0].Primitives[0].Data.Normals = nil
		scene.Meshes[0].Primitives[0].Data.Tangents = nil
		_ = process.Apply(scene, process.PresetQuality, process.Options{})
	}
}

func BenchmarkOptimizeCache(b *testing.B) {
	scene := buildBenchScene()
	b.ReportAllocs()
	for b.Loop() {
		_ = process.Apply(scene, process.PPOptimizeCache, process.Options{})
	}
}

func BenchmarkSplitByBoneCount(b *testing.B) {
	scene := buildBoneScene()
	b.ReportAllocs()
	for b.Loop() {
		s := cloneBoneScene(scene)
		_ = process.Apply(s, process.PPSplitByBoneCount, process.Options{MaxBonesPerMesh: 4})
	}
}

const boneVertCount = 1200

func buildBoneScene() *ir.Asset {
	positions := make([][3]float32, boneVertCount)
	indices := make([]uint32, boneVertCount)
	joints := make([][4]uint16, boneVertCount)
	weights := make([][4]float32, boneVertCount)

	for i := range boneVertCount {
		fi := float32(i)
		positions[i] = [3]float32{fi, fi + 1, fi + 2}
		indices[i] = uint32(i)
		boneID := uint16(i % 8)
		joints[i] = [4]uint16{boneID, 0, 0, 0}
		weights[i] = [4]float32{1, 0, 0, 0}
	}
	return &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					VertexCount: boneVertCount,
					Positions:   positions,
					Indices:     indices,
					Joints0:     joints,
					Weights0:    weights,
				},
			}},
		}},
	}
}

func cloneBoneScene(src *ir.Asset) *ir.Asset {
	d := src.Meshes[0].Primitives[0].Data
	newPositions := make([][3]float32, len(d.Positions))
	copy(newPositions, d.Positions)
	newIndices := make([]uint32, len(d.Indices))
	copy(newIndices, d.Indices)
	newJoints := make([][4]uint16, len(d.Joints0))
	copy(newJoints, d.Joints0)
	newWeights := make([][4]float32, len(d.Weights0))
	copy(newWeights, d.Weights0)

	return &ir.Asset{
		Meshes: []*ir.Mesh{{
			Primitives: []ir.Primitive{{
				Mode: ir.Triangles,
				Data: ir.MeshData{
					VertexCount: boneVertCount,
					Positions:   newPositions,
					Indices:     newIndices,
					Joints0:     newJoints,
					Weights0:    newWeights,
				},
			}},
		}},
	}
}

func BenchmarkDebone(b *testing.B) {
	joints := make([]int, 8)
	ibm := make([][16]float32, 8)
	for i := range joints {
		joints[i] = i
	}
	nodes := make([]ir.Node, 8)
	for i := range nodes {
		nodes[i] = ir.Node{LODGroupIndex: ir.NoIndex, Name: "j"}
		if i > 0 {
			nodes[0].Children = append(nodes[0].Children, i)
		}
	}
	scene := buildBoneScene()
	b.ReportAllocs()
	for b.Loop() {
		s := cloneBoneScene(scene)
		skelJoints := make([]int, len(joints))
		copy(skelJoints, joints)
		ibmCopy := make([][16]float32, len(ibm))
		copy(ibmCopy, ibm)
		nodesCopy := make([]ir.Node, len(nodes))
		copy(nodesCopy, nodes)
		s.Skeletons = []*ir.Skeleton{{Joints: skelJoints, InverseBindMatrices: ibmCopy}}
		s.Nodes = nodesCopy
		_ = process.Apply(s, process.PPDebone, process.Options{DeboneThreshold: 0.5})
	}
}
