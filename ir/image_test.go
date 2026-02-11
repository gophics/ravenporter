package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodePixelsWithFunc(t *testing.T) {
	decoded := &ImageAsset{
		Width:      2,
		Height:     2,
		Compressed: []byte{1, 2, 3},
		PixelDecode: func(d *ImageAsset) (*PixelBuffer, error) {
			return &PixelBuffer{Data: make([]byte, d.Width*d.Height*4), DataType: DataTypeUint8, BitDepth: BitDepth8}, nil
		},
	}

	pb, err := decoded.DecodePixels()
	require.NoError(t, err)
	require.NotNil(t, pb)
	assert.Equal(t, 2*2*4, len(pb.Data))
	assert.Equal(t, DataTypeUint8, pb.DataType)
	assert.Equal(t, BitDepth8, pb.BitDepth)
	assert.Equal(t, pb, decoded.Pixels())
}

func TestDecodePixelsGPUCompressed(t *testing.T) {
	decoded := &ImageAsset{
		CompressionFormat: GPUCompressionBC1,
		Compressed:        []byte{0, 0, 0},
	}
	pb, err := decoded.DecodePixels()
	assert.Nil(t, pb)
	assert.Error(t, err)
}

func TestDecodePixelsNoFunc(t *testing.T) {
	decoded := &ImageAsset{
		Compressed: []byte{1, 2, 3},
	}
	pb, err := decoded.DecodePixels()
	assert.Nil(t, pb)
	assert.Error(t, err)
}

func TestDecodePixelsAlreadyDecoded(t *testing.T) {
	called := false
	decoded := &ImageAsset{
		Compressed: []byte{1, 2, 3},
		PixelDecode: func(_ *ImageAsset) (*PixelBuffer, error) {
			called = true
			return &PixelBuffer{Data: []byte{1, 2, 3}}, nil
		},
	}

	pb1, err := decoded.DecodePixels()
	require.NoError(t, err)
	require.True(t, called)

	called = false
	pb2, err := decoded.DecodePixels()
	require.NoError(t, err)
	assert.False(t, called, "PixelDecode should not be called again")
	assert.Equal(t, pb1, pb2, "should return cached result")
}

func TestDecodePixelsEmptyCompressed(t *testing.T) {
	decoded := &ImageAsset{}
	pb, err := decoded.DecodePixels()
	assert.NoError(t, err)
	assert.Nil(t, pb)
}

func TestCompressedBytesWithLoader(t *testing.T) {
	called := 0
	image := &ImageAsset{}
	image.SetCompressedLoader(func() ([]byte, error) {
		called++
		return []byte{1, 2, 3}, nil
	})

	data, err := image.CompressedBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, data)
	assert.Equal(t, 1, called)

	data, err = image.CompressedBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, data)
	assert.Equal(t, 1, called)
	assert.True(t, image.HasCompressedBytes())
}

func TestIsGPUCompressed(t *testing.T) {
	assert.False(t, (&ImageAsset{CompressionFormat: GPUCompressionNone}).IsGPUCompressed())
	assert.True(t, (&ImageAsset{CompressionFormat: GPUCompressionBC1}).IsGPUCompressed())
	assert.True(t, (&ImageAsset{CompressionFormat: GPUCompressionBC7}).IsGPUCompressed())
	assert.True(t, (&ImageAsset{CompressionFormat: GPUCompressionASTC4x4}).IsGPUCompressed())
}

func TestGPUCompressionString(t *testing.T) {
	assert.Equal(t, "", GPUCompressionNone.String())
	assert.Equal(t, "BC1", GPUCompressionBC1.String())
	assert.Equal(t, "BC7", GPUCompressionBC7.String())
	assert.Equal(t, "ASTC_4x4", GPUCompressionASTC4x4.String())
	assert.Equal(t, "ETC2", GPUCompressionETC2.String())
}
