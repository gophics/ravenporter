package main

import (
	"fmt"
	"strings"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/urfave/cli/v2"
)

func stepsCmd() *cli.Command {
	return &cli.Command{
		Name:  "steps",
		Usage: "List canonical process-step names for CLI overrides and profiles",
		Description: "Use these names with --enable-step and --disable-step " +
			"on import, batch, cook, export, convert, and profile export.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flagJSON, Usage: "Output as JSON"},
		},
		Action: runSteps,
	}
}

func runSteps(c *cli.Context) error {
	steps := pipeline.BuiltInProcessSteps()
	if c.Bool(flagJSON) {
		return writeJSON(steps)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "Canonical process steps:\tEnabled by preset") //nolint:errcheck // stdout
	for _, step := range steps {
		presets := "-"
		if len(step.EnabledBy) > 0 {
			presets = strings.Join(step.EnabledBy, ", ")
		}
		fmt.Fprintf(w, "  %s\t%s\n", step.Name, presets) //nolint:errcheck // stdout
	}
	return w.Flush()
}
