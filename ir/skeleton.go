package ir

// Skeleton holds a joint hierarchy and inverse bind matrices.
type Skeleton struct {
	Name                string
	Joints              []int // Indices into Scene.Nodes
	RootIdx             int   // Index within Joints of the root joint
	InverseBindMatrices [][16]float32
}
