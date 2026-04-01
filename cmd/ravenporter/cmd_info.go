package main

import (
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/ir"
	"github.com/urfave/cli/v2"
)

const triangleVerts = 3

// tw writes a formatted string to w, discarding the error (stdout never fails).
func tw(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...) //nolint:errcheck // stdout
}

// tln writes a line to w, discarding the error.
func tln(w io.Writer, args ...any) {
	fmt.Fprintln(w, args...) //nolint:errcheck // stdout
}

// ---------------------------------------------------------------------------
// Info Command
// ---------------------------------------------------------------------------

func infoCmd() *cli.Command {
	return &cli.Command{
		Name:      "info",
		Usage:     "Print asset information (format, vertex count, materials, etc.)",
		ArgsUsage: "<file> [file...]",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flagJSON, Usage: "Output as JSON"},
		},
		Action: runInfo,
	}
}

type assetInfo struct {
	File       string `json:"file"`
	Format     string `json:"format"`
	Meshes     int    `json:"meshes"`
	Materials  int    `json:"materials"`
	Vertices   int    `json:"vertices"`
	Triangles  int    `json:"triangles"`
	Animations int    `json:"animations"`
	Skeletons  int    `json:"skeletons"`
	Cameras    int    `json:"cameras"`
	Lights     int    `json:"lights"`
	Nodes      int    `json:"nodes"`
}

func runInfo(c *cli.Context) error {
	if err := requireArgs(c); err != nil {
		return err
	}

	asJSON := c.Bool(flagJSON)
	logger := quietLogger()

	if asJSON {
		infos := make([]assetInfo, 0, len(c.Args().Slice()))
		err := forEachFile(c, func(filename string) error {
			asset, err := openAsset(c, filename, []ravenporter.Option{ravenporter.WithLogger(logger)})
			if err != nil {
				return err
			}
			infos = append(infos, newAssetInfo(filepath.Base(filename), asset))
			return nil
		})
		if err != nil {
			return err
		}
		if len(infos) == 1 {
			return writeJSON(infos[0])
		}
		return writeJSON(infos)
	}

	return forEachFile(c, func(filename string) error {
		asset, err := openAsset(c, filename, []ravenporter.Option{ravenporter.WithLogger(logger)})
		if err != nil {
			return err
		}

		info := newAssetInfo(filepath.Base(filename), asset)

		w := newTabWriter()
		lo, hi := asset.SceneBoundingBox(0)
		tw(w, "File:\t%s\n", info.File)
		tw(w, "Format:\t%s\n", info.Format)
		tw(w, "Meshes:\t%d\n", info.Meshes)
		tw(w, "Materials:\t%d\n", info.Materials)
		tw(w, "Vertices:\t%d\n", info.Vertices)
		tw(w, "Triangles:\t%d\n", info.Triangles)
		tw(w, "Animations:\t%d\n", info.Animations)
		tw(w, "Skeletons:\t%d\n", info.Skeletons)
		tw(w, "Cameras:\t%d\n", info.Cameras)
		tw(w, "Lights:\t%d\n", info.Lights)
		tw(w, "Nodes:\t%d\n", info.Nodes)
		tw(w, "Bounds:\tmin(%.2f, %.2f, %.2f) max(%.2f, %.2f, %.2f)\n",
			lo[0], lo[1], lo[2], hi[0], hi[1], hi[2])
		return w.Flush()
	})
}

func newAssetInfo(file string, asset *ir.Asset) assetInfo {
	return assetInfo{
		File:       file,
		Format:     string(asset.Metadata.SourceFormat),
		Meshes:     len(asset.Meshes),
		Materials:  len(asset.Materials),
		Vertices:   asset.TotalVertexCount(),
		Triangles:  asset.TotalTriangleCount(),
		Animations: len(asset.Animations),
		Skeletons:  len(asset.Skeletons),
		Cameras:    len(asset.Cameras),
		Lights:     len(asset.Lights),
		Nodes:      len(asset.Nodes),
	}
}

// ---------------------------------------------------------------------------
// Inspect Command
// ---------------------------------------------------------------------------

func inspectCmd() *cli.Command {
	return &cli.Command{
		Name:      "inspect",
		Usage:     "Deep metadata inspection of an asset file",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flagJSON, Usage: "Output as JSON"},
		},
		Action: runInspect,
	}
}

func runInspect(c *cli.Context) error {
	if err := requireSingleArg(c); err != nil {
		return err
	}

	asset, err := openAsset(c, c.Args().First(), []ravenporter.Option{ravenporter.WithLogger(quietLogger())})
	if err != nil {
		return fmt.Errorf("%s: %w", c.Args().First(), err)
	}

	if c.Bool(flagJSON) {
		return writeJSON(asset)
	}
	printInspectText(asset)
	return nil
}

func printInspectText(asset *ir.Asset) {
	w := newTabWriter()

	tw(w, "Format:\t%s\n\n", asset.Metadata.SourceFormat)
	writeAudioSection(w, asset)
	writeFontSection(w, asset)
	writeMeshSection(w, asset)
	writeMaterialSection(w, asset)
	writeTextureSection(w, asset)
	writeImageSection(w, asset)

	w.Flush() //nolint:errcheck,gosec // stdout
}

func writeAudioSection(w *tabwriter.Writer, asset *ir.Asset) {
	if len(asset.AudioClips) == 0 {
		return
	}
	tln(w, "=== Audio Clips ===")
	for _, clip := range asset.AudioClips {
		tw(w, "  Name:\t%s\n", clip.Name)
		tw(w, "  Format:\t%s\n", clip.Format)
		tw(w, "  Sample Rate:\t%d Hz\n", clip.SampleRate)
		tw(w, "  Layout:\t%v\n", clip.Layout)
		tw(w, "  Bit Depth:\t%v\n", clip.BitDepth)
		tw(w, "  Duration:\t%v\n", clip.Duration)
		if clip.LoopStart != ir.NoIndex {
			tw(w, "  Loop:\t%d → %d\n", clip.LoopStart, clip.LoopEnd)
		}
		m := clip.Metadata
		if m.Title != "" || m.Artist != "" || m.Album != "" {
			tw(w, "  Title:\t%s\n", m.Title)
			tw(w, "  Artist:\t%s\n", m.Artist)
			tw(w, "  Album:\t%s\n", m.Album)
			tw(w, "  Genre:\t%s\n", m.Genre)
		}
	}
	tln(w)
}

func writeFontSection(w *tabwriter.Writer, asset *ir.Asset) {
	if len(asset.Fonts) == 0 {
		return
	}
	tln(w, "=== Fonts ===")
	for _, fnt := range asset.Fonts {
		tw(w, "  Name:\t%s\n", fnt.Name)
		tw(w, "  Format:\t%s\n", fnt.Format)
		if fnt.Family != "" {
			tw(w, "  Family:\t%s\n", fnt.Family)
			tw(w, "  Subfamily:\t%s\n", fnt.Subfamily)
		}
		if fnt.Vector != nil {
			v := fnt.Vector
			tw(w, "  UPM:\t%d\n", v.UnitsPerEm)
			tw(w, "  Glyphs:\t%d\n", v.GlyphCount)
			tw(w, "  Ascender:\t%d\n", v.Ascender)
			tw(w, "  Descender:\t%d\n", v.Descender)
		}
		for k, v := range fnt.Metadata {
			tw(w, "  %s:\t%s\n", k, v)
		}
	}
	tln(w)
}

func writeMeshSection(w *tabwriter.Writer, asset *ir.Asset) {
	if len(asset.Meshes) == 0 {
		return
	}
	tln(w, "=== Meshes ===")
	tw(w, "  Name\tVertices\tTriangles\tMaterial\n")
	for _, mesh := range asset.Meshes {
		for pi := range mesh.Primitives {
			p := &mesh.Primitives[pi]
			tris := len(p.Data.Indices) / triangleVerts
			if tris == 0 {
				tris = p.Data.VertexCount / triangleVerts
			}
			tw(w, "  %s\t%d\t%d\t%d\n",
				mesh.Name, p.Data.VertexCount, tris, p.MaterialIndex)
		}
	}
	tln(w)
}

func writeMaterialSection(w *tabwriter.Writer, asset *ir.Asset) {
	if len(asset.Materials) == 0 {
		return
	}
	tln(w, "=== Materials ===")
	for _, mat := range asset.Materials {
		tw(w, "  %s\tbase=[%.2f,%.2f,%.2f,%.2f]\tmetallic=%.2f\troughness=%.2f\n",
			mat.Name, mat.BaseColorFactor[0], mat.BaseColorFactor[1],
			mat.BaseColorFactor[2], mat.BaseColorFactor[3],
			mat.MetallicFactor, mat.RoughnessFactor)
	}
	tln(w)
}

func writeTextureSection(w *tabwriter.Writer, asset *ir.Asset) {
	if len(asset.Textures) == 0 {
		return
	}
	tln(w, "=== Textures ===")
	for _, tex := range asset.Textures {
		path := ""
		if tex != nil && tex.ImageIndex >= 0 && tex.ImageIndex < len(asset.Images) && asset.Images[tex.ImageIndex] != nil {
			path = asset.Images[tex.ImageIndex].SourcePath
		}
		tw(w, "  %s\timage=%d\tpath=%s\n", tex.Name, tex.ImageIndex, path)
	}
	tln(w)
}

func writeImageSection(w *tabwriter.Writer, asset *ir.Asset) {
	if len(asset.Images) == 0 {
		return
	}
	tln(w, "=== Images ===")
	for _, img := range asset.Images {
		tw(w, "  %s\t%dx%d\t%s\t%v\n", img.Name, img.Width, img.Height, img.Format, img.Channels)
	}
}
