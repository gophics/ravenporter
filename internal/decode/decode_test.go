package decode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
)

func TestRegistrationsContainBuiltIns(t *testing.T) {
	registrations := Registrations()
	require.NotEmpty(t, registrations)

	formats := make(map[ir.FormatID]struct{}, len(registrations))
	for _, registration := range registrations {
		formats[registration.Format] = struct{}{}
	}

	assert.Contains(t, formats, ir.FormatGLTF)
	assert.Contains(t, formats, ir.FormatOBJ)
	assert.Contains(t, formats, ir.FormatPNG)
	assert.Contains(t, formats, ir.FormatWAV)
}

func TestNewRegistryIsIndependent(t *testing.T) {
	defaultFormats := DefaultRegistry().Formats()
	registry := NewRegistry()

	require.NotNil(t, registry)
	assert.NotSame(t, DefaultRegistry(), registry)
	assert.Equal(t, defaultFormats, registry.Formats())
}
