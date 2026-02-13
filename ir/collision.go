package ir

// CollisionType defines the shape of a collision primitive.
type CollisionType int

const (
	// CollisionTypeBox represents a box collision shape.
	CollisionTypeBox CollisionType = iota
	// CollisionTypeSphere represents a spherical collision shape.
	CollisionTypeSphere
	// CollisionTypeCapsule represents a capsule collision shape.
	CollisionTypeCapsule
	// CollisionTypeConvexHull represents a convex hull collision mesh.
	CollisionTypeConvexHull
	// CollisionTypeMesh represents an arbitrary triangle collision mesh.
	CollisionTypeMesh
)

// CollisionMesh represents a collision primitive or mesh attached to the scene.
type CollisionMesh struct {
	Type      CollisionType
	MeshIndex int // Source mesh index when the collision shape comes from mesh data, or NoIndex if not applicable
	NodeIndex int // Node it is attached to, or ir.NoIndex if global
}
