package models_test

import (
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToPBR(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{{
			Name: "PhongMat",
			Properties: map[string]any{
				"DiffuseColor":  [3]float32{0.8, 0.2, 0.1},
				"SpecularPower": float32(64),
				"SpecularColor": [3]float32{1.0, 1.0, 1.0},
			},
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPConvertToPBR, process.Options{}))

	mat := scene.Materials[0]
	assert.InDelta(t, 0.8, mat.BaseColorFactor[0], 0.001)
	assert.InDelta(t, 0.5, mat.RoughnessFactor, 0.01)
	assert.Greater(t, mat.MetallicFactor, float32(0), "specular luminance should produce positive metallic")
}

func TestConvertToPBRSkipsExisting(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{{
			Name:            "PBRMat",
			BaseColorFactor: [4]float32{0.5, 0.5, 0.5, 1.0},
			MetallicFactor:  0.3,
			RoughnessFactor: 0.7,
		}},
	}

	require.NoError(t, process.Apply(scene, process.PPConvertToPBR, process.Options{}))

	mat := scene.Materials[0]
	assert.InDelta(t, 0.3, mat.MetallicFactor, 0.001, "existing PBR should be unchanged")
	assert.InDelta(t, 0.7, mat.RoughnessFactor, 0.001)
}

func TestConvertToPBRSkipsPBRMaterials(t *testing.T) {
	scene := &ir.Asset{
		Materials: []*ir.Material{
			{
				Name:            "dielectric",
				BaseColorFactor: [4]float32{1, 1, 1, 1},
				MetallicFactor:  0, // zero metallic = pure dielectric
				RoughnessFactor: 0.5,
			},
		},
	}

	require.NoError(t, process.Apply(scene, process.PPConvertToPBR, process.Options{}))

	// Should NOT re-convert since BaseColorFactor is non-zero.
	assert.Equal(t, float32(0), scene.Materials[0].MetallicFactor)
	assert.Equal(t, float32(0.5), scene.Materials[0].RoughnessFactor)
}

func TestConvertToPBREdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		mat       *ir.Material
		wantMetal float64
		wantRough float64
		wantBaseR float64
	}{
		{
			name: "specular_glossiness",
			mat: &ir.Material{
				Name: "SpecGloss",
				Properties: map[string]any{
					"specularFactor":   [3]float32{0.5, 0.5, 0.5},
					"glossinessFactor": float32(0.8),
				},
			},
			wantMetal: 0.5, wantRough: 0.2,
		},
		{
			name: "roughness_legacy",
			mat: &ir.Material{
				Name: "Rough",
				Properties: map[string]any{
					"DiffuseColor":    [3]float32{0.5, 0.5, 0.5},
					"roughnessFactor": float32(0.6),
				},
			},
			wantRough: 0.6,
		},
		{
			name: "nil_properties_diffuse_only",
			mat: &ir.Material{
				Name:       "Legacy",
				Properties: map[string]any{"DiffuseColor": [3]float32{1, 0, 0}},
			},
			wantBaseR: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &ir.Asset{Materials: []*ir.Material{tt.mat}}
			require.NoError(t, process.Apply(scene, process.PPConvertToPBR, process.Options{}))
			if tt.wantMetal > 0 {
				assert.InDelta(t, tt.wantMetal, scene.Materials[0].MetallicFactor, 0.01)
			}
			if tt.wantRough > 0 {
				assert.InDelta(t, tt.wantRough, scene.Materials[0].RoughnessFactor, 0.01)
			}
			if tt.wantBaseR > 0 {
				assert.InDelta(t, tt.wantBaseR, scene.Materials[0].BaseColorFactor[0], 0.01)
			}
		})
	}

	t.Run("nil_material", func(t *testing.T) {
		scene := &ir.Asset{Materials: []*ir.Material{nil}}
		require.NoError(t, process.Apply(scene, process.PPConvertToPBR, process.Options{}))
	})

	t.Run("empty_materials", func(t *testing.T) {
		scene := &ir.Asset{Materials: []*ir.Material{}}
		require.NoError(t, process.Apply(scene, process.PPConvertToPBR, process.Options{}))
	})
}
