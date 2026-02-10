package ir

import "testing"

func TestAssetNormalizeGraph(t *testing.T) {
	asset := &Asset{
		Scenes: []*Scene{{Name: "Scene", RootNodes: []int{0}}},
		Nodes: []Node{
			{Name: "Root", ParentIndex: 0, Children: []int{1, 1}},
			{Name: "Child"},
		},
	}

	asset.NormalizeGraph()

	if asset.Nodes[0].ParentIndex != NoIndex {
		t.Fatalf("root parent = %d, want %d", asset.Nodes[0].ParentIndex, NoIndex)
	}
	if asset.Nodes[1].ParentIndex != 0 {
		t.Fatalf("child parent = %d, want 0", asset.Nodes[1].ParentIndex)
	}
	if len(asset.RootNodes) != 1 || asset.RootNodes[0] != 0 {
		t.Fatalf("root nodes = %v, want [0]", asset.RootNodes)
	}
	if len(asset.Scenes) != 1 || len(asset.Scenes[0].RootNodes) != 1 || asset.Scenes[0].RootNodes[0] != 0 {
		t.Fatalf("scene roots = %v, want [0]", asset.Scenes[0].RootNodes)
	}
	if len(asset.Nodes[0].Children) != 1 || asset.Nodes[0].Children[0] != 1 {
		t.Fatalf("children = %v, want [1]", asset.Nodes[0].Children)
	}
}

func TestAssetWalkNodesSkipsCycles(t *testing.T) {
	asset := &Asset{
		RootNodes: []int{0},
		Nodes: []Node{{
			Name:     "Root",
			Children: []int{0},
		}},
	}

	visited := 0
	asset.WalkNodes(0, func(_ int, _ *Node) bool {
		visited++
		return true
	})

	if visited != 1 {
		t.Fatalf("visited = %d, want 1", visited)
	}
}

func TestAssetWorldMatrixStopsOnSelfParent(t *testing.T) {
	asset := &Asset{
		RootNodes: []int{0},
		Nodes: []Node{{
			Name:        "Root",
			ParentIndex: 0,
			Transform: Transform{
				Translation: [3]float32{3, 4, 5},
				Rotation:    [4]float32{0, 0, 0, 1},
				Scale:       [3]float32{1, 1, 1},
			},
		}},
	}

	got := asset.WorldMatrix(0)
	want := asset.Nodes[0].LocalMatrix()
	if got != want {
		t.Fatalf("world matrix = %v, want %v", got, want)
	}
}
