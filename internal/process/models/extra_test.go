package models_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbedTexturesTable(t *testing.T) {
	tmp := t.TempDir()
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "diffuse.png"), pngData, 0o644))

	tests := []struct {
		name           string
		texture        *ir.Texture
		image          *ir.ImageAsset
		assetDir       string
		wantCompressed bool
		wantSourcePath string
	}{
		{
			name:           "embed existing file",
			texture:        &ir.Texture{ImageIndex: 0},
			image:          &ir.ImageAsset{SourcePath: "diffuse.png"},
			assetDir:       tmp,
			wantCompressed: true,
			wantSourcePath: "",
		},
		{
			name:           "skip missing file",
			texture:        &ir.Texture{ImageIndex: 0},
			image:          &ir.ImageAsset{SourcePath: "nonexistent.png"},
			assetDir:       tmp,
			wantCompressed: false,
			wantSourcePath: "nonexistent.png",
		},
		{
			name:           "skip without asset dir",
			texture:        &ir.Texture{ImageIndex: 0},
			image:          &ir.ImageAsset{SourcePath: "diffuse.png"},
			assetDir:       "",
			wantCompressed: false,
			wantSourcePath: "diffuse.png",
		},
		{
			name:           "skip already embedded",
			texture:        &ir.Texture{ImageIndex: 0},
			image:          &ir.ImageAsset{Compressed: []byte{1, 2, 3}},
			assetDir:       tmp,
			wantCompressed: true,
			wantSourcePath: "",
		},
		{
			name:           "nil texture",
			texture:        nil,
			image:          nil,
			assetDir:       tmp,
			wantCompressed: false,
			wantSourcePath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{
				Textures: []*ir.Texture{tt.texture},
				Images:   []*ir.ImageAsset{tt.image},
			}
			opts := process.Options{AssetDir: tt.assetDir}
			require.NoError(t, process.Apply(asset, process.PPEmbedTextures, opts))
			tex := asset.Textures[0]
			if tex == nil {
				return
			}
			img := asset.Images[tex.ImageIndex]
			if tt.wantCompressed {
				assert.NotEmpty(t, img.Compressed)
			} else {
				assert.Empty(t, img.Compressed)
			}
			assert.Equal(t, tt.wantSourcePath, img.SourcePath)
		})
	}
}

func TestEmbedTexturesEmptyScene(t *testing.T) {
	scene := &ir.Asset{}
	require.NoError(t, process.Apply(scene, process.PPEmbedTextures, process.Options{}))
}
