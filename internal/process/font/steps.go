package font

import "github.com/gophics/ravenporter/internal/process/core"

// Steps returns the built-in font post-processing steps.
func Steps() []core.Step {
	return []core.Step{
		&generateFontAtlasStep{},
	}
}
