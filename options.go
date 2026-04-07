package ravenporter

import (
	"log/slog"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

// Option configures an import operation.
type Option = pipeline.Option

// LoadMask controls which content domains are kept after import.
type LoadMask = pipeline.LoadMask

// LoadMask flags keep the selected content domains in the returned scene.
const (
	LoadMeshes     = pipeline.LoadMeshes
	LoadMaterials  = pipeline.LoadMaterials
	LoadTextures   = pipeline.LoadTextures
	LoadAnimations = pipeline.LoadAnimations
	LoadSkeletons  = pipeline.LoadSkeletons
	LoadCameras    = pipeline.LoadCameras
	LoadLights     = pipeline.LoadLights
	LoadAudio      = pipeline.LoadAudio
	LoadFonts      = pipeline.LoadFonts
	LoadImages     = pipeline.LoadImages
	LoadAll        = pipeline.LoadAll
)

// WithRegistry sets the decoder registry used during import.
func WithRegistry(registry *detect.Registry) Option {
	return pipeline.WithRegistry(registry)
}

// WithLogger sets the logger used during import and processing.
func WithLogger(logger *slog.Logger) Option {
	return pipeline.WithLogger(logger)
}

// WithPreset applies a built-in generic preset.
func WithPreset(name string) Option {
	return pipeline.WithPreset(name)
}

// WithProfile applies a serialized import profile.
func WithProfile(profile Profile) Option {
	return pipeline.WithProfile(profile)
}

// WithProfileFile loads and applies a serialized import profile from disk.
func WithProfileFile(path string) Option {
	return pipeline.WithProfileFile(path)
}

// WithDecodeMaxFileSize sets the maximum input size accepted during decode.
func WithDecodeMaxFileSize(size int64) Option {
	return pipeline.WithDecodeMaxFileSize(size)
}

// WithDecodeMaxVertices sets the maximum vertices accepted per mesh during decode.
func WithDecodeMaxVertices(limit int) Option {
	return pipeline.WithDecodeMaxVertices(limit)
}

// WithDecodeMaxImagePixels sets the maximum image pixels accepted during decode.
func WithDecodeMaxImagePixels(limit int) Option {
	return pipeline.WithDecodeMaxImagePixels(limit)
}

// WithDecodeMaxAudioSamples sets the maximum audio samples accepted during decode.
func WithDecodeMaxAudioSamples(limit int) Option {
	return pipeline.WithDecodeMaxAudioSamples(limit)
}

// WithGlobalScale enables global scaling during processing.
func WithGlobalScale(scale float64) Option {
	return pipeline.WithGlobalScale(scale)
}

// WithTargetUpAxis enables up-axis conversion during processing.
func WithTargetUpAxis(axis ir.Axis) Option {
	return pipeline.WithTargetUpAxis(axis)
}

// WithEmbedTextures enables texture embedding during processing.
func WithEmbedTextures() Option {
	return pipeline.WithEmbedTextures()
}

// WithLoadMask keeps only the selected content domains in the returned scene.
func WithLoadMask(mask LoadMask) Option {
	return pipeline.WithLoadMask(mask)
}

// WithProcessFlags replaces the process flag mask directly.
func WithProcessFlags(flags process.PPFlag) Option {
	return pipeline.WithProcessFlags(flags)
}
