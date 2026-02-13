package ir

import "testing"

func FuzzAssetGraphOps(f *testing.F) {
	f.Add([]byte{0})
	f.Add([]byte{1, 1, 0, 1, 0, 0})
	f.Add([]byte{2, 1, 2, 0, 1, 1, 0, 0, 0, 1})

	f.Fuzz(func(_ *testing.T, data []byte) {
		asset := fuzzAsset(data)
		asset.NormalizeGraph()

		_ = asset.PrimaryScene()
		_ = asset.PrimaryRootNodes()
		asset.WalkNodes(0, func(_ int, _ *Node) bool { return true })
		_, _ = asset.SceneBoundingBox(0)

		for i := range asset.Nodes {
			_ = asset.WorldMatrix(i)
		}
	})
}

func fuzzAsset(data []byte) *Asset {
	cursor := fuzzCursor{data: data}

	meshCount := cursor.count(2)
	meshes := make([]*Mesh, meshCount)
	for i := range meshes {
		meshes[i] = &Mesh{
			Name: "Mesh",
			Primitives: []Primitive{{
				Mode: Triangles,
				Data: MeshData{
					VertexCount: 3,
					Positions: [][3]float32{
						{cursor.coord(), cursor.coord(), cursor.coord()},
						{cursor.coord(), cursor.coord(), cursor.coord()},
						{cursor.coord(), cursor.coord(), cursor.coord()},
					},
				},
			}},
		}
	}

	nodeCount := cursor.count(6)
	nodes := make([]Node, nodeCount)
	for i := range nodes {
		nodes[i] = Node{
			Name:        "Node",
			Visible:     cursor.flag(),
			ParentIndex: cursor.parent(nodeCount),
			MeshIndex:   cursor.ref(meshCount),
			Transform: Transform{
				Translation: [3]float32{cursor.coord(), cursor.coord(), cursor.coord()},
				Rotation:    [4]float32{0, 0, 0, 1},
				Scale:       [3]float32{1, 1, 1},
			},
		}
		childCount := cursor.count(3)
		if childCount > 0 {
			nodes[i].Children = make([]int, childCount)
			for j := range nodes[i].Children {
				nodes[i].Children[j] = cursor.index(nodeCount)
			}
		}
	}

	sceneCount := cursor.count(2)
	scenes := make([]*Scene, sceneCount)
	for i := range scenes {
		rootCount := cursor.count(3)
		roots := make([]int, rootCount)
		for j := range roots {
			roots[j] = cursor.index(nodeCount)
		}
		scenes[i] = &Scene{Name: "Scene", RootNodes: roots}
	}

	rootCount := cursor.count(3)
	roots := make([]int, rootCount)
	for i := range roots {
		roots[i] = cursor.index(nodeCount)
	}

	return &Asset{
		DefaultScene: cursor.defaultScene(sceneCount),
		Scenes:       scenes,
		RootNodes:    roots,
		Nodes:        nodes,
		Meshes:       meshes,
	}
}

type fuzzCursor struct {
	data []byte
	pos  int
}

func (c *fuzzCursor) next() byte {
	if len(c.data) == 0 {
		return 0
	}
	value := c.data[c.pos%len(c.data)]
	c.pos++
	return value
}

func (c *fuzzCursor) flag() bool {
	return c.next()%2 == 1
}

func (c *fuzzCursor) count(limit int) int {
	if limit <= 0 {
		return 0
	}
	return int(c.next()) % (limit + 1)
}

func (c *fuzzCursor) index(limit int) int {
	if limit <= 0 {
		return 0
	}
	return int(c.next()) % limit
}

func (c *fuzzCursor) ref(limit int) int {
	if limit <= 0 || c.next()%3 == 0 {
		return NoIndex
	}
	return c.index(limit)
}

func (c *fuzzCursor) parent(limit int) int {
	if limit <= 0 || c.next()%3 == 0 {
		return NoIndex
	}
	return c.index(limit)
}

func (c *fuzzCursor) defaultScene(limit int) int {
	if limit <= 0 {
		return NoIndex
	}
	return c.index(limit)
}

func (c *fuzzCursor) coord() float32 {
	return float32(int8(c.next())) / 8
}
