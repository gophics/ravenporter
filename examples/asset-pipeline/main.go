package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/gophics/ravenporter"
	"github.com/gophics/ravenporter/cache"
	jsonir "github.com/gophics/ravenporter/emit/json"
	"github.com/gophics/ravenporter/validate"
)

type pipelineSummary struct {
	path               string
	format             string
	meshes             int
	issues             int
	validationErrors   int
	validationWarnings int
	jsonMeshes         int
	cacheMeshes        int
}

type exportedAsset struct {
	Meshes []json.RawMessage `json:"Meshes"`
}

func run(w io.Writer) (err error) {
	ctx := context.Background()

	rootDir, err := os.MkdirTemp("", "ravenporter-pipeline-*")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(rootDir))
	}()

	assetsDir := filepath.Join(rootDir, "assets")
	if err := writePipelineAsset(assetsDir, filepath.Join("characters", "hero.obj"), "Hero"); err != nil {
		return err
	}
	if err := writePipelineAsset(assetsDir, filepath.Join("props", "crate.obj"), "Crate"); err != nil {
		return err
	}

	profile, err := ravenporter.ResolveProfile(
		ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
		ravenporter.WithGlobalScale(1.5),
	)
	if err != nil {
		return err
	}

	profilePath := filepath.Join(rootDir, "pipeline.toml")
	if err := ravenporter.SaveProfile(profilePath, profile); err != nil {
		return err
	}

	loadedProfile, err := ravenporter.LoadProfile(profilePath)
	if err != nil {
		return err
	}

	results, err := ravenporter.ImportDir(
		ctx,
		assetsDir,
		ravenporter.WithProfile(loadedProfile),
		ravenporter.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		return err
	}

	summaries := make([]pipelineSummary, 0, len(results))
	for _, result := range results {
		summary, err := summarizeAsset(assetsDir, result)
		if err != nil {
			return err
		}
		summaries = append(summaries, summary)
	}

	slices.SortFunc(summaries, func(a, b pipelineSummary) int {
		if n := cmp.Compare(a.path, b.path); n != 0 {
			return n
		}
		return cmp.Compare(a.format, b.format)
	})

	if _, err := fmt.Fprintf(
		w,
		"profile preset=%s scale=%s assets=%d\n",
		loadedProfile.Preset,
		formatFloat(loadedProfile.Process.GlobalScale),
		len(summaries),
	); err != nil {
		return err
	}

	for _, summary := range summaries {
		if _, err := fmt.Fprintf(
			w,
			"  %s format=%s meshes=%d issues=%d validation_errors=%d validation_warnings=%d json_meshes=%d cache_meshes=%d\n",
			summary.path,
			summary.format,
			summary.meshes,
			summary.issues,
			summary.validationErrors,
			summary.validationWarnings,
			summary.jsonMeshes,
			summary.cacheMeshes,
		); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writePipelineAsset(rootDir, relativePath, name string) error {
	path := filepath.Join(rootDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data := fmt.Sprintf("o %s\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n", name)
	return os.WriteFile(path, []byte(data), 0o600)
}

func summarizeAsset(baseDir string, result *ravenporter.Result) (summary pipelineSummary, err error) {
	relativePath, err := filepath.Rel(baseDir, result.Report.Source.InputPath)
	if err != nil {
		return summary, err
	}

	validation := validate.Asset(result.Asset)

	var jsonBuf bytes.Buffer
	if err := jsonir.WriteTo(result.Asset, &jsonBuf, false); err != nil {
		return summary, err
	}

	var exported exportedAsset
	if err := json.Unmarshal(jsonBuf.Bytes(), &exported); err != nil {
		return summary, err
	}

	var cacheBuf bytes.Buffer
	if err := cache.Write(&cacheBuf, result); err != nil {
		return summary, err
	}

	pkg, err := cache.Read(bytes.NewReader(cacheBuf.Bytes()), int64(cacheBuf.Len()))
	if err != nil {
		return summary, err
	}
	defer func() {
		err = errors.Join(err, pkg.Close())
	}()

	summary = pipelineSummary{
		path:               filepath.ToSlash(relativePath),
		format:             string(result.Report.Source.DetectedFormat),
		meshes:             len(result.Asset.Meshes),
		issues:             len(result.Report.Issues),
		validationErrors:   len(validation.Errors),
		validationWarnings: len(validation.Warnings),
		jsonMeshes:         len(exported.Meshes),
		cacheMeshes:        pkg.Manifest.Summary.Meshes,
	}

	return summary, err
}

func formatFloat(value *float64) string {
	if value == nil {
		return "unset"
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
