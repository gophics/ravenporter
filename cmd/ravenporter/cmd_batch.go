package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/emit"
	emitjson "github.com/gophics/ravenporter/emit/json"
	"github.com/urfave/cli/v2"
)

func batchCmd() *cli.Command {
	return &cli.Command{
		Name:      "batch",
		Usage:     "Process all supported files in a directory into JSON IR",
		ArgsUsage: "<directory>",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: flagOut, Aliases: []string{"o"}, Value: ".", Usage: "Output directory"},
			&cli.StringFlag{
				Name:    flagPreset,
				Aliases: []string{"p"},
				Value:   ravenporter.BuiltInPresetQuality,
				Usage:   "Processing preset (fast, quality, max-quality)",
			},
			&cli.StringFlag{Name: flagProfileFile, Usage: "Load import profile from TOML"},
			&cli.BoolFlag{Name: flagPretty, Usage: "Pretty-print output"},
			&cli.BoolFlag{Name: flagRecursive, Aliases: []string{"r"}, Usage: "Process subdirectories"},
			&cli.BoolFlag{Name: flagVerbose, Aliases: []string{"v"}, Usage: "Enable debug-level logging"},
			&cli.BoolFlag{Name: flagQuiet, Aliases: []string{"q"}, Usage: "Suppress all output except errors"},
			&cli.StringFlag{Name: flagUpAxis, Value: "Y", Usage: "Target up axis (Y|Z)"},
			&cli.Float64Flag{Name: flagScale, Value: 1.0, Usage: "Global scale factor"},
			&cli.IntFlag{
				Name:    flagDecodeMaxVertices,
				Aliases: []string{flagMaxVertices},
				Usage:   "Max vertices per mesh during decode (0 = unlimited)",
			},
			&cli.Int64Flag{
				Name:    flagDecodeMaxFileSize,
				Aliases: []string{flagMaxFileSize},
				Usage:   "Max input file size in bytes during decode (0 = unlimited)",
			},
			&cli.BoolFlag{Name: flagEmbedTextures, Usage: "Embed textures during import processing"},
		}, processStepFlags()...),
		Action: runBatch,
	}
}

func runBatch(c *cli.Context) error {
	if err := requireSingleArg(c); err != nil {
		return err
	}

	dir := c.Args().First()
	outDir := c.String(flagOut)
	pretty := c.Bool(flagPretty)
	recursive := c.Bool(flagRecursive)
	logger := newLogger(c.Bool(flagVerbose), c.Bool(flagQuiet))
	registry := ravenporter.NewRegistry()

	opts, err := buildImportOptions(c, logger, true)
	if err != nil {
		return err
	}

	var processed int

	walkFn := func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		if !registry.SupportsExtension(filepath.Ext(path)) {
			return nil
		}

		result, err := openAndDecode(c, path, opts)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}

		baseName := stripExt(filepath.Base(path))
		outFS := &cliOutputFS{root: outDir}
		if err := (&emitjson.Emitter{}).Emit(result.Asset, outFS, emit.Options{
			BaseName:    baseName,
			Logger:      logger,
			PrettyPrint: pretty,
		}); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}

		processed++
		logger.Info("processed", "file", filepath.Base(path))
		return nil
	}

	if err := filepath.WalkDir(dir, walkFn); err != nil {
		return fmt.Errorf("walk %s: %w", dir, err)
	}

	logger.Info("batch complete", "processed", processed)
	return nil
}

func convertCmd() *cli.Command {
	return &cli.Command{
		Name:      "convert",
		Usage:     "Convert an asset file (shorthand for import + export)",
		ArgsUsage: "<input> -o <output>",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: flagOut, Aliases: []string{"o"}, Required: true, Usage: "Output path"},
			&cli.StringFlag{
				Name:    flagPreset,
				Aliases: []string{"p"},
				Value:   ravenporter.BuiltInPresetQuality,
				Usage:   "Processing preset (fast, quality, max-quality)",
			},
			&cli.StringFlag{Name: flagProfileFile, Usage: "Load import profile from TOML"},
			&cli.StringFlag{Name: flagReportFile, Usage: "Write the structured import report as JSON"},
			&cli.BoolFlag{Name: flagPretty, Usage: "Pretty-print output"},
			&cli.BoolFlag{Name: flagEmbedTextures, Usage: "Embed textures inline in the emitted output"},
			&cli.BoolFlag{Name: flagVerbose, Aliases: []string{"v"}, Usage: "Enable debug-level logging"},
			&cli.BoolFlag{Name: flagQuiet, Aliases: []string{"q"}, Usage: "Suppress all output except errors"},
			&cli.StringFlag{Name: flagUpAxis, Value: "Y", Usage: "Target up axis (Y|Z)"},
			&cli.Float64Flag{Name: flagScale, Value: 1.0, Usage: "Global scale factor"},
			&cli.IntFlag{
				Name:    flagDecodeMaxVertices,
				Aliases: []string{flagMaxVertices},
				Usage:   "Max vertices per mesh during decode (0 = unlimited)",
			},
			&cli.Int64Flag{
				Name:    flagDecodeMaxFileSize,
				Aliases: []string{flagMaxFileSize},
				Usage:   "Max input file size in bytes during decode (0 = unlimited)",
			},
		}, processStepFlags()...),
		Action: runExport,
	}
}
