package ir

import (
	"errors"
	"sync"
)

var (
	errGPUCompressed           = errors.New("GPU-compressed textures cannot be decoded to pixels")
	errPixelDecodeNotSupported = errors.New("pixel decode not implemented for this format")
)

// PixelDecodeFunc decodes raw compressed bytes into pixel data.
// Each image decoder sets this on the ImageAsset at construction time.
type PixelDecodeFunc func(d *ImageAsset) (*PixelBuffer, error)

// DataType defines how the raw bytes in a PixelBuffer should be interpreted.
type DataType uint8

// Common data types for pixel formats.
const (
	DataTypeUint8 DataType = iota
	DataTypeFloat32
)

// GPUCompression identifies a GPU-native block compression scheme.
type GPUCompression uint8

// Supported GPU native block compression schemes.
const (
	GPUCompressionNone GPUCompression = iota
	GPUCompressionBC1
	GPUCompressionBC2
	GPUCompressionBC3
	GPUCompressionBC4
	GPUCompressionBC5
	GPUCompressionBC6H
	GPUCompressionBC7
	GPUCompressionASTC4x4
	GPUCompressionETC2
)

var gpuCompressionNames = [...]string{
	GPUCompressionNone:    "",
	GPUCompressionBC1:     "BC1",
	GPUCompressionBC2:     "BC2",
	GPUCompressionBC3:     "BC3",
	GPUCompressionBC4:     "BC4",
	GPUCompressionBC5:     "BC5",
	GPUCompressionBC6H:    "BC6H",
	GPUCompressionBC7:     "BC7",
	GPUCompressionASTC4x4: "ASTC_4x4",
	GPUCompressionETC2:    "ETC2",
}

func (g GPUCompression) String() string {
	if int(g) < len(gpuCompressionNames) {
		return gpuCompressionNames[g]
	}
	return ""
}

// PixelBuffer holds unified image pixel data.
type PixelBuffer struct {
	Data     []byte
	DataType DataType
	Mipmaps  [][]byte
	BitDepth BitDepth
}

// ImageAsset holds image data.
type ImageAsset struct {
	Name       string
	Format     ImageFormat
	Width      int
	Height     int
	Channels   ChannelCount
	ColorSpace ColorSpace
	MipLevels  int

	pixels   *PixelBuffer
	decOnce  sync.Once
	decError error

	compressedOnce   sync.Once
	compressedError  error
	compressedLoader func() ([]byte, error)

	Compressed        []byte // Raw file bytes for engine passthrough.
	SourceFormat      FormatID
	SourcePath        string
	CompressionFormat GPUCompression // GPU compression scheme (BC1, BC7, etc.) for DDS/KTX.

	Metadata map[string]string

	PixelDecode PixelDecodeFunc `json:"-"` // Set by the decoder, called by DecodePixels.
}

// Pixels returns the decoded pixel buffer, or nil if not yet decoded.
func (d *ImageAsset) Pixels() *PixelBuffer {
	return d.pixels
}

// SetPixels sets the decoded pixel buffer directly.
func (d *ImageAsset) SetPixels(pb *PixelBuffer) {
	d.pixels = pb
	d.decOnce = sync.Once{} // reset for future use
	d.decError = nil
}

// HasCompressedBytes reports whether the image has eager or lazy raw bytes.
func (d *ImageAsset) HasCompressedBytes() bool {
	return d != nil && (len(d.Compressed) != 0 || d.compressedLoader != nil)
}

// CompressedBytes returns the raw encoded bytes, materializing them on demand.
func (d *ImageAsset) CompressedBytes() ([]byte, error) {
	if d == nil {
		return nil, nil
	}
	if len(d.Compressed) != 0 {
		return d.Compressed, nil
	}
	if d.compressedLoader == nil {
		return nil, nil
	}
	d.compressedOnce.Do(func() {
		d.Compressed, d.compressedError = d.compressedLoader()
	})
	return d.Compressed, d.compressedError
}

// SetCompressedBytes replaces the raw encoded bytes and clears any lazy loader.
func (d *ImageAsset) SetCompressedBytes(data []byte) {
	if d == nil {
		return
	}
	d.Compressed = data
	d.compressedLoader = nil
	d.compressedOnce = sync.Once{}
	d.compressedError = nil
}

// SetCompressedLoader installs a lazy raw-byte loader.
func (d *ImageAsset) SetCompressedLoader(loader func() ([]byte, error)) {
	if d == nil {
		return
	}
	d.Compressed = nil
	d.compressedLoader = loader
	d.compressedOnce = sync.Once{}
	d.compressedError = nil
}

// IsGPUCompressed returns true for formats that should be passed through without decoding.
func (d *ImageAsset) IsGPUCompressed() bool {
	return d.CompressionFormat != GPUCompressionNone
}

// DecodePixels decodes Compressed bytes into pixel data.
// Thread-safe: uses sync.Once to ensure decode runs at most once.
// GPU-compressed formats return an error - use IsGPUCompressed() first.
func (d *ImageAsset) DecodePixels() (*PixelBuffer, error) {
	if d.pixels != nil {
		return d.pixels, nil
	}
	if d.IsGPUCompressed() {
		return nil, errGPUCompressed
	}
	compressed, err := d.CompressedBytes()
	if err != nil {
		return nil, err
	}
	if len(compressed) == 0 && d.PixelDecode == nil {
		return nil, nil
	}
	if d.PixelDecode == nil {
		return nil, errPixelDecodeNotSupported
	}
	d.decOnce.Do(func() {
		d.pixels, d.decError = d.PixelDecode(d)
	})
	return d.pixels, d.decError
}
