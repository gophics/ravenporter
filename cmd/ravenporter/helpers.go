package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/cache"
	"github.com/gophics/ravenporter/ir"
	"github.com/urfave/cli/v2"
)

// errNoInput is returned when a command receives no file arguments.
var errNoInput = fmt.Errorf("error: no input file specified")

// requireArgs guards every command that needs file arguments.
func requireArgs(c *cli.Context) error {
	if c.NArg() == 0 {
		return errNoInput
	}
	return nil
}

func requireSingleArg(c *cli.Context) error {
	if err := requireArgs(c); err != nil {
		return err
	}
	if c.NArg() != 1 {
		return fmt.Errorf("error: expected exactly 1 argument, got %d", c.NArg())
	}
	return nil
}

// openAndDecode runs the full import pipeline for a single OS path.
func openAndDecode(c *cli.Context, filename string, opts []ravenporter.Option) (*ravenporter.Result, error) {
	return ravenporter.ImportPath(c.Context, filename, opts...)
}

func openAsset(c *cli.Context, filename string, opts []ravenporter.Option) (*ir.Asset, error) {
	if strings.EqualFold(filepath.Ext(filename), cacheExt) {
		pkg, err := cache.Open(filename, cache.WithEagerMedia())
		if err != nil {
			return nil, err
		}
		asset := pkg.Asset
		if err := pkg.Close(); err != nil {
			return nil, err
		}
		return asset, nil
	}

	result, err := openAndDecode(c, filename, opts)
	if err != nil {
		return nil, err
	}
	return result.Asset, nil
}

// forEachFile iterates over glob-expanded arguments, calling fn for each file.
func forEachFile(c *cli.Context, fn func(filename string) error) error {
	for _, filename := range expandGlobs(c.Args().Slice()) {
		if err := fn(filename); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(filename), err)
		}
	}
	return nil
}

// expandGlobs expands filepath glob patterns in the given args.
// Non-glob args and patterns that match nothing are passed through as-is.
func expandGlobs(args []string) []string {
	var out []string
	for _, arg := range args {
		matches, err := filepath.Glob(arg)
		if err != nil || len(matches) == 0 {
			out = append(out, arg)
		} else {
			out = append(out, matches...)
		}
	}
	return out
}

// quietLogger returns a logger that discards all output.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newLogger builds a logger respecting the --verbose / --quiet flags.
func newLogger(verbose, quiet bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	if quiet {
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// newTabWriter creates a tabwriter aligned on tabs with 2-space padding.
func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0) //nolint:mnd // tab padding
}

// writeJSON encodes v to stdout as pretty-printed JSON.
func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeJSONFile(path string, v any) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	f, err := os.Create(path) //nolint:gosec // CLI path
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeReport(path string, result *ravenporter.Result, primaryErr error) error {
	if path == "" || result == nil {
		return primaryErr
	}
	if err := writeJSONFile(path, result.Report); err != nil {
		if primaryErr != nil {
			return fmt.Errorf("%w (also failed to write report: %v)", primaryErr, err)
		}
		return err
	}
	return primaryErr
}

func buildImportOptions(
	c *cli.Context,
	logger *slog.Logger,
	useEmbedTexturesFlag bool,
) ([]ravenporter.Option, error) {
	profilePath := strings.TrimSpace(c.String(flagProfileFile))
	options := []ravenporter.Option{
		ravenporter.WithLogger(logger),
		ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
	}
	if profilePath != "" {
		options = append(options, ravenporter.WithProfileFile(profilePath))
	}
	if flagIsSet(c, flagPreset) {
		options = append(options, ravenporter.WithPreset(strings.TrimSpace(c.String(flagPreset))))
	}
	if flagIsSet(c, flagScale) {
		options = append(options, ravenporter.WithGlobalScale(c.Float64(flagScale)))
	}
	if flagIsSet(c, flagUpAxis) {
		axis, err := parseCLIUpAxis(c.String(flagUpAxis))
		if err != nil {
			return nil, err
		}
		options = append(options, ravenporter.WithTargetUpAxis(axis))
	}
	if useEmbedTexturesFlag && flagIsSet(c, flagEmbedTextures) && c.Bool(flagEmbedTextures) {
		options = append(options, ravenporter.WithEmbedTextures())
	}
	enabledSteps := compactStringSlice(c.StringSlice(flagEnableStep))
	disabledSteps := compactStringSlice(c.StringSlice(flagDisableStep))
	if len(enabledSteps) > 0 || len(disabledSteps) > 0 {
		options = append(options, ravenporter.WithProfile(ravenporter.Profile{
			Version: ravenporter.ProfileVersion,
			Process: ravenporter.ProcessProfile{
				EnabledSteps:  enabledSteps,
				DisabledSteps: disabledSteps,
			},
		}))
	}
	if flagIsSet(c, flagDecodeMaxFileSize, flagMaxFileSize) {
		options = append(options, ravenporter.WithDecodeMaxFileSize(c.Int64(flagDecodeMaxFileSize)))
	}
	if flagIsSet(c, flagDecodeMaxVertices, flagMaxVertices) {
		options = append(options, ravenporter.WithDecodeMaxVertices(c.Int(flagDecodeMaxVertices)))
	}
	if _, err := ravenporter.ResolveProfile(options...); err != nil {
		return nil, err
	}
	return options, nil
}

func processStepFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{
			Name:  flagEnableStep,
			Usage: "Enable a canonical process step after preset/profile resolution (repeatable; see `ravenporter steps`)",
		},
		&cli.StringSliceFlag{
			Name:  flagDisableStep,
			Usage: "Disable a canonical process step after preset/profile resolution (repeatable; see `ravenporter steps`)",
		},
	}
}

func flagIsSet(c *cli.Context, names ...string) bool {
	for _, name := range names {
		if c.IsSet(name) {
			return true
		}
	}
	return false
}

func compactStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func stripExt(name string) string {
	if ext := filepath.Ext(name); ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
}

func parseCLIUpAxis(value string) (ir.Axis, error) {
	switch strings.TrimSpace(strings.ToUpper(value)) {
	case "Y":
		return ir.YUp, nil
	case "Z":
		return ir.ZUp, nil
	default:
		return ir.YUp, fmt.Errorf("unsupported up axis %q (supported: Y, Z)", value)
	}
}

// cliOutputFS adapts os file creation to the emit.OutputFS interface.
type cliOutputFS struct {
	root string
}

const dirPerm = 0o750

func (fs *cliOutputFS) Create(name string) (io.WriteCloser, error) {
	full := filepath.Join(fs.root, name)
	if err := os.MkdirAll(filepath.Dir(full), dirPerm); err != nil {
		return nil, err
	}
	return os.Create(full) //nolint:gosec // path from CLI
}
