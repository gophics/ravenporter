package main

import (
	"github.com/gophics/ravenporter"
	"github.com/urfave/cli/v2"
)

func profileCmd() *cli.Command {
	return &cli.Command{
		Name:  "profile",
		Usage: "Work with import profiles",
		Subcommands: []*cli.Command{
			profileExportCmd(),
		},
	}
}

func profileExportCmd() *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Write the effective import profile as TOML",
		Flags: append([]cli.Flag{
			&cli.StringFlag{Name: flagOut, Aliases: []string{"o"}, Required: true, Usage: "Output TOML path"},
			&cli.StringFlag{
				Name:    flagPreset,
				Aliases: []string{"p"},
				Value:   ravenporter.BuiltInPresetQuality,
				Usage:   "Base preset (fast, quality, max-quality)",
			},
			&cli.StringFlag{Name: flagProfileFile, Usage: "Load and merge an existing import profile from TOML"},
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
			&cli.BoolFlag{Name: flagVerbose, Aliases: []string{"v"}, Usage: "Enable debug-level logging"},
			&cli.BoolFlag{Name: flagQuiet, Aliases: []string{"q"}, Usage: "Suppress all output except errors"},
		}, processStepFlags()...),
		Action: runProfileExport,
	}
}

func runProfileExport(c *cli.Context) error {
	logger := newLogger(c.Bool(flagVerbose), c.Bool(flagQuiet))
	opts, err := buildImportOptions(c, logger, true)
	if err != nil {
		return err
	}
	profile, err := ravenporter.ResolveProfile(opts...)
	if err != nil {
		return err
	}
	return ravenporter.SaveProfile(c.String(flagOut), profile)
}
