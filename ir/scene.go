package ir

import "github.com/gophics/ravenporter/internal/mathx"

// Scene identifies a scene entry inside an asset.
type Scene struct {
	Name      string
	RootNodes []int
}

// Node represents a single node in the asset graph.
type Node struct {
	Name          string
	Transform     Transform
	Visible       bool
	Mobility      MobilityState
	ParentIndex   int
	MeshIndex     int
	SkinIndex     int
	CameraIndex   int
	LightIndex    int
	LODGroupIndex int
	IsJoint       bool
	IsCollision   bool
	MorphWeights  []float32
	Children      []int
	Extras        map[string]any
}

// LocalMatrix computes the 4x4 local transform matrix from TRS or raw matrix.
func (n *Node) LocalMatrix() mathx.Mat4 {
	if n.Transform.Matrix != [16]float32{} {
		return n.Transform.Matrix
	}
	t := mathx.Vec3(n.Transform.Translation)
	s := n.Transform.Scale
	if s == [3]float32{} {
		s = [3]float32{1, 1, 1}
	}
	r := n.Transform.Rotation
	if r == [4]float32{} {
		r = [4]float32{0, 0, 0, 1}
	}
	q := mathx.Quat{W: r[3], V: mathx.Vec3{r[0], r[1], r[2]}}
	return mathx.ComposeTRS(t, q, s)
}
