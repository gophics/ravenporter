package gltf

import (
	"bytes"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrimitiveMode(t *testing.T) {
	tests := []struct {
		in   int
		want ir.PrimitiveMode
	}{
		{0, ir.Points},
		{1, ir.Lines},
		{2, ir.LineLoop},
		{3, ir.LineStrip},
		{4, ir.Triangles},
		{5, ir.TriangleStrip},
		{6, ir.TriangleFan},
		{999, ir.Triangles}, // default
	}

	for _, tc := range tests {
		got := primitiveMode(tc.in)
		assert.Equal(t, tc.want, got)
	}
}

func TestCheckRequiredExtensions(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "Unsupported Draco",
			json:    `{"asset": {"version": "2.0"}, "extensionsRequired": ["KHR_draco_mesh_compression"]}`,
			wantErr: true,
		},
		{
			name:    "Completely unknown",
			json:    `{"asset": {"version": "2.0"}, "extensionsRequired": ["UNKNOWN_EXT"]}`,
			wantErr: true,
		},
		{
			name:    "Supported Ext",
			json:    `{"asset": {"version": "2.0"}, "extensionsRequired": ["KHR_materials_unlit"]}`,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := &Decoder{}
			_, err := dec.Decode(bytes.NewReader([]byte(tc.json)), detect.DecodeOptions{})
			if tc.wantErr {
				require.Error(t, err)
			} else {
				// the JSON is somewhat incomplete for a full decode, but checkRequiredExtensions runs first
				// so if it passes, it might fail later for other reasons (e.g. no nodes) but not extension error.
				// We actually only want to test the checkRequiredExtensions method directly.
				v := mustParse(tc.json)
				d := &doc{root: v}
				err2 := d.checkRequiredExtensions()
				assert.NoError(t, err2)
			}
		})
	}

	// Test the specific errors using the doc directly to bypass decode pipeline checks
	t.Run("Direct method test", func(t *testing.T) {
		for _, tc := range tests {
			v := mustParse(tc.json)
			d := &doc{root: v}
			err := d.checkRequiredExtensions()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	})
}

func TestGetFloat32Slice(t *testing.T) {
	v := mustParse(`{"weights": [0.5, 1.5, 2.5], "empty": []}`)

	arr := getFloat32Slice(v, "weights")
	assert.Equal(t, []float32{0.5, 1.5, 2.5}, arr)

	arrEmpty := getFloat32Slice(v, "empty")
	assert.Nil(t, arrEmpty)

	arrMissing := getFloat32Slice(v, "missing")
	assert.Nil(t, arrMissing)
}

func TestResolvePointers(t *testing.T) {
	d := &doc{}

	tests := []struct {
		name     string
		prefix   string
		suffix   string
		resolveF func(string, accessor, *ir.AnimationChannel)
		wantTrg  ir.ChannelTarget
	}{
		{"Camera FOV", "", "perspective/yfov", d.resolveCameraPointer, ir.TargetCameraFOV},
		{"Camera Unk", "", "unknown", d.resolveCameraPointer, ir.TargetPointer},
		{"Light Color", "", "color", d.resolveLightPointer, ir.TargetLightColor},
		{"Light Intensity", "", "intensity", d.resolveLightPointer, ir.TargetLightIntensity},
		{"Light Unk", "", "unknown", d.resolveLightPointer, ir.TargetPointer},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ch ir.AnimationChannel
			tc.resolveF("0/"+tc.suffix, accessor{}, &ch)
			assert.Equal(t, tc.wantTrg, ch.Target)
		})
	}
}
