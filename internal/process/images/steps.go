package images

import "github.com/gophics/ravenporter/internal/process/core"

// Steps returns the built-in image post-processing steps.
func Steps() []core.Step {
	return []core.Step{
		&decodePixelsStep{},
		&generateMipmapsStep{},
		&resizeImagesStep{},
	}
}
