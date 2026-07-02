package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/cache"
	"github.com/urfave/cli/v2"
)

const cacheExt = ".rpcache"

func cookCmd() *cli.Command {
	return &cli.Command{
		Name:      "cook",
		Usage:     "Cook an asset file into a RavenPorter runtime cache",
		ArgsUsage: "<input>",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: flagOut, Aliases: []string{"o"}, Usage: "Output cache path (defaults to <input>.rpcache)"},
			&cli.StringFlag{
				Name:    flagPreset,
				Aliases: []string{"p"},
				Value:   ravenporter.BuiltInPresetQuality,
				Usage:   "Processing preset (fast, quality, max-quality)",
			},
			&cli.StringFlag{Name: flagProfileFile, Usage: "Load import profile from TOML"},
			&cli.StringFlag{Name: flagReportFile, Usage: "Write the structured import report as JSON"},
			&cli.BoolFlag{Name: flagEmbedTextures, Usage: "Embed textures during import processing"},
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
			&cli.StringFlag{
				Name:  flagCacheImagePixels,
				Value: cache.ImagePixelsNever.String(),
				Usage: "Persist decoded image pixels in the cache (never, if-present, always)",
			},
			&cli.Int64Flag{
				Name:  flagCacheMaxMedia,
				Usage: "Max total embedded media bytes written into the cache (0 = unlimited)",
			},
		}, processStepFlags()...),
		Action: runCook,
	}
}

func runCook(c *cli.Context) error {
	if err := requireSingleArg(c); err != nil {
		return err
	}

	logger := newLogger(c.Bool(flagVerbose), c.Bool(flagQuiet))
	opts, err := buildImportOptions(c, logger, true)
	if err != nil {
		return err
	}

	inputPath := c.Args().First()
	result, err := openAndDecode(c, inputPath, opts)
	if err != nil {
		return writeReport(c.String("report-file"), result, err)
	}

	outputPath := c.String(flagOut)
	if outputPath == "" {
		outputPath = defaultCookPath(inputPath)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), dirPerm); err != nil {
		return writeReport(c.String(flagReportFile), result, err)
	}
	cacheOptions, err := buildCacheWriteOptions(c)
	if err != nil {
		return writeReport(c.String(flagReportFile), result, err)
	}
	if err := writeCacheFile(outputPath, result, cacheOptions...); err != nil {
		return writeReport(c.String(flagReportFile), result, err)
	}

	return writeReport(c.String(flagReportFile), result, nil)
}

func defaultCookPath(inputPath string) string {
	return filepath.Join(filepath.Dir(inputPath), stripExt(filepath.Base(inputPath))+cacheExt)
}

func writeCacheFile(outputPath string, result *ravenporter.Result, options ...cache.Option) (err error) {
	dir := filepath.Dir(outputPath)
	tmp, err := os.CreateTemp(dir, filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(tmpPath))
		}
	}()

	if writeErr := cache.Write(tmp, result, options...); writeErr != nil {
		err = errors.Join(writeErr, tmp.Close())
		return err
	}
	if closeErr := tmp.Close(); closeErr != nil {
		err = closeErr
		return err
	}
	//nolint:gosec // CLI output path is intentionally user-controlled.
	if renameErr := os.Rename(tmpPath, outputPath); renameErr != nil {
		err = renameErr
		return err
	}
	return nil
}

func buildCacheWriteOptions(c *cli.Context) ([]cache.Option, error) {
	var options []cache.Option
	if flagIsSet(c, flagCacheImagePixels) {
		mode, err := cache.ParseImagePixelsMode(c.String(flagCacheImagePixels))
		if err != nil {
			return nil, err
		}
		options = append(options, cache.WithImagePixels(mode))
	}
	if flagIsSet(c, flagCacheMaxMedia) {
		options = append(options, cache.WithMaxEmbeddedMediaBytes(c.Int64(flagCacheMaxMedia)))
	}
	return options, nil
}
