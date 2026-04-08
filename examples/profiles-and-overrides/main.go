package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/gophics/ravenporter"
)

func run(w io.Writer) (err error) {
	profile, err := ravenporter.ResolveProfile(
		ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
		ravenporter.WithGlobalScale(0.5),
		ravenporter.WithEmbedTextures(),
	)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "ravenporter-profile-*")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(dir))
	}()

	profilePath := filepath.Join(dir, "quality.toml")
	if err := ravenporter.SaveProfile(profilePath, profile); err != nil {
		return err
	}

	loaded, err := ravenporter.LoadProfile(profilePath)
	if err != nil {
		return err
	}

	effective, err := ravenporter.ResolveProfile(
		ravenporter.WithProfile(loaded),
		ravenporter.WithGlobalScale(2),
	)
	if err != nil {
		return err
	}

	scenePath := filepath.Join(dir, "scene.obj")
	if err := os.WriteFile(scenePath, []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o600); err != nil {
		return err
	}

	result, err := ravenporter.ImportPath(
		context.Background(),
		scenePath,
		ravenporter.WithProfile(loaded),
		ravenporter.WithGlobalScale(2),
		ravenporter.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(
		w,
		"saved preset=%s scale=%s embed=%t\n",
		loaded.Preset,
		formatFloat(loaded.Process.GlobalScale),
		hasStep(loaded.Process.EnabledSteps, "embed-textures"),
	); err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		w,
		"effective preset=%s scale=%s embed=%t\n",
		effective.Preset,
		formatFloat(effective.Process.GlobalScale),
		hasStep(effective.Process.EnabledSteps, "embed-textures"),
	)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		w,
		"import format=%s meshes=%d\n",
		result.Report.Source.DetectedFormat,
		len(result.Asset.Meshes),
	)
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func hasStep(steps []string, name string) bool {
	return slices.Contains(steps, name)
}

func formatFloat(value *float64) string {
	if value == nil {
		return "unset"
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
