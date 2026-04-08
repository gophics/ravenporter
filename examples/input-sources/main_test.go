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

	const want = "dirfs=obj/1\nbytes=obj/1\nreader=obj/1\n"
	require.Equal(t, want, out.String())
}
