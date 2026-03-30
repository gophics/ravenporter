package pipeline

import (
	"slices"

	"github.com/gophics/ravenporter/process"
)

const presetCount = 3

// ProcessStepInfo describes a canonical process-step override name.
type ProcessStepInfo struct {
	Name      string   `json:"name"`
	EnabledBy []string `json:"enabled_by,omitempty"`
}

// BuiltInProcessSteps returns the canonical CLI/profile step names.
func BuiltInProcessSteps() []ProcessStepInfo {
	steps := make([]ProcessStepInfo, 0, len(processStepDefs))
	for _, def := range processStepDefs {
		steps = append(steps, ProcessStepInfo{
			Name:      def.name,
			EnabledBy: enabledByPresets(def.flag),
		})
	}
	slices.SortFunc(steps, func(a, b ProcessStepInfo) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return steps
}

func enabledByPresets(flag process.PPFlag) []string {
	presets := make([]string, 0, presetCount)
	if process.PresetFast&flag != 0 {
		presets = append(presets, BuiltInPresetFast)
	}
	if process.PresetQuality&flag != 0 {
		presets = append(presets, BuiltInPresetQuality)
	}
	if process.PresetMaxQuality&flag != 0 {
		presets = append(presets, BuiltInPresetMaxQuality)
	}
	return presets
}
