package cache

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gophics/ravenporter"
)

func BenchmarkWriteRead(b *testing.B) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	b.Run("Write", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			if err := Write(&buf, result); err != nil {
				b.Fatalf("write failed: %v", err)
			}
		}
	})

	var cooked bytes.Buffer
	if err := Write(&cooked, result); err != nil {
		b.Fatalf("cache write setup failed: %v", err)
	}
	data := cooked.Bytes()

	b.Run("Read", func(b *testing.B) {
		b.ReportAllocs()
		reader := bytes.NewReader(data)
		size := int64(len(data))
		for i := 0; i < b.N; i++ {
			asset, err := Read(reader, size)
			if err != nil {
				b.Fatalf("read failed: %v", err)
			}
			if asset.Asset == nil {
				b.Fatal("read returned nil asset")
			}
			if err := asset.Close(); err != nil {
				b.Fatalf("close failed: %v", err)
			}
		}
	})
}

func BenchmarkOpenVsImportPath(b *testing.B) {
	sourcePath := filepath.Join("testdata", "gltf_meshopt_indices.gltf")
	if _, err := os.Stat(sourcePath); err != nil {
		b.Skipf("benchmark asset missing: %v", err)
	}

	result, err := ravenporter.ImportPath(context.Background(), sourcePath)
	if err != nil {
		b.Fatalf("import setup failed: %v", err)
	}

	var cooked bytes.Buffer
	if err := Write(&cooked, result); err != nil {
		b.Fatalf("cache write setup failed: %v", err)
	}

	cachePath := filepath.Join(b.TempDir(), "scene.rpcache")
	if err := os.WriteFile(cachePath, cooked.Bytes(), 0o600); err != nil {
		b.Fatalf("cache file setup failed: %v", err)
	}

	b.Run("ImportPath", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			result, err := ravenporter.ImportPath(context.Background(), sourcePath)
			if err != nil {
				b.Fatalf("import failed: %v", err)
			}
			if result.Asset == nil {
				b.Fatal("import returned nil asset")
			}
		}
	})

	b.Run("CacheOpen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			asset, err := Open(cachePath)
			if err != nil {
				b.Fatalf("cache open failed: %v", err)
			}
			if asset.Asset == nil {
				b.Fatal("cache open returned nil asset")
			}
			if err := asset.Close(); err != nil {
				b.Fatalf("cache close failed: %v", err)
			}
		}
	})
}
