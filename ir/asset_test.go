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

func TestFindMethods(t *testing.T) {
	asset := &Asset{
		Nodes:      []Node{{Name: "Root"}, {Name: "Child"}, {Name: ""}},
		Meshes:     []*Mesh{nil, {Name: "Cube"}, {Name: "Sphere"}},
		Materials:  []*Material{{Name: "Wood"}, nil},
		Animations: []*Animation{{Name: "Walk"}, {Name: "Run"}},
		Images:     []*ImageAsset{{Name: "albedo.png"}},
		Cameras:    []*Camera{{Name: "Main"}},
		Lights:     []*Light{{Name: "Sun"}},
		Skeletons:  []*Skeleton{{Name: "Armature"}},
		AudioClips: []*AudioClip{{Name: "Footstep"}},
		Fonts:      []*Font{{Name: "Roboto"}},
	}

	tests := []struct {
		name   string
		lookup func(string) int
		query  string
		want   int
	}{
		{"FindNode found", asset.FindNode, "Child", 1},
		{"FindNode miss", asset.FindNode, "None", NoIndex},
		{"FindMesh found", asset.FindMesh, "Sphere", 2},
		{"FindMesh nil element", asset.FindMesh, "nil", NoIndex},
		{"FindMesh miss", asset.FindMesh, "None", NoIndex},
		{"FindMaterial found", asset.FindMaterial, "Wood", 0},
		{"FindMaterial miss", asset.FindMaterial, "None", NoIndex},
		{"FindAnimation found", asset.FindAnimation, "Run", 1},
		{"FindAnimation miss", asset.FindAnimation, "None", NoIndex},
		{"FindImage found", asset.FindImage, "albedo.png", 0},
		{"FindImage miss", asset.FindImage, "None", NoIndex},
		{"FindCamera found", asset.FindCamera, "Main", 0},
		{"FindCamera miss", asset.FindCamera, "None", NoIndex},
		{"FindLight found", asset.FindLight, "Sun", 0},
		{"FindLight miss", asset.FindLight, "None", NoIndex},
		{"FindSkeleton found", asset.FindSkeleton, "Armature", 0},
		{"FindSkeleton miss", asset.FindSkeleton, "None", NoIndex},
		{"FindAudioClip found", asset.FindAudioClip, "Footstep", 0},
		{"FindAudioClip miss", asset.FindAudioClip, "None", NoIndex},
		{"FindFont found", asset.FindFont, "Roboto", 0},
		{"FindFont miss", asset.FindFont, "None", NoIndex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lookup(tt.query); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFindMethodsNilAsset(t *testing.T) {
	var asset *Asset

	finders := []struct {
		name   string
		lookup func(string) int
	}{
		{"FindNode", asset.FindNode},
		{"FindMesh", asset.FindMesh},
		{"FindMaterial", asset.FindMaterial},
		{"FindAnimation", asset.FindAnimation},
		{"FindImage", asset.FindImage},
		{"FindCamera", asset.FindCamera},
		{"FindLight", asset.FindLight},
		{"FindSkeleton", asset.FindSkeleton},
		{"FindAudioClip", asset.FindAudioClip},
		{"FindFont", asset.FindFont},
	}

	for _, f := range finders {
		t.Run(f.name, func(t *testing.T) {
			if got := f.lookup("any"); got != NoIndex {
				t.Fatalf("got %d on nil asset, want NoIndex", got)
			}
		})
	}
}
