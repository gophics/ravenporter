package audio

import "github.com/gophics/ravenporter/internal/process/core"

// Steps returns the built-in audio post-processing steps.
func Steps() []core.Step {
	return []core.Step{
		&decodeSamplesStep{},
		&normalizeAudioStep{},
		&trimAudioStep{},
		&mixdownAudioStep{},
		&resampleAudioStep{},
	}
}
