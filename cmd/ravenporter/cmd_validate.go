package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/rperr"
	"github.com/gophics/ravenporter/validate"
	"github.com/urfave/cli/v2"
)

type validationOutput struct {
	File     string                   `json:"file"`
	Format   string                   `json:"format,omitempty"`
	OK       bool                     `json:"ok"`
	Errors   []*rperr.ValidationError `json:"errors,omitempty"`
	Warnings []*rperr.ValidationError `json:"warnings,omitempty"`
}

func validateCmd() *cli.Command {
	return &cli.Command{
		Name:      "validate",
		Usage:     "Validate a 3D asset file for structural and semantic errors",
		ArgsUsage: "<file> [file...]",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flagJSON, Usage: "Output validation results as JSON"},
			&cli.BoolFlag{Name: flagVerbose, Aliases: []string{"v"}, Usage: "Enable debug-level logging"},
		},
		Action: runValidate,
	}
}

func runValidate(c *cli.Context) error {
	if err := requireArgs(c); err != nil {
		return err
	}

	asJSON := c.Bool(flagJSON)
	var logger *slog.Logger
	if !asJSON {
		logger = newLogger(c.Bool(flagVerbose), false)
	}
	allOK := true
	outputs := make([]validationOutput, 0, len(c.Args().Slice()))

	for _, filename := range expandGlobs(c.Args().Slice()) {
		result, err := validateFile(filename, logger)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(filename), err)
		}
		outputs = append(outputs, result)
		if !result.OK {
			allOK = false
		}
	}

	if asJSON {
		if len(outputs) == 1 {
			if err := writeJSON(outputs[0]); err != nil {
				return err
			}
		} else if err := writeJSON(outputs); err != nil {
			return err
		}
	}

	if !allOK {
		if asJSON {
			return cli.Exit("", exitFail)
		}
		return cli.Exit("validation failed", exitFail)
	}
	return nil
}

func validateFile(filename string, logger *slog.Logger) (validationOutput, error) {
	registry := ravenporter.NewRegistry()
	output := validationOutput{File: filepath.Base(filename)}

	f, err := os.Open(filename) //nolint:gosec // CLI user-provided path
	if err != nil {
		return output, err
	}
	defer f.Close() //nolint:errcheck // best-effort close

	format, err := registry.Detect(f, filepath.Base(filename))
	if err != nil {
		return output, fmt.Errorf("detect: %w", err)
	}
	output.Format = string(format)
	if logger != nil {
		logger.Info("detected", "format", format, "file", filename)
	}

	dec, ok := registry.Lookup(format)
	if !ok {
		return output, fmt.Errorf("no decoder for %q", format)
	}

	asset, err := dec.Decode(f, detect.DecodeOptions{})
	if err != nil {
		return output, fmt.Errorf("decode: %w", err)
	}

	result := validate.Asset(asset)
	output.OK = result.OK()
	output.Errors = result.Errors
	output.Warnings = result.Warnings

	if logger != nil {
		for _, e := range result.Errors {
			logger.Error("error", "code", e.Code, "msg", e.Message)
		}
		for _, w := range result.Warnings {
			logger.Warn("warning", "code", w.Code, "msg", w.Message)
		}
	}

	if output.OK && logger != nil {
		logger.Info("passed", "meshes", len(asset.Meshes), "materials", len(asset.Materials))
	}
	return output, nil
}
