package cache

import (
	"fmt"
	"strings"
)

// ImagePixelsMode controls whether decoded image pixels are serialized into the cache.
type ImagePixelsMode uint8

const (
	// ImagePixelsNever omits decoded pixel buffers unless they are the only preserved image representation.
	ImagePixelsNever ImagePixelsMode = iota
	// ImagePixelsIfPresent preserves decoded pixel buffers that are already materialized before cooking.
	ImagePixelsIfPresent
	// ImagePixelsAlways decodes and stores pixel buffers whenever the source image can be decoded.
	ImagePixelsAlways
)

func (m ImagePixelsMode) String() string {
	switch m {
	case ImagePixelsNever:
		return "never"
	case ImagePixelsIfPresent:
		return "if-present"
	case ImagePixelsAlways:
		return "always"
	default:
		return ""
	}
}

// ParseImagePixelsMode parses the supported image-pixel cache modes.
func ParseImagePixelsMode(value string) (ImagePixelsMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "never":
		return ImagePixelsNever, nil
	case "if-present":
		return ImagePixelsIfPresent, nil
	case "always":
		return ImagePixelsAlways, nil
	default:
		return 0, fmt.Errorf("cache: unsupported image pixel mode %q", value)
	}
}

// WithImagePixels controls decoded image pixel persistence during cache writes.
func WithImagePixels(mode ImagePixelsMode) Option {
	return func(cfg *writeConfig) error {
		cfg.imagePixels = mode
		return nil
	}
}

// WithMaxEmbeddedMediaBytes rejects cache writes that exceed the given total blob budget.
func WithMaxEmbeddedMediaBytes(limit int64) Option {
	return func(cfg *writeConfig) error {
		if limit < 0 {
			return fmt.Errorf("cache: max embedded media bytes must be >= 0")
		}
		cfg.maxEmbeddedMediaBytes = limit
		return nil
	}
}

// WithEagerMedia forces cache reads to materialize embedded media during open/read.
func WithEagerMedia() ReadOption {
	return func(cfg *readConfig) error {
		cfg.eagerMedia = true
		return nil
	}
}
