package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/gophics/ravenporter"
)

type batchSummary struct {
	path   string
	format string
}

func run(w io.Writer) (err error) {
	ctx := context.Background()

	rootDir, err := os.MkdirTemp("", "ravenporter-batch-*")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(rootDir))
	}()

	assetsDir := filepath.Join(rootDir, "assets")
	if err := writeBatchFile(assetsDir, "a.obj", objData("A")); err != nil {
		return err
	}
	if err := writeBatchFile(assetsDir, filepath.Join("nested", "b.obj"), objData("B")); err != nil {
		return err
	}
	if err := writeBatchFile(assetsDir, "ignored.txt", "ignore me"); err != nil {
		return err
	}
	if err := writeBatchFile(assetsDir, filepath.Join("nested", "c.bin"), string([]byte{0x01, 0x02})); err != nil {
		return err
	}
	if err := writeBatchFile(assetsDir, filepath.Join("another", "d.tmp"), "tmp"); err != nil {
		return err
	}
	if err := writeBatchFile(assetsDir, filepath.Join("another", "e.data"), "data"); err != nil {
		return err
	}

	localResults, err := ravenporter.ImportDir(ctx, assetsDir)
	if err != nil {
		return err
	}
	localSummaries, err := localBatchSummaries(assetsDir, localResults)
	if err != nil {
		return err
	}

	fsResults, err := ravenporter.ImportFSDir(ctx, os.DirFS(rootDir), "assets")
	if err != nil {
		return err
	}
	fsSummaries := fsBatchSummaries(fsResults)

	if _, err := fmt.Fprintf(w, "import-dir=%d\n", len(localSummaries)); err != nil {
		return err
	}
	for _, summary := range localSummaries {
		if _, err := fmt.Fprintf(w, "  %s -> %s\n", summary.path, summary.format); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "import-fs-dir=%d\n", len(fsSummaries)); err != nil {
		return err
	}
	for _, summary := range fsSummaries {
		if _, err := fmt.Fprintf(w, "  %s -> %s\n", summary.path, summary.format); err != nil {
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

func objData(name string) string {
	return fmt.Sprintf("o %s\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n", name)
}

func writeBatchFile(rootDir, relativePath, contents string) error {
	path := filepath.Join(rootDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(contents), 0o600)
}

func localBatchSummaries(baseDir string, results []*ravenporter.Result) ([]batchSummary, error) {
	summaries := make([]batchSummary, 0, len(results))
	for _, result := range results {
		rel, err := filepath.Rel(baseDir, result.Report.Source.InputPath)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, batchSummary{
			path:   filepath.ToSlash(rel),
			format: string(result.Report.Source.DetectedFormat),
		})
	}
	sortBatchSummaries(summaries)
	return summaries, nil
}

func fsBatchSummaries(results []*ravenporter.Result) []batchSummary {
	summaries := make([]batchSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, batchSummary{
			path:   filepath.ToSlash(result.Report.Source.InputPath),
			format: string(result.Report.Source.DetectedFormat),
		})
	}
	sortBatchSummaries(summaries)
	return summaries
}

func sortBatchSummaries(summaries []batchSummary) {
	slices.SortFunc(summaries, func(a, b batchSummary) int {
		if n := cmp.Compare(a.path, b.path); n != 0 {
			return n
		}
		return cmp.Compare(a.format, b.format)
	})
}
