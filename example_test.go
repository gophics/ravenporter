package ravenporter_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/gophics/ravenporter"
)

func ExampleImportFS() {
	fsys := fstest.MapFS{
		"scene.obj": &fstest.MapFile{
			Data: []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"),
		},
	}

	result, err := ravenporter.ImportFS(context.Background(), fsys, "scene.obj")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(result.Report.Source.DetectedFormat, len(result.Asset.Meshes))
	// Output:
	// obj 1
}

func ExampleImportFSDir() {
	fsys := fstest.MapFS{
		"assets/a.obj": &fstest.MapFile{
			Data: []byte("o A\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"),
		},
		"assets/b.obj": &fstest.MapFile{
			Data: []byte("o B\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"),
		},
	}

	results, err := ravenporter.ImportFSDir(context.Background(), fsys, "assets")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(len(results), results[0].Report.Source.DetectedFormat, results[1].Report.Source.DetectedFormat)
	// Output:
	// 2 obj obj
}

func ExampleImportPath() {
	dir, err := os.MkdirTemp("", "ravenporter-example-*")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "scene.obj")
	if err := os.WriteFile(path, []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o600); err != nil {
		fmt.Println("error:", err)
		return
	}

	result, err := ravenporter.ImportPath(context.Background(), path, ravenporter.WithPreset(ravenporter.BuiltInPresetFast))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(result.Report.Source.DetectedFormat, len(result.Asset.Meshes))
	// Output:
	// obj 1
}

func ExampleImportDir() {
	dir, err := os.MkdirTemp("", "ravenporter-example-dir-*")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "a.obj"), []byte("o A\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o600); err != nil {
		fmt.Println("error:", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "b.obj"), []byte("o B\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o600); err != nil {
		fmt.Println("error:", err)
		return
	}

	results, err := ravenporter.ImportDir(context.Background(), dir)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(len(results), results[0].Report.Source.DetectedFormat, results[1].Report.Source.DetectedFormat)
	// Output:
	// 2 obj obj
}

func ExampleImportBytes() {
	data := []byte("o Tri\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")

	result, err := ravenporter.ImportBytes(context.Background(), data, "scene.obj")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(result.Report.Source.DetectedFormat, len(result.Asset.Meshes))
	// Output:
	// obj 1
}

func ExampleResolveProfile() {
	profile, err := ravenporter.ResolveProfile(
		ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
		ravenporter.WithGlobalScale(2),
		ravenporter.WithEmbedTextures(),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(profile.Preset, profile.Process.EnabledSteps[0], *profile.Process.GlobalScale)
	// Output:
	// quality embed-textures 2
}
