package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

const (
	appVersion = "0.1.0"
	exitFail   = 1
)

func main() {
	app := &cli.App{
		Name:    "ravenporter",
		Usage:   "High-performance pure Go asset importer & converter",
		Version: appVersion,
		Commands: []*cli.Command{
			importCmd(),
			cookCmd(),
			exportCmd(),
			batchCmd(),
			convertCmd(),
			stepsCmd(),
			profileCmd(),
			infoCmd(),
			inspectCmd(),
			validateCmd(),
			formatsCmd(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		exitCode := exitFail
		var exitErr cli.ExitCoder
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitCode)
	}
}
