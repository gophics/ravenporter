package main

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/gophics/ravenporter"
	"github.com/urfave/cli/v2"
)

func formatsCmd() *cli.Command {
	return &cli.Command{
		Name:  "formats",
		Usage: "List all supported import formats",
		Action: func(_ *cli.Context) error {
			registry := ravenporter.NewRegistry()
			formats := registry.Formats()

			type entry struct {
				name string
				exts []string
			}
			entries := make([]entry, 0, len(formats))
			for _, fid := range formats {
				dec, _ := registry.Lookup(fid)
				entries = append(entries, entry{name: dec.FormatName(), exts: dec.Extensions()})
			}
			slices.SortFunc(entries, func(a, b entry) int { return cmp.Compare(a.name, b.name) })

			w := newTabWriter()
			fmt.Fprintln(w, "Supported import formats:") //nolint:errcheck // stdout
			for _, e := range entries {
				fmt.Fprintf(w, "  %s\t%v\n", e.name, e.exts) //nolint:errcheck // stdout
			}
			return w.Flush()
		},
	}
}
