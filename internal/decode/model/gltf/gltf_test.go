package gltf

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fastjson"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

type mapFS map[string][]byte

func (m mapFS) Open(name string) (io.ReadCloser, error) {
	data, ok := m[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

const testMinimalGLTF = `{
  "asset": {"version": "2.0", "generator": "test"},
  "scene": 0,
  "scenes": [{"nodes": [0]}],
  "nodes": [{"name": "Root", "mesh": 0}],
  "meshes": [{"name": "Tri", "primitives": [{"attributes": {}}]}],
  "materials": [{"name": "Red", "pbrMetallicRoughness": {"baseColorFactor": [1, 0, 0, 1]}}]
}`

func TestConvertNodeTransformTRS(t *testing.T) {
	v := mustParse(`{
		"name": "Joint",
		"translation": [1, 2, 3],
		"rotation": [0, 0, 0, 1],
		"scale": [2, 2, 2]
	}`)
	node := convertNode(v)

	assert.Equal(t, [3]float32{1, 2, 3}, node.Transform.Translation)
	assert.Equal(t, [4]float32{0, 0, 0, 1}, node.Transform.Rotation)
	assert.Equal(t, [3]float32{2, 2, 2}, node.Transform.Scale)
}

func TestConvertMaterialAlphaBlend(t *testing.T) {
	v := mustParse(`{"alphaMode": "BLEND", "doubleSided": true}`)
	var mat ir.Material
	convertMaterial(v, &mat)

	assert.Equal(t, ir.AlphaBlend, mat.AlphaMode)
	assert.True(t, mat.DoubleSided)
}

func TestProbeGLB(t *testing.T) {
	d := &Decoder{}
	buf := bytes.NewReader([]byte("glTF\x02\x00\x00\x00\x00\x00\x00\x00"))
	assert.True(t, d.Probe(buf))
}

func TestProbeNonGLB(t *testing.T) {
	d := &Decoder{}
	buf := bytes.NewReader([]byte("RIFF"))
	assert.False(t, d.Probe(buf))
}

func TestExtensions(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, []string{extGLTF, extGLB}, d.Extensions())
}

func TestFormatName(t *testing.T) {
	d := &Decoder{}
	assert.Equal(t, formatName, d.FormatName())
}

func TestConvertCameraPerspective(t *testing.T) {
	v := mustParse(`{
		"name": "MainCam",
		"perspective": {"yfov": 0.7854, "znear": 0.1, "zfar": 100, "aspectRatio": 1.5}
	}`)
	var cam ir.Camera
	convertCamera(v, &cam)

	assert.Equal(t, "MainCam", cam.Name)
	require.NotNil(t, cam.Perspective)
	assert.InDelta(t, float32(0.7854), cam.Perspective.FOV, 0.001)
	assert.InDelta(t, float32(1.5), cam.Perspective.Aspect, 0.001)
	assert.InDelta(t, float32(0.1), cam.Perspective.Near, 0.001)
	assert.InDelta(t, float32(100.0), cam.Perspective.Far, 0.1)
	assert.Nil(t, cam.Orthographic)
}

func TestConvertCameraOrthographic(t *testing.T) {
	v := mustParse(`{
		"name": "OrthoView",
		"orthographic": {"xmag": 5, "ymag": 5, "znear": 0.1, "zfar": 50}
	}`)
	var cam ir.Camera
	convertCamera(v, &cam)

	assert.Nil(t, cam.Perspective)
	require.NotNil(t, cam.Orthographic)
	assert.InDelta(t, float32(5), cam.Orthographic.XMag, 0.001)
	assert.InDelta(t, float32(50), cam.Orthographic.Far, 0.1)
}

func TestDecodeFailsOnMissingExternalBuffer(t *testing.T) {
	src := []byte(`{
  "asset": {"version": "2.0"},
  "buffers": [{"uri": "missing.bin", "byteLength": 36}],
  "bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": 36}],
  "accessors": [{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"}],
  "meshes": [{"primitives": [{"attributes": {"POSITION": 0}}]}],
  "nodes": [{"mesh": 0}],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`)

	_, err := (&Decoder{}).Decode(bytes.NewReader(src), detect.DecodeOptions{FS: mapFS{}})
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing.bin")
}

func TestDecodeRejectsOversizedExternalBuffer(t *testing.T) {
	src := []byte(`{
  "asset": {"version": "2.0"},
  "buffers": [{"uri": "data.bin", "byteLength": 16}],
  "bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": 16}],
  "accessors": [{"bufferView": 0, "componentType": 5126, "count": 1, "type": "VEC4"}],
  "meshes": [{"primitives": [{"attributes": {"POSITION": 0}}]}],
  "nodes": [{"mesh": 0}],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`)

	_, err := (&Decoder{}).Decode(bytes.NewReader(src), detect.DecodeOptions{
		FS:          mapFS{"data.bin": make([]byte, 16)},
		MaxFileSize: 8,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "MaxFileSize")
}

func TestDecodeRejectsVertexLimitExceeded(t *testing.T) {
	buf := make([]byte, 72)
	values := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}
	for i, v := range values {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}

	src := []byte(`{
  "asset": {"version": "2.0"},
  "buffers": [{"uri": "data.bin", "byteLength": 72}],
  "bufferViews": [
    {"buffer": 0, "byteOffset": 0, "byteLength": 36},
    {"buffer": 0, "byteOffset": 36, "byteLength": 36}
  ],
  "accessors": [
    {"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"},
    {"bufferView": 1, "componentType": 5126, "count": 3, "type": "VEC3"}
  ],
  "meshes": [{"primitives": [{"attributes": {"POSITION": 0, "NORMAL": 1}}]}],
  "nodes": [{"mesh": 0}],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`)

	_, err := (&Decoder{}).Decode(bytes.NewReader(src), detect.DecodeOptions{
		FS:          mapFS{"data.bin": buf},
		MaxVertices: 1,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "vertex limit exceeded")
}

func TestConvertSkin(t *testing.T) {
	jsonStr := `{
		"asset": {"version": "2.0"},
		"skins": [{"name": "Armature", "joints": [0, 1, 2], "skeleton": 0}]
	}`
	d, err := parseDoc(&fastjson.Parser{}, []byte(jsonStr), nil, detect.DecodeOptions{})
	require.NoError(t, err)
	skels := d.convertSkins()

	require.Len(t, skels, 1)
	assert.Equal(t, "Armature", skels[0].Name)
	assert.Equal(t, []int{0, 1, 2}, skels[0].Joints)
	assert.Equal(t, 0, skels[0].RootIdx)
}

func TestMarkJointNodes(t *testing.T) {
	nodes := make([]ir.Node, 3)
	skeletons := []*ir.Skeleton{{Joints: []int{0, 2}}}
	markJointNodes(nodes, skeletons)

	assert.True(t, nodes[0].IsJoint)
	assert.False(t, nodes[1].IsJoint)
	assert.True(t, nodes[2].IsJoint)
}

func TestChannelTarget(t *testing.T) {
	assert.Equal(t, ir.TargetTranslation, channelTarget(pathTranslation))
	assert.Equal(t, ir.TargetRotation, channelTarget(pathRotation))
	assert.Equal(t, ir.TargetScale, channelTarget(pathScale))
	assert.Equal(t, ir.TargetMorphWeights, channelTarget(pathWeights))
}

func TestInterpolation(t *testing.T) {
	assert.Equal(t, ir.InterpolationLinear, interpolation(interpLinear))
	assert.Equal(t, ir.InterpolationStep, interpolation(interpStep))
	assert.Equal(t, ir.InterpolationCubicSpline, interpolation(interpCubicSpline))
}

func mustParse(s string) *fastjson.Value {
	var p fastjson.Parser
	v, err := p.Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

const testCamerasGLTF = `{
  "asset": {"version": "2.0"},
  "cameras": [
    {
      "name": "PerspCam",
      "type": "perspective",
      "perspective": {"yfov": 0.785, "znear": 0.1, "zfar": 100.0, "aspectRatio": 1.5}
    },
    {
      "name": "OrthoCam",
      "type": "orthographic",
      "orthographic": {"xmag": 5.0, "ymag": 5.0, "znear": 0.01, "zfar": 50.0}
    }
  ],
  "nodes": [
    {"name": "CamNode0", "camera": 0},
    {"name": "CamNode1", "camera": 1}
  ],
  "scenes": [{"nodes": [0, 1]}], "scene": 0
}`

const testLightsGLTF = `{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_lights_punctual"],
  "extensions": {
    "KHR_lights_punctual": {
      "lights": [
        {"name": "Sun", "type": "directional", "color": [1.0, 0.9, 0.8], "intensity": 5.0},
        {"name": "Bulb", "type": "point", "color": [1.0, 1.0, 1.0], "intensity": 100.0, "range": 25.0},
        {"name": "Spot1", "type": "spot", "color": [0.5, 0.5, 1.0], "intensity": 200.0, "range": 10.0,
         "spot": {"innerConeAngle": 0.1, "outerConeAngle": 0.5}}
      ]
    }
  },
  "nodes": [
    {"name": "SunNode", "extensions": {"KHR_lights_punctual": {"light": 0}}},
    {"name": "BulbNode", "extensions": {"KHR_lights_punctual": {"light": 1}}},
    {"name": "SpotNode", "extensions": {"KHR_lights_punctual": {"light": 2}}}
  ],
  "scenes": [{"nodes": [0, 1, 2]}], "scene": 0
}`

const testTexturesGLTF = `{
  "asset": {"version": "2.0"},
  "images": [
    {"mimeType": "image/png", "uri": "diffuse.png"},
    {"mimeType": "image/jpeg", "uri": "normal.jpg"}
  ],
  "samplers": [
    {"wrapS": 33071, "wrapT": 33648, "minFilter": 9728, "magFilter": 9729}
  ],
  "textures": [
    {"name": "DiffuseTex", "source": 0, "sampler": 0},
    {"name": "NormalTex", "source": 1}
  ],
  "materials": [{
    "name": "PBR",
    "pbrMetallicRoughness": {
      "baseColorTexture": {"index": 0},
      "metallicRoughnessTexture": {"index": 1}
    },
    "normalTexture": {"index": 0, "scale": 0.8},
    "occlusionTexture": {"index": 1, "strength": 0.5},
    "emissiveFactor": [0.2, 0.1, 0.0],
    "emissiveTexture": {"index": 0}
  }],
  "nodes": [{"name": "N"}],
  "scenes": [{"nodes": [0]}], "scene": 0
}`

const testTextureWebPGLTF = `{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["EXT_texture_webp"],
  "images": [
    {"mimeType": "image/png", "uri": "fallback.png"},
    {"mimeType": "image/webp", "uri": "diffuse.webp"}
  ],
  "textures": [{
    "name": "DiffuseTex",
    "source": 0,
    "extensions": {"EXT_texture_webp": {"source": 1}}
  }],
  "materials": [{
    "name": "PBR",
    "pbrMetallicRoughness": {
      "baseColorTexture": {"index": 0}
    }
  }],
  "nodes": [{"name": "N"}],
  "scenes": [{"nodes": [0]}], "scene": 0
}`

func TestConvertAnimations(t *testing.T) {
	const animJSON = `{
  "asset": {"version": "2.0"},
  "buffers": [{"byteLength": 44}],
  "bufferViews": [
    {"buffer": 0, "byteOffset": 0, "byteLength": 8},
    {"buffer": 0, "byteOffset": 8, "byteLength": 24}
  ],
  "accessors": [
    {"bufferView": 0, "componentType": 5126, "count": 2, "type": "SCALAR"},
    {"bufferView": 1, "componentType": 5126, "count": 2, "type": "VEC3"}
  ],
  "animations": [{
    "name": "Move",
    "samplers": [{"input": 0, "output": 1, "interpolation": "LINEAR"}],
    "channels": [{"sampler": 0, "target": {"node": 0, "path": "translation"}}]
  }],
  "nodes": [{"name": "Animated"}],
  "scenes": [{"nodes": [0]}], "scene": 0
}`
	buf := make([]byte, 44)
	putFloat32LE(buf[0:], 0.0)
	putFloat32LE(buf[4:], 0.5)
	putFloat32LE(buf[8:], 0.0)
	putFloat32LE(buf[12:], 0.0)
	putFloat32LE(buf[16:], 0.0)
	putFloat32LE(buf[20:], 1.0)
	putFloat32LE(buf[24:], 2.0)
	putFloat32LE(buf[28:], 3.0)

	d, err := parseDoc(&fastjson.Parser{}, []byte(animJSON), buf, detect.DecodeOptions{})
	require.NoError(t, err)

	asset, err := d.convertDoc()
	require.NoError(t, err)

	require.Len(t, asset.Animations, 1)
	anim := asset.Animations[0]
	assert.Equal(t, "Move", anim.Name)
	assert.InDelta(t, 0.5, anim.Duration, 0.01, "duration=0.5s")

	require.Len(t, anim.Channels, 1)
	ch := anim.Channels[0]
	assert.Equal(t, 0, ch.NodeIndex, "target node=0")
	assert.Equal(t, ir.TargetTranslation, ch.Target, "target=translation")
	assert.Equal(t, ir.InterpolationLinear, ch.Interpolation, "interp=LINEAR")

	require.Len(t, ch.Times, 2)
	assert.InDelta(t, float32(0.0), ch.Times[0], 0.001, "time[0]=0")
	assert.InDelta(t, float32(0.5), ch.Times[1], 0.001, "time[1]=0.5")

	require.Len(t, ch.Translations, 2)
	assert.InDelta(t, float32(0.0), ch.Translations[0][0], 0.001, "T0.x=0")
	assert.InDelta(t, float32(1.0), ch.Translations[1][0], 0.001, "T1.x=1")
	assert.InDelta(t, float32(2.0), ch.Translations[1][1], 0.001, "T1.y=2")
	assert.InDelta(t, float32(3.0), ch.Translations[1][2], 0.001, "T1.z=3")
}

func TestConvertTexturesEXTTextureWebP(t *testing.T) {
	d, err := parseDoc(&fastjson.Parser{}, []byte(testTextureWebPGLTF), nil, detect.DecodeOptions{})
	require.NoError(t, err)

	images := d.convertImages()
	textures := d.convertTextures()
	require.Len(t, textures, 1)
	assert.Equal(t, "DiffuseTex", textures[0].Name)
	assert.Equal(t, ir.ImageWebP, images[textures[0].ImageIndex].Format)
	assert.Equal(t, "diffuse.webp", images[textures[0].ImageIndex].SourcePath)
}

func TestConvertNodesEXTMeshGPUInstancing(t *testing.T) {
	root := mustParse(`{
		"asset": {"version": "2.0"},
		"scene": 0,
		"scenes": [{"nodes": [0]}],
		"nodes": [{
			"name": "TreeCluster",
			"mesh": 0,
			"children": [1],
			"extensions": {
				"EXT_mesh_gpu_instancing": {
					"attributes": {
						"TRANSLATION": 0,
						"ROTATION": 1,
						"SCALE": 2
					}
				}
			}
		}, {
			"name": "Anchor"
		}],
		"accessors": [
			{"bufferView": 0, "componentType": 5126, "count": 2, "type": "VEC3"},
			{"bufferView": 1, "componentType": 5126, "count": 2, "type": "VEC4"},
			{"bufferView": 2, "componentType": 5126, "count": 2, "type": "VEC3"}
		]
	}`)

	buffer := make([]byte, 0, 80)
	for _, value := range []float32{
		1, 2, 3,
		4, 5, 6,
		0, 0, 0, 1,
		0, 0, 0.70710677, 0.70710677,
		1, 1, 1,
		2, 2, 2,
	} {
		buffer = binary.LittleEndian.AppendUint32(buffer, math.Float32bits(value))
	}

	d := &doc{
		root: root,
		bufs: bufferSet{
			buffers: [][]byte{buffer},
			views: []bufferView{
				{buffer: 0, byteOffset: 0, byteLength: 24},
				{buffer: 0, byteOffset: 24, byteLength: 32},
				{buffer: 0, byteOffset: 56, byteLength: 24},
			},
		},
	}

	translations, rotations, scales, count := d.readInstancing(root.GetArray(keyNodes)[0])
	require.Len(t, translations, 2)
	require.Len(t, rotations, 2)
	require.Len(t, scales, 2)
	require.Equal(t, 2, count)

	asset := &ir.Asset{}
	nodes, roots := d.convertNodes(asset)

	require.Len(t, roots, 1)
	assert.Equal(t, 0, roots[0])
	require.Len(t, nodes, 4)
	assert.Equal(t, ir.NoIndex, nodes[0].MeshIndex)
	assert.Equal(t, []int{2, 3, 1}, nodes[0].Children)

	assert.Equal(t, 0, nodes[2].MeshIndex)
	assert.Equal(t, [3]float32{1, 2, 3}, nodes[2].Transform.Translation)
	assert.Equal(t, [4]float32{0, 0, 0, 1}, nodes[2].Transform.Rotation)
	assert.Equal(t, [3]float32{1, 1, 1}, nodes[2].Transform.Scale)

	assert.Equal(t, 0, nodes[3].MeshIndex)
	assert.Equal(t, [3]float32{4, 5, 6}, nodes[3].Transform.Translation)
	assert.Equal(t, [4]float32{0, 0, 0.70710677, 0.70710677}, nodes[3].Transform.Rotation)
	assert.Equal(t, [3]float32{2, 2, 2}, nodes[3].Transform.Scale)
}

func TestSplitIndexSuffix(t *testing.T) {
	tests := []struct {
		input      string
		wantIdx    int
		wantSuffix string
	}{
		{"0/pbrMetallicRoughness/baseColorFactor", 0, "pbrMetallicRoughness/baseColorFactor"},
		{"5/emissiveFactor", 5, "emissiveFactor"},
		{"0", 0, ""},
		{"12", 12, ""},
		{"abc", -1, ""},
		{"", -1, ""},
		{"0/", 0, ""},
	}
	for _, tt := range tests {
		idx, suffix := splitIndexSuffix(tt.input)
		assert.Equal(t, tt.wantIdx, idx, "input=%q idx", tt.input)
		assert.Equal(t, tt.wantSuffix, suffix, "input=%q suffix", tt.input)
	}
}

func TestAnimationPointers(t *testing.T) {
	makeJSON := func(pointer, outputType string) string {
		return `{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_animation_pointer"],
  "buffers": [{"byteLength": 24}],
  "bufferViews": [
    {"buffer": 0, "byteOffset": 0, "byteLength": 8},
    {"buffer": 0, "byteOffset": 8, "byteLength": 12}
  ],
  "accessors": [
    {"bufferView": 0, "componentType": 5126, "count": 2, "type": "SCALAR"},
    {"bufferView": 1, "componentType": 5126, "count": 1, "type": "` + outputType + `"}
  ],
  "animations": [{"name": "Anim",
    "samplers": [{"input": 0, "output": 1, "interpolation": "LINEAR"}],
    "channels": [{"sampler": 0, "target": {"path": "pointer",
      "extensions": {"KHR_animation_pointer": {"pointer": "` + pointer + `"}}
    }}]
  }],
  "materials": [{"name": "M", "pbrMetallicRoughness": {}}],
  "nodes": [{"name": "N"}],
  "scenes": [{"nodes": [0]}], "scene": 0
}`
	}
	tests := []struct {
		name       string
		pointer    string
		outputType string
		wantTarget ir.ChannelTarget
	}{
		{"MaterialColor", "/materials/0/pbrMetallicRoughness/baseColorFactor", "VEC3", ir.TargetMaterialColor},
		{"MaterialScalar", "/materials/0/pbrMetallicRoughness/roughnessFactor", "SCALAR", ir.TargetMaterialScalar},
		{"NodeTranslation", "/nodes/0/translation", "VEC3", ir.TargetTranslation},
		{"NodeRotation", "/nodes/0/rotation", "VEC4", ir.TargetRotation},
		{"NodeScale", "/nodes/0/scale", "VEC3", ir.TargetScale},
		{"NodeWeights", "/nodes/0/weights", "SCALAR", ir.TargetMorphWeights},
		{"CameraYfov", "/cameras/0/perspective/yfov", "SCALAR", ir.TargetCameraFOV},
		{"LightIntensity", "/extensions/KHR_lights_punctual/lights/0/intensity", "SCALAR", ir.TargetLightIntensity},
		{"LightColor", "/extensions/KHR_lights_punctual/lights/0/color", "VEC3", ir.TargetLightColor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 24)
			putFloat32LE(buf[0:], 0.0)
			putFloat32LE(buf[4:], 1.0)
			putFloat32LE(buf[8:], 1.0)
			putFloat32LE(buf[12:], 2.0)
			putFloat32LE(buf[16:], 3.0)

			d, err := parseDoc(&fastjson.Parser{}, []byte(makeJSON(tt.pointer, tt.outputType)), buf, detect.DecodeOptions{})
			require.NoError(t, err)
			asset, err := d.convertDoc()
			require.NoError(t, err)

			require.Len(t, asset.Animations, 1)
			require.Len(t, asset.Animations[0].Channels, 1)
			ch := asset.Animations[0].Channels[0]
			assert.Equal(t, tt.wantTarget, ch.Target)
			assert.Equal(t, tt.pointer, ch.Pointer)
		})
	}
}

func putFloat32LE(b []byte, v float32) {
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
}

func TestKHRMaterialExtensions(t *testing.T) {
	src := `{
  "asset": {"version": "2.0"},
  "materials": [{
    "name": "FullPBR",
    "pbrMetallicRoughness": {"baseColorFactor": [1,1,1,1]},
    "extensions": {
      "KHR_materials_clearcoat": {
        "clearcoatFactor": 0.5,
        "clearcoatRoughnessFactor": 0.1
      },
      "KHR_materials_sheen": {
        "sheenColorFactor": [0.1, 0.2, 0.3],
        "sheenRoughnessFactor": 0.4
      },
      "KHR_materials_transmission": {
        "transmissionFactor": 0.8
      },
      "KHR_materials_volume": {
        "thicknessFactor": 2.5,
        "attenuationDistance": 10.0,
        "attenuationColor": [0.5, 0.5, 0.5]
      },
      "KHR_materials_ior": {
        "ior": 1.45
      },
      "KHR_materials_specular": {
        "specularFactor": 0.9,
        "specularColorFactor": [1.0, 0.8, 0.6]
      },
      "KHR_materials_anisotropy": {
        "anisotropyStrength": 0.7,
        "anisotropyRotation": 1.57
      },
      "KHR_materials_iridescence": {
        "iridescenceFactor": 0.6,
        "iridescenceIor": 1.5,
        "iridescenceThicknessMinimum": 200,
        "iridescenceThicknessMaximum": 500
      },
      "KHR_materials_dispersion": {
        "dispersion": 0.3
      },
      "KHR_materials_diffuse_transmission": {
        "diffuseTransmissionFactor": 0.4,
        "diffuseTransmissionColorFactor": [0.9, 0.8, 0.7]
      }
    }
  }],
  "nodes": [{"name": "N"}],
  "scenes": [{"nodes": [0]}], "scene": 0
}`
	d := &Decoder{}
	asset, err := d.Decode(bytes.NewReader([]byte(src)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, asset.Materials, 1)
	mat := asset.Materials[0]
	require.NotNil(t, mat.Clearcoat)

	assert.InDelta(t, float32(0.5), mat.Clearcoat.Factor, 0.01)
	assert.InDelta(t, float32(0.1), mat.Clearcoat.RoughnessFactor, 0.01)

	require.NotNil(t, mat.Sheen)
	sheenColor := mat.Sheen.ColorFactor
	assert.InDelta(t, float32(0.1), sheenColor[0], 0.01)
	assert.InDelta(t, float32(0.2), sheenColor[1], 0.01)
	assert.InDelta(t, float32(0.4), mat.Sheen.RoughnessFactor, 0.01)

	require.NotNil(t, mat.Transmission)
	assert.InDelta(t, float32(0.8), mat.Transmission.Factor, 0.01)

	require.NotNil(t, mat.Volume)
	assert.InDelta(t, float32(2.5), mat.Volume.ThicknessFactor, 0.01)
	assert.InDelta(t, float32(10.0), mat.Volume.AttenuationDistance, 0.01)
	attColor := mat.Volume.AttenuationColor
	assert.InDelta(t, float32(0.5), attColor[0], 0.01)

	require.NotNil(t, mat.IOR)
	assert.InDelta(t, float32(1.45), mat.IOR.IOR, 0.01)

	require.NotNil(t, mat.Specular)
	assert.InDelta(t, float32(0.9), mat.Specular.Factor, 0.01)
	specColor := mat.Specular.ColorFactor
	assert.InDelta(t, float32(1.0), specColor[0], 0.01)
	assert.InDelta(t, float32(0.8), specColor[1], 0.01)

	require.NotNil(t, mat.Anisotropy)
	assert.InDelta(t, float32(0.7), mat.Anisotropy.Strength, 0.01)
	assert.InDelta(t, float32(1.57), mat.Anisotropy.Rotation, 0.01)

	require.NotNil(t, mat.Iridescence)
	assert.InDelta(t, float32(0.6), mat.Iridescence.Factor, 0.01)
	assert.InDelta(t, float32(1.5), mat.Iridescence.IOR, 0.01)
	assert.InDelta(t, float32(200), mat.Iridescence.ThicknessMinimum, 0.1)
	assert.InDelta(t, float32(500), mat.Iridescence.ThicknessMaximum, 0.1)

	require.NotNil(t, mat.Dispersion)
	assert.InDelta(t, float32(0.3), mat.Dispersion.Dispersion, 0.01)

	require.NotNil(t, mat.DiffuseTransmission)
	assert.InDelta(t, float32(0.4), mat.DiffuseTransmission.Factor, 0.01)
	dtColor := mat.DiffuseTransmission.ColorFactor
	assert.InDelta(t, float32(0.9), dtColor[0], 0.01)
}

func TestKHRMaterialVariants(t *testing.T) {
	src := `{
  "asset": {"version": "2.0"},
  "extensions": {
    "KHR_materials_variants": {
      "variants": [{"name": "Day"}, {"name": "Night"}]
    }
  },
  "materials": [{"name": "MatA"}, {"name": "MatB"}],
  "meshes": [{
    "name": "Obj",
    "primitives": [{
      "attributes": {},
      "material": 0,
      "extensions": {
        "KHR_materials_variants": {
          "mappings": [
            {"material": 1, "variants": [0]},
            {"material": 0, "variants": [1]}
          ]
        }
      }
    }]
  }],
  "nodes": [{"name": "N", "mesh": 0}],
  "scenes": [{"nodes": [0]}], "scene": 0
}`
	d := &Decoder{}
	asset, err := d.Decode(bytes.NewReader([]byte(src)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.NotNil(t, asset.Metadata.ExtraProperties)
	assert.Equal(t, "Day,Night", asset.Metadata.ExtraProperties["materialVariants"])
	assert.Equal(t, "0:1,1:0", asset.Metadata.ExtraProperties["variantMappings_mesh0_prim0"])
}

func TestMSFTLOD(t *testing.T) {
	src := `{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["MSFT_lod"],
  "nodes": [
    {
      "name": "LOD0",
      "mesh": 0,
      "extensions": {"MSFT_lod": {"ids": [1, 2]}},
      "extras": {"MSFT_screencoverage": [0.75, 0.35, 0.10]}
    },
    {"name": "LOD1", "mesh": 1},
    {"name": "LOD2", "mesh": 2}
  ],
  "meshes": [
    {"primitives": [{"attributes": {}}]},
    {"primitives": [{"attributes": {}}]},
    {"primitives": [{"attributes": {}}]}
  ],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`
	d := &Decoder{}
	asset, err := d.Decode(bytes.NewReader([]byte(src)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, asset.LODGroups, 1)
	require.Len(t, asset.LODGroups[0].Levels, 3)
	assert.Equal(t, 0, asset.LODGroups[0].Levels[0].NodeIndex)
	assert.InDelta(t, 0.75, asset.LODGroups[0].Levels[0].Threshold, 0.001)
	assert.Equal(t, 1, asset.LODGroups[0].Levels[1].NodeIndex)
	assert.InDelta(t, 0.35, asset.LODGroups[0].Levels[1].Threshold, 0.001)
	assert.Equal(t, 2, asset.LODGroups[0].Levels[2].NodeIndex)
	assert.InDelta(t, 0.10, asset.LODGroups[0].Levels[2].Threshold, 0.001)

	require.Len(t, asset.Nodes, 3)
	assert.Equal(t, 0, asset.Nodes[0].LODGroupIndex)
	assert.Equal(t, 0, asset.Nodes[1].LODGroupIndex)
	assert.Equal(t, 0, asset.Nodes[2].LODGroupIndex)
	assert.Equal(t, []int{0}, asset.RootNodes)
}

func TestAccessorCasts(t *testing.T) {
	src := `{
  "asset": {"version": "2.0"},
  "buffers": [{"byteLength": 64}],
  "bufferViews": [{"buffer": 0, "byteOffset": 0, "byteLength": 64}],
  "accessors": [
    {"bufferView": 0, "byteOffset": 0, "componentType": 5126, "count": 1, "type": "VEC2"},
    {"bufferView": 0, "byteOffset": 8, "componentType": 5126, "count": 1, "type": "VEC3"},
    {"bufferView": 0, "byteOffset": 20, "componentType": 5126, "count": 1, "type": "VEC4"},
    {"bufferView": 0, "byteOffset": 36, "componentType": 5126, "count": 2, "type": "SCALAR"}
  ],
  "meshes": [{"primitives": [{"attributes": {"V2": 0, "V3": 1, "V4": 2, "SC": 3}}]}],
  "nodes": [{"mesh": 0}], "scenes": [{"nodes": [0]}], "scene": 0
}`
	buf := make([]byte, 64)
	putFloat32LE(buf[0:], 1)
	putFloat32LE(buf[4:], 2)

	putFloat32LE(buf[8:], 3)
	putFloat32LE(buf[12:], 4)
	putFloat32LE(buf[16:], 5)

	putFloat32LE(buf[20:], 6)
	putFloat32LE(buf[24:], 7)
	putFloat32LE(buf[28:], 8)
	putFloat32LE(buf[32:], 9)

	putFloat32LE(buf[36:], 10)
	putFloat32LE(buf[40:], 11)

	d, err := parseDoc(&fastjson.Parser{}, []byte(src), buf, detect.DecodeOptions{})
	require.NoError(t, err)
	asset, err := d.convertDoc()
	require.NoError(t, err)

	require.Len(t, asset.Meshes, 1)
	// Just decoding the document invokes the accessor readers for standard attributes.
	// We'll test the actual internal cast functions directly since standard meshes only use predefined attributes.
	v2 := d.bufs.readVec2s(d.getAccessor(0))
	assert.Equal(t, [][2]float32{{1, 2}}, v2)

	v3 := d.bufs.readVec3s(d.getAccessor(1))
	assert.Equal(t, [][3]float32{{3, 4, 5}}, v3)

	v4 := d.bufs.readVec4s(d.getAccessor(2))
	assert.Equal(t, [][4]float32{{6, 7, 8, 9}}, v4)

	f32 := d.bufs.readFloat32s(d.getAccessor(3))
	assert.Equal(t, []float32{10, 11}, f32)
}

func TestPBRSpecGloss(t *testing.T) {
	src := `{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_materials_pbrSpecularGlossiness"],
  "materials": [{
    "name": "SpecGloss",
    "extensions": {
      "KHR_materials_pbrSpecularGlossiness": {
        "diffuseFactor": [1.0, 0.5, 0.0, 1.0],
        "specularFactor": [0.2, 0.2, 0.2],
        "glossinessFactor": 0.8
      }
    }
  }],
  "nodes": [{"name": "N"}], "scenes": [{"nodes": [0]}], "scene": 0
}`
	d := &Decoder{}
	asset, err := d.Decode(bytes.NewReader([]byte(src)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, asset.Materials, 1)
	mat := asset.Materials[0]
	assert.Equal(t, "SpecGloss", mat.Name)

	require.NotNil(t, mat.SpecularGlossiness)
	assert.InDelta(t, float32(0.8), mat.SpecularGlossiness.GlossinessFactor, 0.01)

	diffuse := mat.SpecularGlossiness.DiffuseFactor
	assert.InDelta(t, float32(1.0), diffuse[0], 0.01)
	assert.InDelta(t, float32(0.5), diffuse[1], 0.01)

	spec := mat.SpecularGlossiness.SpecularFactor
	assert.InDelta(t, float32(0.2), spec[0], 0.01)
}

func makeBufferSet(data []byte, stride int) *bufferSet {
	return &bufferSet{
		buffers: [][]byte{data},
		views: []bufferView{{
			buffer:     0,
			byteOffset: 0,
			byteLength: len(data),
			byteStride: stride,
		}},
	}
}

func TestReadMat4s(t *testing.T) {
	// 1 mat4 = 16 float32s = 64 bytes
	buf := make([]byte, 64)
	for i := range 16 {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(float32(i+1)))
	}
	bs := makeBufferSet(buf, 64)
	a := accessor{bufferView: 0, componentType: compFloat, count: 1, elemCount: elemMat4}
	mats := bs.readMat4s(a)
	require.Len(t, mats, 1)
	assert.InDelta(t, float32(1), mats[0][0], 0.001)
	assert.InDelta(t, float32(16), mats[0][15], 0.001)
}

func TestReadJoints(t *testing.T) {
	// 1 joint vec4 = 4 uint8s
	buf := []byte{10, 20, 30, 40}
	bs := makeBufferSet(buf, 4)
	a := accessor{bufferView: 0, componentType: compUByte, count: 1, elemCount: elemVec4}
	joints := bs.readJoints(a)
	require.Len(t, joints, 1)
	assert.Equal(t, uint16(10), joints[0][0])
	assert.Equal(t, uint16(40), joints[0][3])

	// UShort variant
	buf2 := make([]byte, 8)
	binary.LittleEndian.PutUint16(buf2[0:], 100)
	binary.LittleEndian.PutUint16(buf2[2:], 200)
	binary.LittleEndian.PutUint16(buf2[4:], 300)
	binary.LittleEndian.PutUint16(buf2[6:], 400)
	bs2 := makeBufferSet(buf2, 8)
	a2 := accessor{bufferView: 0, componentType: compUShort, count: 1, elemCount: elemVec4}
	joints2 := bs2.readJoints(a2)
	require.Len(t, joints2, 1)
	assert.Equal(t, uint16(100), joints2[0][0])
	assert.Equal(t, uint16(400), joints2[0][3])
}

func TestReadColors(t *testing.T) {
	// VEC4 UByte colors: [255, 128, 0, 255]
	buf := []byte{255, 128, 0, 255}
	bs := makeBufferSet(buf, 4)
	a := accessor{bufferView: 0, componentType: compUByte, count: 1, elemCount: elemVec4}
	colors := bs.readColors(a)
	require.Len(t, colors, 1)
	assert.InDelta(t, 1.0, colors[0][0], 0.01)         // 255/255
	assert.InDelta(t, 128.0/255.0, colors[0][1], 0.01) // 128/255
	assert.InDelta(t, 0.0, colors[0][2], 0.01)         // 0/255
	assert.InDelta(t, 1.0, colors[0][3], 0.01)         // 255/255

	// VEC3 float colors: alpha should be set to 1.0
	buf3 := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf3[0:], math.Float32bits(0.5))
	binary.LittleEndian.PutUint32(buf3[4:], math.Float32bits(0.6))
	binary.LittleEndian.PutUint32(buf3[8:], math.Float32bits(0.7))
	bs3 := makeBufferSet(buf3, 12)
	a3 := accessor{bufferView: 0, componentType: compFloat, count: 1, elemCount: elemVec3}
	colors3 := bs3.readColors(a3)
	require.Len(t, colors3, 1)
	assert.InDelta(t, 0.5, colors3[0][0], 0.01)
	assert.InDelta(t, 1.0, colors3[0][3], 0.01) // alpha defaults to 1.0 for VEC3
}

func TestReadComponentNorm(t *testing.T) {
	// UByte normalized
	assert.InDelta(t, 1.0, readComponentNorm([]byte{255}, compUByte), 0.01)
	assert.InDelta(t, 0.5, readComponentNorm([]byte{128}, compUByte), 0.02)

	// Byte normalized (signed)
	assert.InDelta(t, 1.0, readComponentNorm([]byte{127}, compByte), 0.01)
	assert.InDelta(t, -1.0, readComponentNorm([]byte{129}, compByte), 0.02) // -127/127

	// UShort normalized
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, 65535)
	assert.InDelta(t, 1.0, readComponentNorm(buf, compUShort), 0.01)
}

func TestComponentSize(t *testing.T) {
	assert.Equal(t, 1, componentSize(compByte))
	assert.Equal(t, 1, componentSize(compUByte))
	assert.Equal(t, 2, componentSize(compShort))
	assert.Equal(t, 2, componentSize(compUShort))
	assert.Equal(t, 4, componentSize(compUInt))
	assert.Equal(t, 4, componentSize(compFloat))
	assert.Equal(t, 0, componentSize(9999))
}

func TestTypeElemCount(t *testing.T) {
	assert.Equal(t, 1, typeElemCount("SCALAR"))
	assert.Equal(t, 2, typeElemCount("VEC2"))
	assert.Equal(t, 3, typeElemCount("VEC3"))
	assert.Equal(t, 4, typeElemCount("VEC4"))
	assert.Equal(t, 4, typeElemCount("MAT2"))
	assert.Equal(t, 9, typeElemCount("MAT3"))
	assert.Equal(t, 16, typeElemCount("MAT4"))
	assert.Equal(t, 0, typeElemCount("UNKNOWN"))
}

func TestReadComponent(t *testing.T) {
	// Float
	fb := make([]byte, 4)
	binary.LittleEndian.PutUint32(fb, math.Float32bits(42.5))
	assert.InDelta(t, 42.5, readComponent(fb, compFloat), 0.001)

	// Byte (signed)
	assert.InDelta(t, -1.0, readComponent([]byte{0xFF}, compByte), 0.001)

	// UByte
	assert.InDelta(t, 200.0, readComponent([]byte{200}, compUByte), 0.001)

	// Short (signed)
	sb := make([]byte, 2)
	v := int16(-100)
	binary.LittleEndian.PutUint16(sb, uint16(v))
	assert.InDelta(t, -100.0, readComponent(sb, compShort), 0.001)

	// UShort
	binary.LittleEndian.PutUint16(sb, 1000)
	assert.InDelta(t, 1000.0, readComponent(sb, compUShort), 0.001)

	// UInt
	ub := make([]byte, 4)
	binary.LittleEndian.PutUint32(ub, 50000)
	assert.InDelta(t, 50000.0, readComponent(ub, compUInt), 0.001)

	// Unknown
	assert.InDelta(t, 0.0, readComponent(fb, 9999), 0.001)
}

func TestReadIndices(t *testing.T) {
	// UByte
	buf1 := []byte{1, 2, 3}
	bs1 := makeBufferSet(buf1, 1)
	a1 := accessor{bufferView: 0, componentType: compUByte, count: 3, elemCount: 1}
	idx1 := bs1.readIndices(a1)
	require.Len(t, idx1, 3)
	assert.Equal(t, uint32(1), idx1[0])
	assert.Equal(t, uint32(3), idx1[2])

	// UShort
	buf2 := make([]byte, 6)
	binary.LittleEndian.PutUint16(buf2[0:], 10)
	binary.LittleEndian.PutUint16(buf2[2:], 20)
	binary.LittleEndian.PutUint16(buf2[4:], 30)
	bs2 := makeBufferSet(buf2, 2)
	a2 := accessor{bufferView: 0, componentType: compUShort, count: 3, elemCount: 1}
	idx2 := bs2.readIndices(a2)
	require.Len(t, idx2, 3)
	assert.Equal(t, uint32(10), idx2[0])
	assert.Equal(t, uint32(30), idx2[2])

	// UInt
	buf3 := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf3[0:], 100)
	binary.LittleEndian.PutUint32(buf3[4:], 200)
	bs3 := makeBufferSet(buf3, 4)
	a3 := accessor{bufferView: 0, componentType: compUInt, count: 2, elemCount: 1}
	idx3 := bs3.readIndices(a3)
	require.Len(t, idx3, 2)
	assert.Equal(t, uint32(100), idx3[0])
	assert.Equal(t, uint32(200), idx3[1])
}

func TestApplySparseVec3s(t *testing.T) {
	// Main buffer: 2 vec3s, both zero
	mainBuf := make([]byte, 24) // 2 * 3 * 4 bytes

	// Sparse indices: 1 index pointing to vertex 1
	idxBuf := []byte{1} // UByte index = 1

	// Sparse values: 1 vec3 = [10.0, 20.0, 30.0]
	valBuf := make([]byte, 12)
	binary.LittleEndian.PutUint32(valBuf[0:], math.Float32bits(10.0))
	binary.LittleEndian.PutUint32(valBuf[4:], math.Float32bits(20.0))
	binary.LittleEndian.PutUint32(valBuf[8:], math.Float32bits(30.0))

	bs := &bufferSet{
		buffers: [][]byte{mainBuf, idxBuf, valBuf},
		views: []bufferView{
			{buffer: 0, byteOffset: 0, byteLength: 24, byteStride: 12},
			{buffer: 1, byteOffset: 0, byteLength: 1},
			{buffer: 2, byteOffset: 0, byteLength: 12},
		},
	}
	a := accessor{
		bufferView:    0,
		componentType: compFloat,
		count:         2,
		elemCount:     elemVec3,
		sparseCount:   1,
		sparseIdxBV:   1,
		sparseIdxComp: compUByte,
		sparseValBV:   2,
	}
	result := bs.readVec3s(a)
	require.Len(t, result, 2)
	assert.InDelta(t, 0.0, result[0][0], 0.001)
	assert.InDelta(t, 10.0, result[1][0], 0.001)
	assert.InDelta(t, 20.0, result[1][1], 0.001)
	assert.InDelta(t, 30.0, result[1][2], 0.001)
}

func TestResolveBytes_OOB(t *testing.T) {
	bs := makeBufferSet([]byte{1, 2, 3}, 0)
	_, _, err := bs.resolveBytes(accessor{bufferView: 99, componentType: compFloat, count: 1, elemCount: 1})
	assert.Error(t, err)
}

func TestCastVec3Slice(t *testing.T) {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(2.0))
	binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(3.0))
	out := castVec3Slice(buf, 1)
	require.Len(t, out, 1)
	assert.InDelta(t, 1.0, out[0][0], 0.001)

	assert.Nil(t, castVec3Slice(buf[:4], 1))
}

func TestCastVec4Slice(t *testing.T) {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(2.0))
	binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(3.0))
	binary.LittleEndian.PutUint32(buf[12:], math.Float32bits(4.0))
	out := castVec4Slice(buf, 1)
	require.Len(t, out, 1)
	assert.InDelta(t, 4.0, out[0][3], 0.001)

	assert.Nil(t, castVec4Slice(buf[:8], 1))
}

func TestGLTFDecodeAll(t *testing.T) {
	tests := []struct {
		name    string
		inputFn func() ([]byte, error)
		check   func(t *testing.T, asset *ir.Asset)
	}{
		{
			"MinimalGLTF",
			func() ([]byte, error) { return []byte(testMinimalGLTF), nil },
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.FormatGLTF, asset.Metadata.SourceFormat)
				assert.Equal(t, "2.0", asset.Metadata.SourceVersion)
				assert.Equal(t, ir.YUp, asset.UpAxis)
				require.Len(t, asset.Meshes, 1)
				assert.Equal(t, "Tri", asset.Meshes[0].Name)
				require.Len(t, asset.Materials, 1)
				mat := asset.Materials[0]
				assert.Equal(t, "Red", mat.Name)
				assert.InDelta(t, float32(1.0), mat.BaseColorFactor[0], 0.01)
				require.Len(t, asset.Nodes, 1)
				assert.Equal(t, "Root", asset.Nodes[0].Name)
				assert.Equal(t, 0, asset.Nodes[0].MeshIndex)
				assert.Equal(t, []int{0}, asset.RootNodes)
			},
		},
		{
			"GLTFJSON",
			func() ([]byte, error) { return []byte(`{"asset": {"version": "2.0"}}`), nil },
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.FormatGLTF, asset.Metadata.SourceFormat)
			},
		},
		{
			"BoxGLB",
			func() ([]byte, error) { return os.ReadFile("testdata/Box.glb") },
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.FormatGLTF, asset.Metadata.SourceFormat)
				assert.Equal(t, "2.0", asset.Metadata.SourceVersion)
				assert.Equal(t, ir.YUp, asset.UpAxis)
				require.Len(t, asset.Meshes, 1)
				assert.Equal(t, "Mesh", asset.Meshes[0].Name)
				require.Len(t, asset.Meshes[0].Primitives, 1)
				prim := asset.Meshes[0].Primitives[0]
				assert.Equal(t, ir.Triangles, prim.Mode)
				assert.Equal(t, 24, prim.Data.VertexCount)
				assert.Len(t, prim.Data.Positions, 24)
				assert.Len(t, prim.Data.Normals, 24)
				assert.Len(t, prim.Data.Indices, 36)
				assert.InDelta(t, float32(-0.5), prim.Data.Positions[0][0], 0.001)
				assert.InDelta(t, float32(-0.5), prim.Data.Positions[0][1], 0.001)
				assert.InDelta(t, float32(0.5), prim.Data.Positions[0][2], 0.001)
				assert.InDelta(t, float32(0), prim.Data.Normals[0][0], 0.001)
				assert.InDelta(t, float32(0), prim.Data.Normals[0][1], 0.001)
				assert.InDelta(t, float32(1), prim.Data.Normals[0][2], 0.001)
				require.Len(t, asset.Materials, 1)
				mat := asset.Materials[0]
				assert.Equal(t, "Red", mat.Name)
				assert.InDelta(t, float32(0.8), mat.BaseColorFactor[0], 0.001)
				assert.InDelta(t, float32(1.0), mat.RoughnessFactor, 0.001)
			},
		},
		{
			"Cameras",
			func() ([]byte, error) { return []byte(testCamerasGLTF), nil },
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Cameras, 2)
				cam0 := asset.Cameras[0]
				assert.Equal(t, "PerspCam", cam0.Name)
				require.NotNil(t, cam0.Perspective)
				assert.Nil(t, cam0.Orthographic)
				assert.InDelta(t, float32(0.785), cam0.Perspective.FOV, 0.001)
				assert.InDelta(t, float32(0.1), cam0.Perspective.Near, 0.001)
				assert.InDelta(t, float32(100.0), cam0.Perspective.Far, 0.01)
				assert.InDelta(t, float32(1.5), cam0.Perspective.Aspect, 0.001)
				cam1 := asset.Cameras[1]
				assert.Equal(t, "OrthoCam", cam1.Name)
				require.NotNil(t, cam1.Orthographic)
				assert.Nil(t, cam1.Perspective)
				assert.InDelta(t, float32(5.0), cam1.Orthographic.XMag, 0.001)
				assert.InDelta(t, float32(5.0), cam1.Orthographic.YMag, 0.001)
				assert.InDelta(t, float32(0.01), cam1.Orthographic.Near, 0.001)
				assert.InDelta(t, float32(50.0), cam1.Orthographic.Far, 0.01)
				require.Len(t, asset.Nodes, 2)
				assert.Equal(t, 0, asset.Nodes[0].CameraIndex)
				assert.Equal(t, 1, asset.Nodes[1].CameraIndex)
			},
		},
		{
			"Lights",
			func() ([]byte, error) { return []byte(testLightsGLTF), nil },
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Lights, 3)
				sun := asset.Lights[0]
				assert.Equal(t, "Sun", sun.Name)
				require.NotNil(t, sun.Directional)
				assert.InDelta(t, float32(5.0), sun.Intensity, 0.01)
				assert.InDelta(t, float32(1.0), sun.Color[0], 0.01)
				bulb := asset.Lights[1]
				assert.Equal(t, "Bulb", bulb.Name)
				require.NotNil(t, bulb.Point)
				assert.InDelta(t, float32(100.0), bulb.Intensity, 0.01)
				assert.InDelta(t, float32(25.0), bulb.Point.Range, 0.01)
				spot := asset.Lights[2]
				assert.Equal(t, "Spot1", spot.Name)
				require.NotNil(t, spot.Spot)
				assert.InDelta(t, float32(200.0), spot.Intensity, 0.01)
				assert.InDelta(t, float32(10.0), spot.Spot.Range, 0.01)
				assert.InDelta(t, float32(0.1), spot.Spot.InnerConeAngle, 0.01)
				require.Len(t, asset.Nodes, 3)
				assert.Equal(t, 0, asset.Nodes[0].LightIndex)
				assert.Equal(t, 1, asset.Nodes[1].LightIndex)
				assert.Equal(t, 2, asset.Nodes[2].LightIndex)
			},
		},
		{
			"Textures",
			func() ([]byte, error) { return []byte(testTexturesGLTF), nil },
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Textures, 2)
				tex0 := asset.Textures[0]
				assert.Equal(t, "DiffuseTex", tex0.Name)
				assert.Equal(t, ir.ImagePNG, asset.Images[tex0.ImageIndex].Format)
				assert.Equal(t, "diffuse.png", asset.Images[tex0.ImageIndex].SourcePath)
				assert.Equal(t, ir.WrapClamp, tex0.WrapS)
				assert.Equal(t, ir.WrapMirror, tex0.WrapT)
				assert.Equal(t, ir.FilterNearest, tex0.MinFilter)
				assert.Equal(t, ir.FilterLinear, tex0.MagFilter)
				tex1 := asset.Textures[1]
				assert.Equal(t, "NormalTex", tex1.Name)
				assert.Equal(t, ir.ImageJPEG, asset.Images[tex1.ImageIndex].Format)
				assert.Equal(t, "normal.jpg", asset.Images[tex1.ImageIndex].SourcePath)
				assert.Equal(t, ir.WrapRepeat, tex1.WrapS)
				assert.Equal(t, ir.WrapRepeat, tex1.WrapT)
				require.Len(t, asset.Materials, 1)
				mat := asset.Materials[0]
				assert.Equal(t, "PBR", mat.Name)
				require.NotNil(t, mat.BaseColorTexture)
				assert.Equal(t, 0, mat.BaseColorTexture.TextureIndex)
				require.NotNil(t, mat.MetallicTexture)
				assert.Equal(t, 1, mat.MetallicTexture.TextureIndex)
				require.NotNil(t, mat.NormalTexture)
				assert.Equal(t, 0, mat.NormalTexture.TextureIndex)
				assert.InDelta(t, float32(0.8), mat.NormalScale, 0.01)
				require.NotNil(t, mat.OcclusionTexture)
				assert.InDelta(t, float32(0.5), mat.OcclusionStrength, 0.01)
				require.NotNil(t, mat.EmissiveTexture)
				assert.InDelta(t, float32(0.2), mat.EmissiveFactor[0], 0.01)
			},
		},
	}

	d := &Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.inputFn()
			require.NoError(t, err)

			asset, err := d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
			require.NoError(t, err)

			tt.check(t, asset)
		})
	}
}
