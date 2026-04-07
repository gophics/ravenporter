package ir

import (
	"fmt"
	"math"

	"github.com/gophics/ravenporter/internal/mathx"
)

const triangleVertexStride = 3

// Asset is the top-level container for all decoded asset data.
// After Import returns, the Asset is fully self-contained on the GC heap.
// All mmap file mappings are released during decode; no OS resources are held.
type Asset struct {
	Name            string
	UpAxis          Axis
	Unit            float64
	DefaultScene    int
	Scenes          []*Scene
	RootNodes       []int
	Nodes           []Node
	Meshes          []*Mesh
	Materials       []*Material
	Textures        []*Texture
	Images          []*ImageAsset
	Animations      []*Animation
	Skeletons       []*Skeleton
	Cameras         []*Camera
	Lights          []*Light
	AudioClips      []*AudioClip
	Fonts           []*Font
	LODGroups       []*LODGroup
	CollisionMeshes []*CollisionMesh
	Metadata        AssetMetadata
}

// NewAsset returns a new asset with provenance initialized for the source format.
func NewAsset(format FormatID) *Asset {
	return &Asset{
		DefaultScene: NoIndex,
		Unit:         1,
		Metadata: AssetMetadata{
			SourceFormat: format,
		},
	}
}

// NewAssetWithScene returns a new asset with a primary scene entry.
func NewAssetWithScene(format FormatID, name string) (*Asset, *Scene) {
	asset := NewAsset(format)
	scene := &Scene{Name: name}
	asset.Scenes = []*Scene{scene}
	asset.DefaultScene = 0
	return asset, scene
}

// Close is a no-op; ir.Asset holds no resources.
func (a *Asset) Close() error { return nil }

// AssetMetadata holds provenance and source information.
type AssetMetadata struct {
	SourceFormat    FormatID
	SourceVersion   string
	Generator       string
	CreationTime    string
	ExtraProperties map[string]string
}

// PrimaryScene returns the default scene when present, or the first scene.
func (a *Asset) PrimaryScene() *Scene {
	if a == nil {
		return nil
	}
	if len(a.Scenes) == 0 {
		if len(a.RootNodes) == 0 {
			return nil
		}
		return &Scene{Name: a.Name, RootNodes: a.RootNodes}
	}
	if scene := a.primarySceneEntry(); scene != nil {
		return scene
	}
	return nil
}

// PrimaryRootNodes returns the root node indices from the primary scene, or Asset.RootNodes.
func (a *Asset) PrimaryRootNodes() []int {
	if a == nil {
		return nil
	}
	if scene := a.primarySceneEntry(); scene != nil {
		return scene.RootNodes
	}
	return a.RootNodes
}

// WorldMatrix computes the world transform by walking the parent chain.
func (a *Asset) WorldMatrix(idx int) mathx.Mat4 {
	if a == nil || idx < 0 || idx >= len(a.Nodes) {
		return mathx.Ident4()
	}

	m := a.Nodes[idx].LocalMatrix()
	fast := a.nextParentIndex(a.nextParentIndex(a.Nodes[idx].ParentIndex))

	for parent := a.nextParentIndex(a.Nodes[idx].ParentIndex); parent != NoIndex; parent = a.nextParentIndex(parent) {
		if parent == idx {
			break
		}
		m = a.Nodes[parent].LocalMatrix().Mul4(m)
		if parent == fast {
			break
		}
		fast = a.nextParentIndex(a.nextParentIndex(fast))
	}
	return m
}

// WalkNodes performs a depth-first traversal for a single scene.
// Return false from fn to stop the walk.
func (a *Asset) WalkNodes(sceneIndex int, fn func(idx int, n *Node) bool) {
	if a == nil || fn == nil {
		return
	}
	scene := a.PrimaryScene()
	if len(a.Scenes) > 0 {
		if sceneIndex < 0 || sceneIndex >= len(a.Scenes) {
			return
		}
		scene = a.Scenes[sceneIndex]
	}
	if scene == nil {
		return
	}
	visited := make([]bool, len(a.Nodes))
	for _, root := range scene.RootNodes {
		if !walkNode(a.Nodes, root, visited, fn) {
			return
		}
	}
}

func walkNode(nodes []Node, idx int, visited []bool, fn func(int, *Node) bool) bool {
	if idx < 0 || idx >= len(nodes) {
		return true
	}
	if visited[idx] {
		return true
	}
	visited[idx] = true
	if !fn(idx, &nodes[idx]) {
		return false
	}
	for _, child := range nodes[idx].Children {
		if !walkNode(nodes, child, visited, fn) {
			return false
		}
	}
	return true
}

// FindNode returns the index of the first node with the given name, or NoIndex.
func (a *Asset) FindNode(name string) int {
	if a == nil {
		return NoIndex
	}
	for i := range a.Nodes {
		if a.Nodes[i].Name == name {
			return i
		}
	}
	return NoIndex
}

// FindMesh returns the index of the first mesh with the given name, or NoIndex.
func (a *Asset) FindMesh(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, m := range a.Meshes {
		if m != nil && m.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindMaterial returns the index of the first material with the given name, or NoIndex.
func (a *Asset) FindMaterial(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, m := range a.Materials {
		if m != nil && m.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindAnimation returns the index of the first animation with the given name, or NoIndex.
func (a *Asset) FindAnimation(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, anim := range a.Animations {
		if anim != nil && anim.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindImage returns the index of the first image with the given name, or NoIndex.
func (a *Asset) FindImage(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, img := range a.Images {
		if img != nil && img.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindCamera returns the index of the first camera with the given name, or NoIndex.
func (a *Asset) FindCamera(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, cam := range a.Cameras {
		if cam != nil && cam.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindLight returns the index of the first light with the given name, or NoIndex.
func (a *Asset) FindLight(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, l := range a.Lights {
		if l != nil && l.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindSkeleton returns the index of the first skeleton with the given name, or NoIndex.
func (a *Asset) FindSkeleton(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, sk := range a.Skeletons {
		if sk != nil && sk.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindAudioClip returns the index of the first audio clip with the given name, or NoIndex.
func (a *Asset) FindAudioClip(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, clip := range a.AudioClips {
		if clip != nil && clip.Name == name {
			return i
		}
	}
	return NoIndex
}

// FindFont returns the index of the first font with the given name, or NoIndex.
func (a *Asset) FindFont(name string) int {
	if a == nil {
		return NoIndex
	}
	for i, f := range a.Fonts {
		if f != nil && f.Name == name {
			return i
		}
	}
	return NoIndex
}

// TotalVertexCount returns the sum of vertex counts across all mesh primitives.
func (a *Asset) TotalVertexCount() int {
	if a == nil {
		return 0
	}
	total := 0
	for _, mesh := range a.Meshes {
		if mesh == nil {
			continue
		}
		for i := range mesh.Primitives {
			total += mesh.Primitives[i].Data.VertexCount
		}
	}
	return total
}

// TotalTriangleCount returns the sum of triangle counts across all mesh primitives.
func (a *Asset) TotalTriangleCount() int {
	if a == nil {
		return 0
	}
	total := 0
	for _, mesh := range a.Meshes {
		if mesh == nil {
			continue
		}
		for i := range mesh.Primitives {
			primitive := &mesh.Primitives[i]
			if primitive.Mode != Triangles {
				continue
			}
			if primitive.Data.HasIndices() {
				total += len(primitive.Data.Indices) / triangleVertexStride
				continue
			}
			total += primitive.Data.VertexCount / triangleVertexStride
		}
	}
	return total
}

// SceneBoundingBox computes the world-space axis-aligned bounds for one scene.
func (a *Asset) SceneBoundingBox(sceneIndex int) (lo, hi [3]float32) {
	if a == nil {
		return [3]float32{}, [3]float32{}
	}
	if len(a.Scenes) > 0 && (sceneIndex < 0 || sceneIndex >= len(a.Scenes)) {
		return [3]float32{}, [3]float32{}
	}
	if len(a.Scenes) == 0 && len(a.RootNodes) == 0 {
		return [3]float32{}, [3]float32{}
	}

	lo = [3]float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
	hi = [3]float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}

	found := false
	a.WalkNodes(sceneIndex, func(idx int, node *Node) bool {
		if node.MeshIndex < 0 || node.MeshIndex >= len(a.Meshes) {
			return true
		}
		mesh := a.Meshes[node.MeshIndex]
		if mesh == nil {
			return true
		}

		matrix := a.WorldMatrix(idx)
		for primitiveIndex := range mesh.Primitives {
			positions := mesh.Primitives[primitiveIndex].Data.Positions
			for _, position := range positions {
				found = true
				world := matrix.Mul4x1(mathx.Vec4{position[0], position[1], position[2], 1})
				for axis := 0; axis < 3; axis++ {
					if world[axis] < lo[axis] {
						lo[axis] = world[axis]
					}
					if world[axis] > hi[axis] {
						hi[axis] = world[axis]
					}
				}
			}
		}
		return true
	})

	if !found {
		return [3]float32{}, [3]float32{}
	}
	return lo, hi
}

// FormatDescription returns a human-readable summary of the asset.
func (a *Asset) FormatDescription() string {
	if a == nil {
		return ""
	}
	return fmt.Sprintf("%s: %d scenes, %d meshes, %d materials, %d vertices, %d triangles",
		a.Metadata.SourceFormat, len(a.Scenes), len(a.Meshes), len(a.Materials),
		a.TotalVertexCount(), a.TotalTriangleCount())
}

func (a *Asset) primarySceneEntry() *Scene {
	if a == nil || len(a.Scenes) == 0 {
		return nil
	}
	if a.DefaultScene >= 0 && a.DefaultScene < len(a.Scenes) && a.Scenes[a.DefaultScene] != nil {
		return a.Scenes[a.DefaultScene]
	}
	for _, scene := range a.Scenes {
		if scene != nil {
			return scene
		}
	}
	return nil
}

func (a *Asset) nextParentIndex(idx int) int {
	if a == nil || idx < 0 || idx >= len(a.Nodes) {
		return NoIndex
	}
	return a.Nodes[idx].ParentIndex
}
