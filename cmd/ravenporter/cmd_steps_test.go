package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestStepsCommandJSON(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{stepsCmd()},
	}

	stdout, err := captureStdout(t, func() error {
		return app.Run([]string{"app", "steps", "--json"})
	})
	require.NoError(t, err)

	var steps []struct {
		Name      string   `json:"name"`
		EnabledBy []string `json:"enabled_by"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &steps))
	require.NotEmpty(t, steps)
	require.Contains(t, steps[0].Name, "-")

	var found bool
	for _, step := range steps {
		if step.Name == "decode-pixels" {
			found = true
			require.Contains(t, step.EnabledBy, "quality")
			require.Contains(t, step.EnabledBy, "max-quality")
			break
		}
	}
	require.True(t, found)
}

func TestStepsCommandText(t *testing.T) {
	app := &cli.App{
		Commands: []*cli.Command{stepsCmd()},
	}

	stdout, err := captureStdout(t, func() error {
		return app.Run([]string{"app", "steps"})
	})
	require.NoError(t, err)
	require.Contains(t, stdout, "Canonical process steps:")
	require.Contains(t, stdout, "embed-textures")
	require.True(t, strings.Contains(stdout, "quality") || strings.Contains(stdout, "max-quality"))
}
