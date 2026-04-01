package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestImportCommandWritesFailureReport(t *testing.T) {
	inputDir := t.TempDir()
	inputPath := filepath.Join(inputDir, "broken.gltf")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	require.NoError(t, os.WriteFile(inputPath, []byte("{"), 0o644))

	app := &cli.App{
		Commands: []*cli.Command{importCmd()},
	}

	err := app.Run([]string{"app", "import", "--out", t.TempDir(), "--report-file", reportPath, inputPath})
	require.Error(t, err)

	report, readErr := os.ReadFile(reportPath)
	require.NoError(t, readErr)
	require.Contains(t, string(report), "\"issues\"")
	require.True(t, strings.Contains(string(report), "DECODE_FAILED") || strings.Contains(string(report), "DETECT_FAILED"))
}
