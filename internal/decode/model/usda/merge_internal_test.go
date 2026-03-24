package usda

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
)

func TestMergeRefSceneOffsetsAssetWideReferences(t *testing.T) {
	dst := &ir.Asset{
		Images:    []*ir.ImageAsset{{Name: "dst-image"}},
		Textures:  []*ir.Texture{{Name: "dst-texture", ImageIndex: 0}},
		Cameras:   []*ir.Camera{{Name: "dst-camera"}},
		Lights:    []*ir.Light{{Name: "dst-light"}},
		Materials: []*ir.Material{{Name: "dst-material"}},
		Nodes: []ir.Node{{
			Name:        "dst-node",
			CameraIndex: 0,
			LightIndex:  0,
		}},
		RootNodes: []int{0},
	}

	src := &ir.Asset{
		Images: []*ir.ImageAsset{{Name: "src-image"}},
		Textures: []*ir.Texture{{
			Name:       "src-texture",
			ImageIndex: 0,
		}},
		Cameras: []*ir.Camera{{Name: "src-camera"}},
		Lights:  []*ir.Light{{Name: "src-light"}},
		Materials: []*ir.Material{{
			Name:             "src-material",
			BaseColorTexture: &ir.TextureRef{TextureIndex: 0},
		}},
		Nodes: []ir.Node{{
			Name:        "src-node",
			CameraIndex: 0,
			LightIndex:  0,
		}},
		RootNodes: []int{0},
	}

	mergeRefScene(dst, src)

	require.Len(t, dst.Images, 2)
	require.Len(t, dst.Textures, 2)
	require.Len(t, dst.Cameras, 2)
	require.Len(t, dst.Lights, 2)
	require.Len(t, dst.Nodes, 2)

	assert.Equal(t, 1, dst.Textures[1].ImageIndex)
	require.NotNil(t, dst.Materials[1].BaseColorTexture)
	assert.Equal(t, 1, dst.Materials[1].BaseColorTexture.TextureIndex)
	assert.Equal(t, 1, dst.Nodes[1].CameraIndex)
	assert.Equal(t, 1, dst.Nodes[1].LightIndex)
	assert.Equal(t, []int{0, 1}, dst.RootNodes)
}
