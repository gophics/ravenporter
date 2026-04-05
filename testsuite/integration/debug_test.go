//go:build integration

package integration

import (
	"slices"
	"testing"

	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/ir"
)

func TestDefaultRegistryIncludesCoreFormats(t *testing.T) {
	registry := decode.DefaultRegistry()

	alembic, ok := registry.Lookup(ir.FormatAlembic)
	if !ok {
		t.Fatal("expected Alembic decoder to be registered")
	}
	if !slices.Contains(alembic.Extensions(), ".abc") {
		t.Fatalf("expected Alembic decoder to advertise .abc, got %v", alembic.Extensions())
	}

	stl, ok := registry.Lookup(ir.FormatSTL)
	if !ok {
		t.Fatal("expected STL decoder to be registered")
	}
	if !slices.Contains(stl.Extensions(), ".stl") {
		t.Fatalf("expected STL decoder to advertise .stl, got %v", stl.Extensions())
	}
}
