package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, run(&out))

	const want = "" +
		"import-dir=2\n" +
		"  a.obj -> obj\n" +
		"  nested/b.obj -> obj\n" +
		"import-fs-dir=2\n" +
		"  assets/a.obj -> obj\n" +
		"  assets/nested/b.obj -> obj\n"

	require.Equal(t, want, out.String())
}
