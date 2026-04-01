package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/emit"
	emitjson "github.com/gophics/ravenporter/emit/json"
	"github.com/urfave/cli/v2"
)

func importCmd() *cli.Command {
	return &cli.Command{
		Name:      "import",
		Usage:     "Import an asset file and emit JSON IR",
		ArgsUsage: "<file> [file...]",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: flagOut, Aliases: []string{"o"}, Value: ".", Usage: "Output directory"},
			&cli.StringFlag{
				Name:    flagPreset,
				Aliases: []string{"p"},
				Value:   ravenporter.BuiltInPresetQuality,
				Usage:   "Processing preset (fast, quality, max-quality)",
			},
			&cli.StringFlag{Name: flagProfileFile, Usage: "Load import profile from TOML"},
			&cli.StringFlag{Name: flagReportFile, Usage: "Write the structured import report as JSON (single input only)"},
			&cli.BoolFlag{Name: flagPretty, Usage: "Pretty-print output"},
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
		Action: runImport,
	}
}

func runImport(c *cli.Context) error {
	if err := requireArgs(c); err != nil {
		return err
	}

	inputs := expandGlobs(c.Args().Slice())
	reportPath := c.String(flagReportFile)
	if reportPath != "" && len(inputs) != 1 {
		return fmt.Errorf("--%s only supports a single input", flagReportFile)
	}

	logger := newLogger(c.Bool(flagVerbose), c.Bool(flagQuiet))
	opts, err := buildImportOptions(c, logger, true)
	if err != nil {
		return err
	}

	outDir := c.String(flagOut)
	pretty := c.Bool(flagPretty)

	return forEachFile(c, func(filename string) error {
		result, err := openAndDecode(c, filename, opts)
		if err != nil {
			return writeReport(reportPath, result, err)
		}

		outFS := &cliOutputFS{root: outDir}
		baseName := stripExt(filepath.Base(filename))
		if err := (&emitjson.Emitter{}).Emit(result.Asset, outFS, emit.Options{
			BaseName:    baseName,
			Logger:      logger,
			PrettyPrint: pretty,
		}); err != nil {
			return writeReport(reportPath, result, err)
		}

		return writeReport(reportPath, result, nil)
	})
}

func exportCmd() *cli.Command {
	return &cli.Command{
		Name:      "export",
		Usage:     "Export an asset to a target format",
		ArgsUsage: "<input>",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: flagFormat, Aliases: []string{"f"}, Required: true, Usage: "Output format (json)"},
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

func runExport(c *cli.Context) error {
	if err := requireSingleArg(c); err != nil {
		return err
	}

	logger := newLogger(c.Bool(flagVerbose), c.Bool(flagQuiet))
	filename := c.Args().First()
	outPath := c.String(flagOut)
	formatName := c.String(flagFormat)
	if formatName == "" {
		formatName = strings.TrimPrefix(strings.ToLower(filepath.Ext(outPath)), ".")
	}
	opts, err := buildImportOptions(c, logger, false)
	if err != nil {
		return err
	}

	result, err := openAndDecode(c, filename, opts)
	if err != nil {
		return writeReport(c.String(flagReportFile), result, fmt.Errorf("%s: %w", filepath.Base(filename), err))
	}

	emitter, err := lookupEmitter(formatName)
	if err != nil {
		return writeReport(c.String(flagReportFile), result, err)
	}

	outDir := filepath.Dir(outPath)
	baseName := stripExt(filepath.Base(outPath))
	outFS := &cliOutputFS{root: outDir}
	if err := emitter.Emit(result.Asset, outFS, emit.Options{
		BaseName:      baseName,
		Logger:        logger,
		PrettyPrint:   c.Bool(flagPretty),
		EmbedTextures: c.Bool(flagEmbedTextures),
	}); err != nil {
		return writeReport(c.String(flagReportFile), result, err)
	}
	return writeReport(c.String(flagReportFile), result, nil)
}

func lookupEmitter(format string) (emit.Emitter, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return &emitjson.Emitter{}, nil
	default:
		return nil, fmt.Errorf("unsupported export format %q (only 'json' is currently available)", format)
	}
}
