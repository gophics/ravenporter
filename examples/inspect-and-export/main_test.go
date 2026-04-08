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

	const want = "format=obj report_issues=0 validation_errors=0 validation_warnings=0 json_meshes=1 json_root_nodes=1\n"
	require.Equal(t, want, out.String())
}
