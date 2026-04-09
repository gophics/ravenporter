package ir

import (
	"errors"
	"sync"
	"time"
)

var errSampleDecodeNotSupported = errors.New("sample decode not implemented for this format")

// SampleDecodeFunc decodes raw compressed bytes into PCM float32 samples.
type SampleDecodeFunc func(clip *AudioClip) ([]float32, error)

// AudioClip holds audio data.
type AudioClip struct {
	Name        string
	Format      AudioFormat
	SampleRate  int
	Layout      ChannelLayout
	ChannelMask uint32
	BitDepth    BitDepth
	Duration    time.Duration
	LoopStart   int // NoIndex if no loop
	LoopEnd     int // NoIndex if no loop
	Metadata    AudioMetadata

	samples  []float32
	decOnce  sync.Once
	decError error

	compressedOnce   sync.Once
	compressedError  error
	compressedLoader func() ([]byte, error)

	Compressed   []byte           `json:",omitempty"` // Raw file bytes for engine passthrough.
	SourceCodec  AudioFormat      `json:",omitempty"` // Codec of the Compressed data.
	SampleDecode SampleDecodeFunc `json:"-"`
}

// HasCompressedBytes reports whether the clip has eager or lazy raw bytes.
func (c *AudioClip) HasCompressedBytes() bool {
	return c != nil && (len(c.Compressed) != 0 || c.compressedLoader != nil)
}

// CompressedBytes returns the raw encoded bytes, materializing them on demand.
func (c *AudioClip) CompressedBytes() ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	if len(c.Compressed) != 0 {
		return c.Compressed, nil
	}
	if c.compressedLoader == nil {
		return nil, nil
	}
	c.compressedOnce.Do(func() {
		c.Compressed, c.compressedError = c.compressedLoader()
	})
	return c.Compressed, c.compressedError
}

// SetCompressedBytes replaces the raw encoded bytes and clears any lazy loader.
func (c *AudioClip) SetCompressedBytes(data []byte) {
	if c == nil {
		return
	}
	c.Compressed = data
	c.compressedLoader = nil
	c.compressedOnce = sync.Once{}
	c.compressedError = nil
}

// SetCompressedLoader installs a lazy raw-byte loader.
func (c *AudioClip) SetCompressedLoader(loader func() ([]byte, error)) {
	if c == nil {
		return
	}
	c.Compressed = nil
	c.compressedLoader = loader
	c.compressedOnce = sync.Once{}
	c.compressedError = nil
}

// DecodeSamples lazily decodes Compressed bytes into PCM float32 samples.
// Thread-safe via sync.Once.
func (c *AudioClip) DecodeSamples() ([]float32, error) {
	if c.samples != nil {
		return c.samples, nil
	}
	compressed, err := c.CompressedBytes()
	if err != nil {
		return nil, err
	}
	if len(compressed) == 0 && c.SampleDecode == nil {
		return nil, nil
	}
	if c.SampleDecode == nil {
		return nil, errSampleDecodeNotSupported
	}
	c.decOnce.Do(func() {
		c.samples, c.decError = c.SampleDecode(c)
	})
	return c.samples, c.decError
}

// AudioMetadata holds optional audio metadata.
type AudioMetadata struct {
	Title     string
	Artist    string
	Album     string
	Genre     string
	Comment   string
	Artwork   []byte
	CuePoints []CuePoint
}

// CuePoint marks a named position in audio data.
type CuePoint struct {
	Name   string
	Sample int
}
