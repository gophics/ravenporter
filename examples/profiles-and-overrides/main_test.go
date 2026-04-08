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
		"saved preset=quality scale=0.5 embed=true\n" +
		"effective preset=quality scale=2 embed=true\n" +
		"import format=obj meshes=1\n"

	require.Equal(t, want, out.String())
}
