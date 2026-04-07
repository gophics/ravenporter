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

const fullSceneContractHash = "5ef55275d574b15f51fe3da7e975b3e46d65371b4d7d03b9893854cd304b29ca"

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
