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
		"profile preset=quality scale=1.5 assets=2\n" +
		"  characters/hero.obj format=obj meshes=1 issues=0 validation_errors=0 validation_warnings=0 json_meshes=1 cache_meshes=1\n" +
		"  props/crate.obj format=obj meshes=1 issues=0 validation_errors=0 validation_warnings=0 json_meshes=1 cache_meshes=1\n"

	require.Equal(t, want, out.String())
}
