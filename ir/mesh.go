package ir

// Mesh holds geometry data with one or more primitives.
type Mesh struct {
	Name         string
	Primitives   []Primitive
	MorphWeights []float32     // Default morph weights
	BoundingBox  [2][3]float32 // [min, max] AABB
}

// Primitive is a sub-mesh with a single material and topology.
type Primitive struct {
	Mode          PrimitiveMode
	MaterialIndex int // NoIndex = no material
	Data          MeshData
	MorphTargets  []MorphTarget
}

// MeshData stores vertex attributes in a Struct-of-Arrays layout.
// Nil slices indicate the attribute is not present.
// All non-nil attribute slices must have the same length as VertexCount.
type MeshData struct {
	VertexCount int

	// Geometry (always present).
	Positions [][3]float32
	Indices   []uint32

	// Optional attributes â€” nil if not present in source format.
	Normals   [][3]float32
	Tangents  [][4]float32 // xyz = tangent, w = handedness
	TexCoord0 [][2]float32
	TexCoord1 [][2]float32
	TexCoord2 [][2]float32
	TexCoord3 [][2]float32
	Colors0   [][4]float32 // RGBA, always normalized float32
	Joints0   [][4]uint16
	Joints1   [][4]uint16 // >4 bone influences
	Weights0  [][4]float32
	Weights1  [][4]float32 // >4 bone influences

	// Per-face metadata - nil if not available.
	SmoothGroups []int // per-face smooth group ID (from OBJ `s` or 3DS smooth chunks)
}

// HasNormals reports whether normals are present.
func (m *MeshData) HasNormals() bool { return m.Normals != nil }

// HasTangents reports whether tangents are present.
func (m *MeshData) HasTangents() bool { return m.Tangents != nil }

// HasUVs reports whether UV set 0 is present.
func (m *MeshData) HasUVs() bool { return m.TexCoord0 != nil }

// HasColors reports whether vertex colors are present.
func (m *MeshData) HasColors() bool { return m.Colors0 != nil }

// HasBones reports whether bone weight data is present.
func (m *MeshData) HasBones() bool { return m.Joints0 != nil }

// HasIndices reports whether index data is present.
func (m *MeshData) HasIndices() bool { return m.Indices != nil }

// MorphTarget stores per-vertex deltas for blend shapes.
// Sparse: Indices identifies which base vertices are affected.
type MorphTarget struct {
	Name      string
	Indices   []uint32 // Indices into base mesh vertices (sparse)
	Positions [][3]float32
	Normals   [][3]float32
	Tangents  [][3]float32
}
