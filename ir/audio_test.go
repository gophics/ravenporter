package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressedAudioBytesWithLoader(t *testing.T) {
	called := 0
	clip := &AudioClip{}
	clip.SetCompressedLoader(func() ([]byte, error) {
		called++
		return []byte{4, 5, 6}, nil
	})

	data, err := clip.CompressedBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte{4, 5, 6}, data)
	assert.Equal(t, 1, called)

	data, err = clip.CompressedBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte{4, 5, 6}, data)
	assert.Equal(t, 1, called)
	assert.True(t, clip.HasCompressedBytes())
}
