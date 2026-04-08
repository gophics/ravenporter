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

	const want = "source=obj cooked_meshes=1 summary_meshes=1\n"
	require.Equal(t, want, out.String())
}
