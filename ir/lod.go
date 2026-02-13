package ir

// LODGroup defines a set of Levels of Detail.
type LODGroup struct {
	Name   string
	Levels []LODLevel
}

// LODLevel maps a threshold to a specific node index.
type LODLevel struct {
	Threshold float32
	NodeIndex int
}
