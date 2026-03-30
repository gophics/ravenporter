package ravenporter

import (
	"bytes"
	"context"
	"io/fs"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/internal/pipeline"
	"github.com/gophics/ravenporter/ir"
)

// Result is the root-level import result.
type Result = pipeline.Result

// Report is the structured import report.
type Report = pipeline.Report

// SourceReport describes input provenance and effective import behavior.
type SourceReport = pipeline.SourceReport

// Issue is a stable import finding entry.
type Issue = pipeline.Issue

// Dependency is a direct external dependency discovered during import.
type Dependency = pipeline.Dependency

// AssetSummary records high-level counts for the imported asset.
type AssetSummary = pipeline.AssetSummary

// Profile is the serializable import profile schema.
type Profile = pipeline.Profile

// DecodeProfile stores serializable decode safeguards.
type DecodeProfile = pipeline.DecodeProfile

// ProcessProfile stores serializable process overrides.
type ProcessProfile = pipeline.ProcessProfile

// ReadSeekerAt is the in-memory/custom stream shape accepted by ImportReader.
type ReadSeekerAt = detect.ReadSeekerAt

const (
	// ProfileVersion is the current TOML profile schema version.
	ProfileVersion = pipeline.ProfileVersion

	// BuiltInPresetFast is the built-in preset optimized for speed.
	BuiltInPresetFast = pipeline.BuiltInPresetFast

	// BuiltInPresetQuality is the built-in preset tuned for balanced quality.
	BuiltInPresetQuality = pipeline.BuiltInPresetQuality

	// BuiltInPresetMaxQuality is the built-in preset tuned for the highest built-in quality.
	BuiltInPresetMaxQuality = pipeline.BuiltInPresetMaxQuality
)

// ImportPath imports an asset from the local filesystem.
func ImportPath(ctx context.Context, path string, options ...Option) (*Result, error) {
	return pipeline.ImportPath(ctx, path, options...)
}

// ImportFS imports an asset from an arbitrary fs.FS.
func ImportFS(ctx context.Context, fsys fs.FS, path string, options ...Option) (*Result, error) {
	return pipeline.ImportFS(ctx, fsys, path, options...)
}

// ImportDir imports every supported asset found under a local directory tree.
func ImportDir(ctx context.Context, dir string, options ...Option) ([]*Result, error) {
	return pipeline.ImportDir(ctx, dir, options...)
}

// ImportFSDir imports every supported asset found under a directory within an arbitrary fs.FS.
func ImportFSDir(ctx context.Context, fsys fs.FS, dir string, options ...Option) ([]*Result, error) {
	return pipeline.ImportFSDir(ctx, fsys, dir, options...)
}

// ImportReader imports an asset from a memory-backed or custom reader.
func ImportReader(ctx context.Context, reader ReadSeekerAt, filename string, options ...Option) (*Result, error) {
	return pipeline.ImportReader(ctx, reader, filename, options...)
}

// ImportBytes imports an asset from an in-memory byte slice.
func ImportBytes(ctx context.Context, data []byte, filename string, options ...Option) (*Result, error) {
	return pipeline.ImportReader(ctx, bytes.NewReader(data), filename, options...)
}

// LoadProfile reads a TOML import profile from disk.
func LoadProfile(path string) (Profile, error) {
	return pipeline.LoadProfile(path)
}

// SaveProfile writes a TOML import profile to disk.
func SaveProfile(path string, profile Profile) error {
	return pipeline.SaveProfile(path, profile)
}

// ParseProfileTOML parses the supported import profile schema.
func ParseProfileTOML(data []byte) (Profile, error) {
	return pipeline.ParseProfileTOML(data)
}

// ResolveProfile returns the effective serializable profile represented by the given options.
func ResolveProfile(options ...Option) (Profile, error) {
	return pipeline.ResolveProfile(options...)
}

// BuiltInPresetNames returns the supported generic preset names.
func BuiltInPresetNames() []string {
	return pipeline.BuiltInPresetNames()
}

// NewRegistry returns a fresh registry containing all built-in decoders.
func NewRegistry() *detect.Registry {
	return decode.NewRegistry()
}

// SupportedFormats returns the built-in import formats exposed by the default registry.
func SupportedFormats() []ir.FormatID {
	return decode.DefaultRegistry().Formats()
}

// SupportedExtensions returns the built-in import file extensions exposed by the default registry.
func SupportedExtensions() []string {
	return decode.DefaultRegistry().Extensions()
}

// SupportsExtension reports whether the built-in registry supports the given file extension.
func SupportsExtension(ext string) bool {
	return decode.DefaultRegistry().SupportsExtension(ext)
}
