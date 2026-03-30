// Package pipeline exposes advanced import orchestration internals.
package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/gophics/ravenporter/validate"
	"golang.org/x/sync/errgroup"
)

type config struct {
	Registry          *detect.Registry
	ProcessFlags      process.PPFlag
	ProcessOpts       process.Options
	DecodeOpts        detect.DecodeOptions
	loadMask          LoadMask
	loadMaskSet       bool
	workerLimit       int
	Logger            *slog.Logger
	Preset            string
	ProfileFile       string
	profile           Profile
	profileResolvable bool
}

type batchInput struct {
	Reader     detect.ReadSeekerAt
	OpenReader func() (detect.ReadSeekerAt, error)
	Filename   string
	Path       string
	DecodeFS   detect.SeekableFS
	AssetDir   string
	AssetFS    detect.SeekableFS
}

func (o *config) workers() int {
	if o.workerLimit > 0 {
		return o.workerLimit
	}
	return runtime.GOMAXPROCS(0)
}

func importReader(ctx context.Context, r detect.ReadSeekerAt, filename string, opts config) (*Result, error) {
	opts = normalizeOptions(ctx, opts)
	reg := registryForOptions(opts)
	result := newImportResult(filename, opts)
	collector := newReportCollector()
	opts.DecodeOpts.Reporter = collector

	if err := ctx.Err(); err != nil {
		return result, contextError(result, StageDetect, err)
	}

	format, decoder, err := detectDecoder(reg, r, filename, opts, result)
	if err != nil {
		finalizeReport(result, nil, collector)
		return result, err
	}
	result.Report.Source.DetectedFormat = format

	if err := ctx.Err(); err != nil {
		finalizeReport(result, nil, collector)
		return result, contextError(result, StageDecode, err)
	}

	asset, err := decodeAsset(decoder, r, filename, opts, collector, result)
	if err != nil {
		finalizeReport(result, nil, collector)
		return result, err
	}
	asset.NormalizeGraph()

	if err := ctx.Err(); err != nil {
		finalizeReport(result, asset, collector)
		return result, contextError(result, StageValidateStructural, err)
	}

	if err := validateStructural(asset, opts.Logger, &result.Report); err != nil {
		finalizeReport(result, asset, collector)
		return result, err
	}

	if err := ctx.Err(); err != nil {
		finalizeReport(result, asset, collector)
		return result, contextError(result, StageProcess, err)
	}

	if err := processAsset(asset, opts, &result.Report); err != nil {
		finalizeReport(result, asset, collector)
		return result, err
	}

	applyLoadMask(asset, opts)

	if err := ctx.Err(); err != nil {
		finalizeReport(result, asset, collector)
		return result, contextError(result, StageValidateSemantic, err)
	}

	validateSemantic(asset, opts.Logger, &result.Report)
	result.Asset = asset
	finalizeReport(result, asset, collector)
	return result, nil
}

func importBatch(ctx context.Context, files []batchInput, opts config) ([]*Result, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	results := make([]*Result, len(files))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.workers())

	for i := range files {
		index := i
		input := files[i]
		g.Go(func() error {
			result, err := runBatchInput(gctx, input, opts)
			if err != nil {
				return fmt.Errorf("pipeline: %s: %w", input.Filename, err)
			}
			results[index] = result
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// ImportPath imports an asset from the local filesystem.
func ImportPath(ctx context.Context, filePath string, options ...Option) (result *Result, err error) {
	opts, err := resolveOptions(options...)
	if err != nil {
		return nil, err
	}
	root, rootFS, baseName, err := openPathRoot(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = joinCloseError(err, root)
	}()

	dir := filepath.Dir(filePath)
	opts = optionsForInput(opts, batchInput{
		DecodeFS: rootFS,
		AssetDir: dir,
		AssetFS:  rootFS,
	})

	reader, err := root.Open(baseName)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = joinCloseError(err, reader)
	}()

	result, err = importReader(ctx, reader, baseName, opts)
	if result != nil {
		result.Report.Source.InputPath = filePath
	}
	return result, err
}

// ImportFS imports an asset from an arbitrary fs.FS.
func ImportFS(ctx context.Context, fsys fs.FS, filePath string, options ...Option) (result *Result, err error) {
	opts, err := resolveOptions(options...)
	if err != nil {
		return nil, err
	}
	dirFS := ensureFSForPath(fsys, filePath)
	opts = optionsForInput(opts, batchInput{
		Path:     filePath,
		DecodeFS: dirFS,
		AssetFS:  dirFS,
	})

	reader, err := openFSFile(fsys, filePath)
	if err != nil {
		return nil, err
	}
	if closer, ok := reader.(io.Closer); ok {
		defer func() {
			err = joinCloseError(err, closer)
		}()
	}

	result, err = importReader(ctx, reader, filepath.Base(filePath), opts)
	if result != nil {
		result.Report.Source.InputPath = filePath
	}
	return result, err
}

// ImportDir imports every supported asset found under a local directory tree.
func ImportDir(ctx context.Context, dir string, options ...Option) ([]*Result, error) {
	opts, err := resolveOptions(options...)
	if err != nil {
		return nil, err
	}
	return importDir(ctx, dir, opts)
}

// ImportFSDir imports every supported asset found under a directory within an arbitrary fs.FS.
func ImportFSDir(ctx context.Context, fsys fs.FS, dir string, options ...Option) ([]*Result, error) {
	opts, err := resolveOptions(options...)
	if err != nil {
		return nil, err
	}
	return importFSDir(ctx, fsys, dir, opts)
}

// ImportReader imports an asset from a memory-backed or custom reader.
func ImportReader(ctx context.Context, reader detect.ReadSeekerAt, filename string, options ...Option) (*Result, error) {
	opts, err := resolveOptions(options...)
	if err != nil {
		return nil, err
	}
	return importReader(ctx, reader, filename, opts)
}

// ImportBytes imports an asset from an in-memory byte slice.
func ImportBytes(ctx context.Context, data []byte, filename string, options ...Option) (*Result, error) {
	return ImportReader(ctx, bytes.NewReader(data), filename, options...)
}

func importDir(ctx context.Context, dir string, opts config) ([]*Result, error) {
	reg := opts.Registry
	if reg == nil {
		reg = decode.DefaultRegistry()
	}

	var files []batchInput
	err := filepath.WalkDir(dir, func(filePath string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if reg.SupportsExtension(filepath.Ext(filePath)) {
			filePath := filePath
			fileDir := filepath.Dir(filePath)
			dirFS := ensureFS(os.DirFS(fileDir))
			files = append(files, batchInput{
				OpenReader: func() (detect.ReadSeekerAt, error) {
					return os.Open(filepath.Clean(filePath))
				},
				Filename: filepath.Base(filePath),
				Path:     filePath,
				DecodeFS: dirFS,
				AssetDir: fileDir,
				AssetFS:  dirFS,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return importBatch(ctx, files, opts)
}

func importFSDir(ctx context.Context, fsys fs.FS, dir string, opts config) ([]*Result, error) {
	reg := opts.Registry
	if reg == nil {
		reg = decode.DefaultRegistry()
	}

	var files []batchInput
	err := fs.WalkDir(fsys, dir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if reg.SupportsExtension(filepath.Ext(filePath)) {
			filePath := filePath
			dirFS := ensureFSForPath(fsys, filePath)
			files = append(files, batchInput{
				OpenReader: func() (detect.ReadSeekerAt, error) {
					return openFSFile(fsys, filePath)
				},
				Filename: filepath.Base(filePath),
				Path:     filePath,
				DecodeFS: dirFS,
				AssetFS:  dirFS,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return importBatch(ctx, files, opts)
}

// openFSFile opens a file from an fs.FS and ensures it implements detect.ReadSeekerAt.
func openFSFile(fsys fs.FS, filePath string) (detect.ReadSeekerAt, error) {
	f, err := fsys.Open(filePath)
	if err != nil {
		return nil, err
	}

	if rs, ok := f.(detect.ReadSeekerAt); ok {
		return rs, nil
	}

	data, err := io.ReadAll(f)
	closeErr := f.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return bytes.NewReader(data), nil
}

type osRootFS struct {
	root *os.Root
}

func (fsys osRootFS) Open(name string) (io.ReadCloser, error) {
	return fsys.root.Open(filepath.FromSlash(name))
}

func openPathRoot(filePath string) (*os.Root, detect.SeekableFS, string, error) {
	filePath = filepath.Clean(filePath)
	dir := filepath.Dir(filePath)
	baseName := filepath.Base(filePath)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, "", err
	}
	return root, osRootFS{root: root}, baseName, nil
}

func normalizeOptions(ctx context.Context, opts config) config {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ProcessOpts.Logger == nil {
		opts.ProcessOpts.Logger = opts.Logger
	}
	if opts.DecodeOpts.Context == nil {
		opts.DecodeOpts.Context = ctx
	}
	if !opts.loadMaskSet {
		opts.loadMask = LoadAll
	}
	return opts
}

func registryForOptions(opts config) *detect.Registry {
	if opts.Registry != nil {
		return opts.Registry
	}
	return decode.DefaultRegistry()
}

func newImportResult(filename string, opts config) *Result {
	return &Result{
		Report: Report{
			Source: SourceReport{
				InputName:   filename,
				Preset:      canonicalPresetName(opts.Preset),
				ProfileFile: opts.ProfileFile,
				Options:     effectiveProfileSnapshot(opts),
			},
		},
	}
}

func contextError(result *Result, stage string, err error) error {
	addIssue(&result.Report, stage, SeverityError, "CONTEXT_CANCELED", err.Error())
	return err
}

func detectDecoder(
	reg *detect.Registry,
	r detect.ReadSeekerAt,
	filename string,
	opts config,
	result *Result,
) (ir.FormatID, detect.Decoder, error) {
	format, err := reg.Detect(r, filename)
	if err != nil {
		addIssue(&result.Report, StageDetect, SeverityError, "DETECT_FAILED", err.Error())
		return "", nil, fmt.Errorf("pipeline: detect: %w", err)
	}

	opts.Logger.Debug("format detected", "format", format, "file", filename)
	decoder, ok := reg.Lookup(format)
	if !ok {
		addIssue(&result.Report, StageDetect, SeverityError, "NO_DECODER",
			fmt.Sprintf("no decoder registered for format %q", format))
		return "", nil, fmt.Errorf("pipeline: no decoder registered for format %q", format)
	}
	return format, decoder, nil
}

func decodeAsset(
	decoder detect.Decoder,
	r detect.ReadSeekerAt,
	filename string,
	opts config,
	collector *reportCollector,
	result *Result,
) (*ir.Asset, error) {
	opts.DecodeOpts.Sanitize()
	opts.Logger.Debug("calling decoder.Decode", "MaxFileSize", opts.DecodeOpts.MaxFileSize)
	asset, err := decoder.Decode(r, opts.DecodeOpts)
	if err != nil {
		addIssue(&result.Report, StageDecode, SeverityError, "DECODE_FAILED", err.Error())
		return nil, fmt.Errorf("pipeline: decode: %w", err)
	}

	opts.Logger.Debug("decode complete", "file", filename, "meshes", len(asset.Meshes), "nodes", len(asset.Nodes))
	collectAssetDependencies(asset, collector)
	return asset, nil
}

func validateStructural(asset *ir.Asset, logger *slog.Logger, report *Report) error {
	result := validate.Structural(asset)
	appendValidationIssues(report, StageValidateStructural, result)
	if result.OK() {
		for _, issue := range result.Warnings {
			logger.Warn("structural validation", "code", issue.Code, "msg", issue.Message)
		}
		return nil
	}

	for _, issue := range result.Errors {
		logger.Error("structural validation", "code", issue.Code, "msg", issue.Message)
	}
	for _, issue := range result.Warnings {
		logger.Warn("structural validation", "code", issue.Code, "msg", issue.Message)
	}
	return fmt.Errorf("pipeline: structural validation failed with %d errors", len(result.Errors))
}

func processAsset(asset *ir.Asset, opts config, report *Report) error {
	if opts.ProcessFlags == 0 {
		return nil
	}
	if err := process.Apply(asset, opts.ProcessFlags, opts.ProcessOpts); err != nil {
		addIssue(report, StageProcess, SeverityError, "PROCESS_FAILED", err.Error())
		return fmt.Errorf("pipeline: process: %w", err)
	}
	asset.NormalizeGraph()
	opts.Logger.Info("processing complete")
	return nil
}

func applyLoadMask(asset *ir.Asset, opts config) {
	if !opts.loadMaskSet || opts.loadMask == LoadAll {
		return
	}
	pruneAsset(asset, opts.loadMask)
}

func validateSemantic(asset *ir.Asset, logger *slog.Logger, report *Report) {
	result := validate.Semantic(asset)
	appendValidationIssues(report, StageValidateSemantic, result)
	if !result.OK() {
		logger.Warn("semantic validation errors", "count", len(result.Errors))
		for _, issue := range result.Errors {
			logger.Error("semantic validation", "code", issue.Code, "msg", issue.Message)
		}
	}
	for _, issue := range result.Warnings {
		logger.Warn("semantic validation", "code", issue.Code, "msg", issue.Message)
	}
}

func runBatchInput(ctx context.Context, input batchInput, opts config) (_ *Result, err error) {
	reader, closeReader, err := input.open()
	if err != nil {
		return nil, err
	}
	if closeReader != nil {
		defer func() {
			err = errors.Join(err, closeReader())
		}()
	}

	result, err := importReader(ctx, reader, input.Filename, optionsForInput(opts, input))
	if result != nil && input.Path != "" {
		result.Report.Source.InputPath = input.Path
	}
	return result, err
}

func (input batchInput) open() (detect.ReadSeekerAt, func() error, error) {
	if input.Reader != nil {
		return input.Reader, nil, nil
	}
	if input.OpenReader == nil {
		return nil, nil, fmt.Errorf("pipeline: missing reader for %q", input.Filename)
	}

	reader, err := input.OpenReader()
	if err != nil {
		return nil, nil, err
	}
	closer, ok := reader.(io.Closer)
	if !ok {
		return reader, nil, nil
	}
	return reader, closer.Close, nil
}

func joinCloseError(err error, closer io.Closer) error {
	if closer == nil {
		return err
	}
	return errors.Join(err, closer.Close())
}

func optionsForInput(opts config, input batchInput) config {
	clone := opts
	clone.DecodeOpts = opts.DecodeOpts
	clone.ProcessOpts = opts.ProcessOpts
	if clone.DecodeOpts.FS == nil && input.DecodeFS != nil {
		clone.DecodeOpts.FS = input.DecodeFS
	}
	if clone.ProcessOpts.AssetDir == "" && input.AssetDir != "" {
		clone.ProcessOpts.AssetDir = input.AssetDir
	}
	if clone.ProcessOpts.AssetFS == nil && input.AssetFS != nil {
		clone.ProcessOpts.AssetFS = input.AssetFS
	}
	return clone
}

func ensureFSForPath(fsys fs.FS, filePath string) detect.SeekableFS {
	dir := path.Dir(filePath)
	if dir == "." || dir == "" {
		return ensureFS(fsys)
	}
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		return ensureFS(fsys)
	}
	return ensureFS(sub)
}

func effectiveProfileSnapshot(opts config) Profile {
	return profileFromConfig(opts)
}

func finalizeReport(result *Result, asset *ir.Asset, collector *reportCollector) {
	if result == nil {
		return
	}
	if asset != nil {
		result.Report.Source.Metadata = asset.Metadata
		result.Report.Summary = assetSummary(asset)
	}
	result.Report.Dependencies = collector.dependenciesList()
	result.Report.Source.Notes = collector.noteMap()
}

func addIssue(report *Report, stage, severity, code, message string) {
	report.Issues = append(report.Issues, Issue{
		Stage:    stage,
		Severity: severity,
		Code:     code,
		Message:  message,
	})
}

func appendValidationIssues(report *Report, stage string, result *validate.Result) {
	if result == nil {
		return
	}
	for _, issue := range result.Errors {
		if issue == nil {
			continue
		}
		addIssue(report, stage, SeverityError, issue.Code, issue.Message)
	}
	for _, issue := range result.Warnings {
		if issue == nil {
			continue
		}
		addIssue(report, stage, SeverityWarning, issue.Code, issue.Message)
	}
}
