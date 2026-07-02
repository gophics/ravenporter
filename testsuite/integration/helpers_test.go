//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/ir"
)

// corpusDir returns the path to the source corpus directory for a given fixture.
func corpusDir(t testing.TB, subpath string) string {
	t.Helper()
	root := filepath.Join("..")
	if strings.HasPrefix(subpath, "isolation/") ||
		strings.HasPrefix(subpath, "rejection/") ||
		strings.HasPrefix(subpath, "third_party/") {
		return filepath.Join(root, "corpus")
	}
	dir := filepath.Join(root, "testdata", "source", "assimp")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("test corpus not found at %s", dir)
	}
	return dir
}

// runPipeline executes the full public E2E pipeline for a single file.
func runPipeline(t *testing.T, subpath string, opts ...pipeline.Option) *ir.Asset {
	t.Helper()
	path := filepath.Join(corpusDir(t, subpath), filepath.FromSlash(subpath))
	result, err := pipeline.ImportPath(context.Background(), path, opts...)
	require.NoErrorf(t, err, "pipeline failed to process %s", subpath)
	return result.Asset
}

// goldenCompare checks an IR asset against its known-good JSON structural output.
func goldenCompare(t *testing.T, asset *ir.Asset, goldenPath string) {
	t.Helper()

	actualJSON, err := json.MarshalIndent(asset, "", "  ")
	require.NoError(t, err, "failed to marshal asset JSON")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0755))
		require.NoError(t, os.WriteFile(goldenPath, actualJSON, 0644))
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expectedJSON, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		t.Skipf("golden file missing, run with UPDATE_GOLDEN=1 to create: %s", goldenPath)
	}
	require.NoError(t, err)
	require.JSONEq(t, string(expectedJSON), string(actualJSON), "semantic structure drifted significantly")
}
