package core_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type mockStep struct {
	name     string
	flag     core.PPFlag
	called   bool
	failMode bool
}

func (m *mockStep) Name() string      { return m.name }
func (m *mockStep) Flag() core.PPFlag { return m.flag }
func (m *mockStep) Apply(scene *ir.Asset, _ core.Options) (*ir.Asset, error) {
	m.called = true
	if m.failMode {
		return nil, errors.New("mock failure") //nolint:err113 // generic validation error
	}
	return scene, nil
}

func TestCoreRegistryAndApply(t *testing.T) {
	s1 := &mockStep{name: "TestStepSub", flag: core.PPTriangulate}
	s2 := &mockStep{name: "FailStepSub", flag: core.PPGenNormals, failMode: true}
	registry := core.NewRegistry(s1, s2)

	scene := &ir.Asset{}
	opts := core.Options{MaxBoneWeights: 4}

	t.Run("Successful Step Apply", func(t *testing.T) {
		err := registry.Apply(scene, core.PPTriangulate, opts)
		require.NoError(t, err, "PPTriangulate processing pipeline should not fail natively")
		assert.True(t, s1.called, "The step should be explicitly called on execution")
	})

	t.Run("Failure Cascade Apply", func(t *testing.T) {
		err := registry.Apply(scene, core.PPGenNormals, opts)
		require.Error(t, err, "Pipeline expecting error from cascading failure step")
	})

	t.Run("Clean Implicit Empty Pipeline", func(t *testing.T) {
		err := registry.Apply(scene, 1<<62, opts) // Invalid high block bit
		require.NoError(t, err, "Implicit empty or mismatched flag masks should run cleanly by design")
	})
}

func TestRegistrySortsUnknownFlagsLast(t *testing.T) {
	unknown := &mockStep{name: "Unknown", flag: 1 << 62}
	known := &mockStep{name: "Known", flag: core.PPTriangulate}

	registry := core.NewRegistry(unknown, known)
	steps := registry.Steps()

	require.Len(t, steps, 2)
	assert.Same(t, known, steps[0])
	assert.Same(t, unknown, steps[1])
}
