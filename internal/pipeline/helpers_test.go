package pipeline

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/ir"
)

func TestNormalizeOptionsAndRegistry(t *testing.T) {
	opts := normalizeOptions(context.Background(), config{})
	require.NotNil(t, opts.Logger)
	require.NotNil(t, opts.ProcessOpts.Logger)
	require.NotNil(t, opts.DecodeOpts.Context)
	assert.Equal(t, LoadAll, opts.loadMask)
	assert.Same(t, decode.DefaultRegistry(), registryForOptions(config{}))

	custom := detect.NewRegistry()
	assert.Same(t, custom, registryForOptions(config{Registry: custom}))
}

func TestJoinCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	err := joinCloseError(nil, closerFunc(func() error { return closeErr }))
	assert.ErrorIs(t, err, closeErr)

	baseErr := errors.New("base")
	err = joinCloseError(baseErr, closerFunc(func() error { return closeErr }))
	assert.ErrorIs(t, err, baseErr)
	assert.ErrorIs(t, err, closeErr)
}

func TestEnsureFSHelpers(t *testing.T) {
	fsys := fstest.MapFS{
		"nested/file.txt": &fstest.MapFile{Data: []byte("ok")},
	}

	dirFS := ensureFSForPath(fsys, "nested/file.txt")
	rc, err := dirFS.Open("file.txt")
	require.NoError(t, err)
	defer rc.Close()

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), data)

	rootFS := ensureFSForPath(fsys, "file.txt")
	rc, err = rootFS.Open("nested/file.txt")
	require.NoError(t, err)
	rc.Close()
}

func TestOptionsForInputAndNewImportResult(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config{
		Logger:   logger,
		Preset:   BuiltInPresetQuality,
		loadMask: LoadAll,
	}

	input := batchInput{
		Path:     "nested/file.mock",
		DecodeFS: ensureFS(fstest.MapFS{}),
		AssetDir: "nested",
		AssetFS:  ensureFS(fstest.MapFS{}),
	}
	updated := optionsForInput(cfg, input)
	assert.Equal(t, "nested", updated.ProcessOpts.AssetDir)
	require.NotNil(t, updated.DecodeOpts.FS)
	require.NotNil(t, updated.ProcessOpts.AssetFS)

	result := newImportResult("file.mock", cfg)
	assert.Equal(t, "file.mock", result.Report.Source.InputName)
	assert.Equal(t, BuiltInPresetQuality, result.Report.Source.Preset)
}

func TestReportCollectorAndFinalize(t *testing.T) {
	collector := newReportCollector()
	collector.AddDependency("texture", "b.png", "texture", "scene")
	collector.AddDependency("texture", "a.png", "texture", "scene")
	collector.AddDependency("texture", "a.png", "texture", "scene")
	collector.AddProvenanceNote("source", "b")
	collector.AddProvenanceNote("source", "a")
	collector.AddProvenanceNote("source", "a")

	report := &Result{}
	scene := &ir.Asset{
		Metadata: ir.AssetMetadata{SourceFormat: ir.FormatOBJ},
		Meshes:   []*ir.Mesh{{Name: "mesh"}},
		Nodes:    []ir.Node{{MeshIndex: 0, SkinIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex, LODGroupIndex: ir.NoIndex}},
	}
	finalizeReport(report, scene, collector)
	require.Len(t, report.Report.Dependencies, 2)
	assert.Equal(t, "a.png", report.Report.Dependencies[0].Path)
	assert.Equal(t, []string{"a", "b"}, report.Report.Source.Notes["source"])
	assert.Equal(t, 1, report.Report.Summary.Meshes)
}

func TestRunBatchInputErrorPaths(t *testing.T) {
	_, err := runBatchInput(context.Background(), batchInput{Filename: "missing.mock"}, config{})
	assert.Error(t, err)

	input := batchInput{
		Filename: "file.mock",
		OpenReader: func() (detect.ReadSeekerAt, error) {
			return &closableReader{Reader: bytes.NewReader([]byte("glTF"))}, nil
		},
	}
	reg := detect.NewRegistry()
	reg.Register(ir.FormatGLB, &mockDecoder{scene: &ir.Asset{Metadata: ir.AssetMetadata{SourceFormat: ir.FormatGLB}}})
	result, err := runBatchInput(context.Background(), input, config{Registry: reg})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAppendValidationIssuesAndAddIssue(t *testing.T) {
	report := &Report{}
	addIssue(report, StageDecode, SeverityError, "CODE", "message")
	require.Len(t, report.Issues, 1)

	appendValidationIssues(report, StageValidateStructural, nil)
	assert.Len(t, report.Issues, 1)
}

type closerFunc func() error

func (c closerFunc) Close() error { return c() }

type closableReader struct {
	*bytes.Reader
	closed bool
}

func (r *closableReader) Close() error {
	r.closed = true
	return nil
}
