package bvh

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid BVH", []byte("HIERARCHY\nROOT"), true},
		{"glTF magic", []byte("glTF"), false},
		{"junk", []byte("not a bvh"), false},
		{"empty", nil, false},
		{"short", []byte("HIE"), false},
	}
	dec := &Decoder{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dec.Probe(bytes.NewReader(tc.data)))
		})
	}
}

func TestBVHDecodeAll(t *testing.T) {
	tests := []struct {
		name    string
		inputFn func() ([]byte, error)
		wantErr bool
		check   func(t *testing.T, scene *ir.Asset)
	}{
		{
			"ValidSimple",
			func() ([]byte, error) { return os.ReadFile("testdata/simple.bvh") },
			false,
			func(t *testing.T, scene *ir.Asset) {
				assert.Equal(t, ir.FormatBVH, scene.Metadata.SourceFormat)
				assert.Equal(t, ir.YUp, scene.UpAxis)
				require.Len(t, scene.Nodes, 3)
				assert.Equal(t, "Hips", scene.Nodes[0].Name)
				assert.Equal(t, "Spine", scene.Nodes[1].Name)
				assert.True(t, scene.Nodes[0].IsJoint)
				assert.True(t, scene.Nodes[1].IsJoint)
				assert.InDelta(t, float32(5.0), scene.Nodes[1].Transform.Translation[1], 0.01)
				require.Len(t, scene.Skeletons, 1)
				assert.Len(t, scene.Skeletons[0].Joints, 3)
				require.Len(t, scene.Animations, 1)
				anim := scene.Animations[0]
				assert.Greater(t, anim.Duration, 0.0)
				assert.NotEmpty(t, anim.Channels)
				hasTranslation, hasRotation := false, false
				for _, ch := range anim.Channels {
					if ch.NodeIndex == 0 && ch.Target == ir.TargetTranslation {
						hasTranslation = true
						require.Len(t, ch.Translations, 2)
					}
					if ch.NodeIndex == 0 && ch.Target == ir.TargetRotation {
						hasRotation = true
						require.Len(t, ch.Rotations, 2)
					}
				}
				assert.True(t, hasTranslation)
				assert.True(t, hasRotation)
				require.Len(t, scene.Nodes[0].Children, 1)
				assert.Equal(t, 1, scene.Nodes[0].Children[0])
			},
		},
		{
			"EmptyFile",
			func() ([]byte, error) { return []byte(""), nil },
			true,
			nil,
		},
		{
			"NoHierarchy",
			func() ([]byte, error) { return []byte("MOTION\nFrames: 0\n"), nil },
			true,
			nil,
		},
		{
			"HierarchyOnly",
			func() ([]byte, error) {
				return []byte("HIERARCHY\nROOT Test\n{\nOFFSET 0 0 0\nCHANNELS 3 Xrotation Yrotation Zrotation\n}\n"), nil
			},
			true,
			nil,
		},
	}

	dec := &Decoder{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.inputFn()
			require.NoError(t, err)

			scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tc.check != nil {
					tc.check(t, scene)
				}
			}
		})
	}
}

func TestExtensions(t *testing.T) {
	dec := &Decoder{}
	assert.Equal(t, []string{".bvh"}, dec.Extensions())
}

func TestFormatName(t *testing.T) {
	dec := &Decoder{}
	assert.Equal(t, "BVH", dec.FormatName())
}

func TestRegistered(t *testing.T) {
	_, ok := detect.NewRegistry(Registrations()...).Lookup(ir.FormatBVH)
	assert.True(t, ok)
}
