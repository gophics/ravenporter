package obj

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestParseMTLReportsDependency(t *testing.T) {
	reporter := &stubReporter{}
	asset := &ir.Asset{}
	fsys := &stubFS{files: map[string]string{
		"test.mtl": "newmtl red\nKd 1 0 0\n",
	}}

	require.NoError(t, parseMTL(fsys, reporter, "test.mtl", 0, asset))

	require.Len(t, reporter.dependencies, 1)
	assert.Equal(t, dependencyRecord{
		kind:       "material-library",
		path:       "test.mtl",
		relation:   "material-library",
		reportedBy: objReportedBy,
	}, reporter.dependencies[0])
	require.Len(t, asset.Materials, 1)
	assert.Equal(t, "red", asset.Materials[0].Name)
}

func TestParseMTLRejectsOversizedSidecar(t *testing.T) {
	reporter := &stubReporter{}
	asset := &ir.Asset{}
	fsys := &stubFS{files: map[string]string{
		"test.mtl": strings.Repeat("newmtl red\n", 64),
	}}

	err := parseMTL(fsys, reporter, "test.mtl", 16, asset)
	require.Error(t, err)
	assert.ErrorContains(t, err, "file exceeds MaxFileSize limit")
}

type stubReporter struct {
	dependencies []dependencyRecord
}

func (s *stubReporter) AddDependency(kind, path, relation, reportedBy string) {
	s.dependencies = append(s.dependencies, dependencyRecord{
		kind:       kind,
		path:       path,
		relation:   relation,
		reportedBy: reportedBy,
	})
}

func (s *stubReporter) AddProvenanceNote(_, _ string) {}

type dependencyRecord struct {
	kind       string
	path       string
	relation   string
	reportedBy string
}

type stubFS struct {
	files map[string]string
}

func (s *stubFS) Open(name string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(s.files[name])), nil
}

var _ detect.DecodeReporter = (*stubReporter)(nil)
var _ detect.SeekableFS = (*stubFS)(nil)
