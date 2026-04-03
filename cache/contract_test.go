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

const fullSceneContractHash = "842884c36ed99edc900506a16604f0a6d16a4626e6b1cfdb6c538bb727e286ba"

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
