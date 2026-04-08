package pipeline

import (
	"fmt"
	"log/slog"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

// Option configures a pipeline import.
type Option func(*config) error

func resolveOptions(options ...Option) (config, error) {
	cfg := config{profileResolvable: true}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&cfg); err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}

// ResolveProfile returns the effective serializable profile represented by the given options.
func ResolveProfile(options ...Option) (Profile, error) {
	cfg, err := resolveOptions(options...)
	if err != nil {
		return Profile{}, err
	}
	if cfg.profileResolvable && cfg.profile.HasContent() {
		if cfg.profile.Version == 0 {
			cfg.profile.Version = ProfileVersion
		}
		return cfg.profile, nil
	}
	return profileFromConfig(cfg), nil
}

// WithRegistry sets the decoder registry used during import.
func WithRegistry(registry *detect.Registry) Option {
	return func(cfg *config) error {
		cfg.Registry = registry
		return nil
	}
}

// WithLogger sets the logger used during import and processing.
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) error {
		cfg.Logger = logger
		cfg.ProcessOpts.Logger = logger
		return nil
	}
}

// WithPreset applies a built-in generic preset.
func WithPreset(name string) Option {
	return func(cfg *config) error {
		flags, err := ResolveBuiltInPreset(name)
		if err != nil {
			return err
		}
		cfg.Preset = canonicalPresetName(name)
		cfg.ProcessFlags = flags
		cfg.recordProfile(func(profile *Profile) {
			profile.Preset = cfg.Preset
		})
		return nil
	}
}

// WithProfile applies a serialized import profile.
func WithProfile(profile Profile) Option {
	return func(cfg *config) error {
		cfg.recordProfile(func(tracked *Profile) {
			*tracked = MergeProfiles(*tracked, profile)
		})
		return applyProfile(cfg, profile)
	}
}

// WithProfileFile loads and applies a serialized import profile from disk.
func WithProfileFile(path string) Option {
	return func(cfg *config) error {
		profile, err := LoadProfile(path)
		if err != nil {
			return err
		}
		cfg.ProfileFile = path
		return WithProfile(profile)(cfg)
	}
}

// WithDecodeMaxFileSize sets the maximum input size accepted during decode.
func WithDecodeMaxFileSize(size int64) Option {
	return func(cfg *config) error {
		cfg.DecodeOpts.MaxFileSize = size
		cfg.recordProfile(func(profile *Profile) {
			profile.Decode.MaxFileSize = clonePtr(&size)
		})
		return nil
	}
}

// WithDecodeMaxVertices sets the maximum vertices accepted per mesh during decode.
func WithDecodeMaxVertices(limit int) Option {
	return func(cfg *config) error {
		cfg.DecodeOpts.MaxVertices = limit
		cfg.recordProfile(func(profile *Profile) {
			profile.Decode.MaxVertices = clonePtr(&limit)
		})
		return nil
	}
}

// WithDecodeMaxImagePixels sets the maximum image pixels accepted during decode.
func WithDecodeMaxImagePixels(limit int) Option {
	return func(cfg *config) error {
		cfg.DecodeOpts.MaxImagePixels = limit
		cfg.recordProfile(func(profile *Profile) {
			profile.Decode.MaxImagePixels = clonePtr(&limit)
		})
		return nil
	}
}

// WithDecodeMaxAudioSamples sets the maximum decoded audio samples accepted during decode.
func WithDecodeMaxAudioSamples(limit int) Option {
	return func(cfg *config) error {
		cfg.DecodeOpts.MaxAudioSamples = limit
		cfg.recordProfile(func(profile *Profile) {
			profile.Decode.MaxAudioSamples = clonePtr(&limit)
		})
		return nil
	}
}

// WithGlobalScale enables global scaling during processing.
func WithGlobalScale(scale float64) Option {
	return func(cfg *config) error {
		cfg.ProcessFlags |= process.PPGlobalScale
		cfg.ProcessOpts.GlobalScale = scale
		cfg.recordProfile(func(profile *Profile) {
			profile.Process.GlobalScale = clonePtr(&scale)
		})
		return nil
	}
}

// WithTargetUpAxis enables up-axis conversion during processing.
func WithTargetUpAxis(axis ir.Axis) Option {
	return func(cfg *config) error {
		cfg.ProcessFlags |= process.PPFixUpAxis
		cfg.ProcessOpts.TargetUpAxis = axis
		axisName := axisName(axis)
		cfg.recordProfile(func(profile *Profile) {
			profile.Process.TargetUpAxis = &axisName
		})
		return nil
	}
}

// WithEmbedTextures enables texture embedding during processing.
func WithEmbedTextures() Option {
	return func(cfg *config) error {
		cfg.ProcessFlags |= process.PPEmbedTextures
		cfg.recordProfile(func(profile *Profile) {
			profile.Process.EnabledSteps = mergeStringSlices(profile.Process.EnabledSteps, []string{"embed-textures"})
		})
		return nil
	}
}

// WithLoadMask keeps only the selected content domains in the returned scene.
func WithLoadMask(mask LoadMask) Option {
	return func(cfg *config) error {
		cfg.loadMask = mask
		cfg.loadMaskSet = true
		cfg.profileResolvable = false
		return nil
	}
}

// WithProcessFlags replaces the process flag mask directly.
func WithProcessFlags(flags process.PPFlag) Option {
	return func(cfg *config) error {
		cfg.ProcessFlags = flags
		cfg.profileResolvable = false
		return nil
	}
}

// WithBatchConcurrency sets the maximum number of assets imported concurrently by ImportDir and ImportFSDir.
// A limit of 0 uses the default worker count derived from runtime.GOMAXPROCS(0).
func WithBatchConcurrency(limit int) Option {
	return func(cfg *config) error {
		if limit < 0 {
			return fmt.Errorf("pipeline: batch concurrency must be >= 0")
		}
		cfg.workerLimit = limit
		return nil
	}
}

func (cfg *config) recordProfile(update func(*Profile)) {
	if !cfg.profileResolvable {
		return
	}
	if cfg.profile.Version == 0 {
		cfg.profile.Version = ProfileVersion
	}
	update(&cfg.profile)
}
