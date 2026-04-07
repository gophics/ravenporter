package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gophics/ravenporter"
)

const (
	formatVersion  = uint16(2)
	headerSize     = 16
	tableEntrySize = 20
)

var (
	cacheMagic    = [8]byte{'R', 'P', 'C', 'A', 'C', 'H', 'E', 0}
	chunkManifest = [4]byte{'M', 'A', 'N', 'F'}
	chunkScene    = [4]byte{'S', 'C', 'E', 'N'}
	chunkBlob     = [4]byte{'B', 'L', 'O', 'B'}

	errInvalidCache = errors.New("cache: invalid asset")
	errNilResult    = errors.New("cache: nil result")
	errNilAsset     = errors.New("cache: nil asset")
)

type chunk struct {
	id   [4]byte
	data []byte
}

func manifestFromResult(result *ravenporter.Result) Manifest {
	sourceFormat := result.Report.Source.DetectedFormat
	if sourceFormat == "" && result.Asset != nil {
		sourceFormat = result.Asset.Metadata.SourceFormat
	}
	return Manifest{
		FormatVersion: formatVersion,
		SourceFormat:  sourceFormat,
		SourceProfile: result.Report.Source.Options,
		Dependencies:  cloneDependencies(result.Report.Dependencies),
		Notes:         cloneNotes(result.Report.Source.Notes),
		Summary:       result.Report.Summary,
	}
}

func marshalManifest(manifest Manifest) ([]byte, error) {
	return json.Marshal(manifest)
}

func unmarshalManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.FormatVersion != formatVersion {
		return Manifest{}, fmtErrorf("cache: unsupported manifest version %d", manifest.FormatVersion)
	}
	return manifest, nil
}

func fsOpen(path string) (*os.File, error) {
	return os.Open(filepath.Clean(path))
}

func fmtErrorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func cloneDependencies(in []ravenporter.Dependency) []ravenporter.Dependency {
	if len(in) == 0 {
		return nil
	}
	out := make([]ravenporter.Dependency, len(in))
	copy(out, in)
	return out
}

func cloneNotes(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		cloned := make([]string, len(values))
		copy(cloned, values)
		out[key] = cloned
	}
	return out
}

type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, io.EOF
	}
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
