package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectorFontRawBytesWithLoader(t *testing.T) {
	called := 0
	font := &VectorFontData{}
	font.SetRawBytesLoader(func() ([]byte, error) {
		called++
		return []byte{7, 8, 9}, nil
	})

	data, err := font.RawBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte{7, 8, 9}, data)
	assert.Equal(t, 1, called)

	data, err = font.RawBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte{7, 8, 9}, data)
	assert.Equal(t, 1, called)
	assert.True(t, font.HasRawBytes())
}
