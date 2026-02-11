package ir

// Light uses composition: exactly one of Directional, Point, hoặc Spot is non-nil.
type Light struct {
	Name        string
	Color       [3]float32  // RGB linear (default: {1,1,1})
	Temperature float32     // Temperature in Kelvin (0 = ignore)
	Intensity   float32     // Candelas (point/spot), Lux (directional)
	IESProfile  *TextureRef // Optional IES photometric profile
	Directional *DirectionalLight
	Point       *PointLight
	Spot        *SpotLight
}

// DirectionalLight has no extra fields (direction from node transform).
type DirectionalLight struct{}

// PointLight holds point light parameters.
type PointLight struct {
	Range        float32 // Max distance (0 = infinite)
	SourceRadius float32 // Size of the glowing sphere (Area light radius)
	SourceLength float32 // Length of the glowing tube (Area light length)
}

// SpotLight holds spot light parameters.
type SpotLight struct {
	Range          float32 // Max distance (0 = infinite)
	InnerConeAngle float32 // Radians
	OuterConeAngle float32 // Radians
	SourceRadius   float32 // Size of the emissive surface
}
