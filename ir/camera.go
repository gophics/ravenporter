package ir

// Camera uses composition: exactly one of Perspective or Orthographic is non-nil.
type Camera struct {
	Name         string
	Perspective  *PerspectiveCamera
	Orthographic *OrthographicCamera
}

// PerspectiveCamera holds perspective projection parameters.
type PerspectiveCamera struct {
	FOV           float32 // Vertical field of view in radians
	Aspect        float32 // Width / Height (0 = use viewport default)
	Near          float32
	Far           float32 // 0 = infinite
	FocalLength   float32 // Physical lens focal length in mm
	FocusDistance float32 // Physical lens manual focus distance
	FStop         float32 // Physical lens aperture (e.g., 1.4, 2.0, 2.8)
	SensorWidth   float32 // Physical sensor width in mm
	SensorHeight  float32 // Physical sensor height in mm
}

// OrthographicCamera holds orthographic projection parameters.
type OrthographicCamera struct {
	XMag float32
	YMag float32
	Near float32
	Far  float32
}
