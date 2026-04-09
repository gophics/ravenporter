package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter"
)

const fullSceneContractHash = "fd1eac342bbc73dd390c62f258547870c3e9475f16633c2145f2e6ddd0b04401"

func TestWriteContractStable(t *testing.T) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var first bytes.Buffer
	var second bytes.Buffer
	require.NoError(t, Write(&first, result))
	require.NoError(t, Write(&second, result))
	assert.Equal(t, first.Bytes(), second.Bytes())

	sum := sha256.Sum256(first.Bytes())
	got := hex.EncodeToString(sum[:])
	if got != fullSceneContractHash {
		t.Fatalf("cache contract hash changed: got %s", got)
	}
}
