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

	const want = "format=obj preset=fast meshes=1 issues=0\n"
	require.Equal(t, want, out.String())
}
