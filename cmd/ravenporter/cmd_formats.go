package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

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
			seen := make(map[string]struct{}, len(formats))
			for _, fid := range formats {
				dec, _ := registry.Lookup(fid)
				exts := slices.Clone(dec.Extensions())
				slices.Sort(exts)

				key := dec.FormatName() + "\x00" + strings.Join(exts, "\x00")
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}

				entries = append(entries, entry{name: dec.FormatName(), exts: exts})
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
