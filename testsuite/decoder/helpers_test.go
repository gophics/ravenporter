package decoder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func sourcePath(parts ...string) string {
	elems := append([]string{"..", "testdata", "source", "assimp"}, parts...)
	return filepath.Join(elems...)
}

func sourceData(t testing.TB, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(sourcePath(parts...))
	require.NoError(t, err)
	return data
}
