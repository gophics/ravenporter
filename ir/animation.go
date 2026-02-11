package ir

// Animation holds a named animation with channels targeting scene nodes.
type Animation struct {
	Name     string
	Channels []AnimationChannel
	Duration float64 // Total duration in seconds
}

// AnimationChannel targets a single property on a single node.
type AnimationChannel struct {
	NodeIndex     int
	Target        ChannelTarget
	Interpolation Interpolation
	Times         []float32 // Keyframe timestamps (seconds)
	Pointer       string    // JSON Pointer path (KHR_animation_pointer)
	MaterialIndex int       // Target material index (TargetMaterialColor/Scalar)
	Values        []float32 // Generic float values (scalar/vec2/vec3/vec4)

	// Exactly one of the following is non-nil, matching Target.
	Translations [][3]float32 // TargetTranslation
	Rotations    [][4]float32 // TargetRotation
	Scales       [][3]float32 // TargetScale
	Weights      []float32    // TargetMorphWeights (stride = morph target count)
}
