package fbx

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
)

func TestConvertTexture(t *testing.T) {
	node := fbxNode{
		name: "Texture",
		properties: []fbxProp{
			{intVal: 100},
			{strVal: "Texture::Diffuse"},
		},
		children: []fbxNode{
			{name: texRelFilename, properties: []fbxProp{{strVal: "textures/diffuse.png"}}},
			{name: texFileName, properties: []fbxProp{{strVal: "C:/full/diff.png"}}},
		},
	}
	image, tex := convertTexture(&node)
	assert.NotNil(t, image)
	assert.NotNil(t, tex)
	assert.Equal(t, "textures/diffuse.png", image.SourcePath)

	nodeNoRel := fbxNode{
		name: "Texture",
		properties: []fbxProp{
			{intVal: 101},
			{strVal: "Texture::Normal"},
		},
		children: []fbxNode{
			{name: texFileName, properties: []fbxProp{{strVal: "C:/full/normal.png"}}},
		},
	}
	image2, tex2 := convertTexture(&nodeNoRel)
	assert.NotNil(t, image2)
	assert.NotNil(t, tex2)
	assert.Equal(t, "C:/full/normal.png", image2.SourcePath)

	nodeEmpty := fbxNode{name: "Texture"}
	image3, tex3 := convertTexture(&nodeEmpty)
	assert.Nil(t, image3)
	assert.Nil(t, tex3)
}

func TestConvertFBXCamera(t *testing.T) {
	// Build P70 with FOV, Near, Far
	p70 := fbxNode{
		name: nodeProperties70,
		children: []fbxNode{
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propFOV}, {}, {}, {},
					{floatVal: 60.0},
				},
			},
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propNearPlane}, {}, {}, {},
					{floatVal: 0.5},
				},
			},
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propFarPlane}, {}, {}, {},
					{floatVal: 500.0},
				},
			},
		},
	}
	node := fbxNode{
		name: "NodeAttribute",
		properties: []fbxProp{
			{intVal: 10},
			{strVal: "NodeAttribute::MyCam"},
		},
		children: []fbxNode{p70},
	}
	cam := convertFBXCamera(&node)
	assert.NotNil(t, cam)
	assert.InDelta(t, 60.0*degToRad, cam.Perspective.FOV, 0.001)
	assert.InDelta(t, 0.5, cam.Perspective.Near, 0.001)
	assert.InDelta(t, 500.0, cam.Perspective.Far, 0.001)
}

func TestConvertFBXLight(t *testing.T) {
	p70 := fbxNode{
		name: nodeProperties70,
		children: []fbxNode{
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propLightType}, {}, {}, {},
					{intVal: 2},
				},
			},
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propIntensity}, {}, {}, {},
					{floatVal: 200.0},
				},
			},
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propInnerAngle}, {}, {}, {},
					{floatVal: 30.0},
				},
			},
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propOuterAngle}, {}, {}, {},
					{floatVal: 45.0},
				},
			},
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: propLightColor}, {}, {}, {},
					{floatVal: 1.0}, {floatVal: 0.5}, {floatVal: 0.25},
				},
			},
		},
	}
	node := fbxNode{
		name: "NodeAttribute",
		properties: []fbxProp{
			{intVal: 20},
			{strVal: "NodeAttribute::MyLight"},
		},
		children: []fbxNode{p70},
	}
	light := convertFBXLight(&node)
	assert.NotNil(t, light)
	assert.NotNil(t, light.Spot)
	assert.Nil(t, light.Point)
	assert.InDelta(t, 200.0/100.0, light.Intensity, 0.001)
	assert.InDelta(t, 30.0*degToRad, light.Spot.InnerConeAngle, 0.001)
	assert.InDelta(t, 45.0*degToRad, light.Spot.OuterConeAngle, 0.001)
	assert.InDelta(t, 1.0, light.Color[0], 0.001)
	assert.InDelta(t, 0.5, light.Color[1], 0.001)
	assert.InDelta(t, 0.25, light.Color[2], 0.001)
}

func TestBuildASCIINodes(t *testing.T) {
	meshes := []*ir.Mesh{
		{Name: "Mesh0"},
		{Name: "Mesh1"},
	}
	nodes := buildASCIINodes(meshes)
	assert.Len(t, nodes, 2)
	assert.Equal(t, "Mesh0", nodes[0].Name)
	assert.Equal(t, 0, nodes[0].MeshIndex)
	assert.Equal(t, "Mesh1", nodes[1].Name)
	assert.Equal(t, 1, nodes[1].MeshIndex)
}

func TestAppendCollisionMeshes(t *testing.T) {
	asset := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "UCX_Wall", MeshIndex: 2, IsCollision: true},
			{Name: "UBX_Crate", MeshIndex: 3, IsCollision: true},
			{Name: "USP_Ball", MeshIndex: 4, IsCollision: true},
			{Name: "UCP_Capsule", MeshIndex: 5, IsCollision: true},
			{Name: "VisibleMesh", MeshIndex: 6},
		},
	}

	appendCollisionMeshes(asset)

	assert.Len(t, asset.CollisionMeshes, 4)
	assert.Equal(t, ir.CollisionTypeConvexHull, asset.CollisionMeshes[0].Type)
	assert.Equal(t, 2, asset.CollisionMeshes[0].MeshIndex)
	assert.Equal(t, 0, asset.CollisionMeshes[0].NodeIndex)
	assert.Equal(t, ir.CollisionTypeBox, asset.CollisionMeshes[1].Type)
	assert.Equal(t, ir.CollisionTypeSphere, asset.CollisionMeshes[2].Type)
	assert.Equal(t, ir.CollisionTypeCapsule, asset.CollisionMeshes[3].Type)
}

func TestSafeSliceParsers(t *testing.T) {
	// readI32SliceSafe: short data
	i32s := readI32SliceSafe([]byte{1, 0, 0, 0}, 3)
	assert.Len(t, i32s, 3)
	assert.Equal(t, int32(1), i32s[0])
	assert.Equal(t, int32(0), i32s[1]) // short

	// readI64SliceSafe: short data
	buf8 := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf8, 42)
	i64s := readI64SliceSafe(buf8, 2)
	assert.Len(t, i64s, 2)
	assert.Equal(t, int64(42), i64s[0])
	assert.Equal(t, int64(0), i64s[1]) // short

	// readF32SliceSafe: short data
	buf4 := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf4, math.Float32bits(3.14))
	f32s := readF32SliceSafe(buf4, 2)
	assert.Len(t, f32s, 2)
	assert.InDelta(t, 3.14, f32s[0], 0.001)
	assert.Equal(t, float32(0), f32s[1])

	// readF64SliceSafe: short data
	buf64 := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf64, math.Float64bits(2.718))
	f64s := readF64SliceSafe(buf64, 2)
	assert.Len(t, f64s, 2)
	assert.InDelta(t, 2.718, f64s[0], 0.001)
	assert.Equal(t, float64(0), f64s[1])
}

func TestExtractVideoContent(t *testing.T) {
	node := fbxNode{
		name: "Video",
		children: []fbxNode{
			{name: videoContent, properties: []fbxProp{{rawVal: []byte{0xFF, 0xD8, 0xFF}}}},
		},
	}
	data := extractVideoContent(&node)
	assert.Equal(t, []byte{0xFF, 0xD8, 0xFF}, data)

	nodeEmpty := fbxNode{name: "Video"}
	assert.Nil(t, extractVideoContent(&nodeEmpty))
}

func TestExpandUVs(t *testing.T) {
	le := &layerElement{
		data:    []float64{0.0, 0.0, 1.0, 0.0, 1.0, 1.0},
		mapMode: mapByPolygonVtx,
	}
	polyIdx := []int32{0, 1, -3}
	uvs := expandUVs(le, polyIdx)
	assert.Len(t, uvs, 3)
	assert.InDelta(t, 0.0, uvs[0][0], 0.001)
	assert.InDelta(t, 1.0, uvs[1][0], 0.001)
	assert.InDelta(t, 1.0, uvs[2][0], 0.001)
	assert.InDelta(t, 1.0, uvs[2][1], 0.001)
}

func TestApplyBinormalHandedness(t *testing.T) {
	tangents := [][4]float32{
		{1, 0, 0, 1},
		{1, 0, 0, 1},
	}
	normals := [][3]float32{
		{0, 0, 1},
		{0, 0, 1},
	}
	binormals := [][3]float32{
		{0, 1, 0},
		{0, -1, 0},
	}

	applyBinormalHandedness(tangents, normals, binormals)

	assert.Equal(t, float32(1), tangents[0][3])
	assert.Equal(t, float32(-1), tangents[1][3])
}

func TestZlibDecompress(t *testing.T) {
	// Compress some data first
	input := []byte("hello world, this is a test of zlib decompression in FBX")
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err := w.Write(input)
	assert.NoError(t, err)
	assert.NoError(t, w.Close())

	decompressed, err := zlibDecompressSized(buf.Bytes(), 0)
	assert.NoError(t, err)
	assert.Equal(t, input, decompressed)
}

func TestParseConnections(t *testing.T) {
	node := fbxNode{
		name: "Connections",
		children: []fbxNode{
			{name: "C", properties: []fbxProp{
				{strVal: connOO},
				{intVal: 100},
				{intVal: 200},
			}},
			{name: "C", properties: []fbxProp{
				{strVal: connOP},
				{intVal: 300},
				{intVal: 400},
				{strVal: "DiffuseColor"},
			}},
		},
	}
	conns := parseConnections(&node)
	assert.Len(t, conns, 2)
	assert.Equal(t, int64(100), conns[0].childID)
	assert.Equal(t, int64(200), conns[0].parentID)
	assert.Equal(t, int64(300), conns[1].childID)
	assert.Equal(t, "DiffuseColor", conns[1].propName)
}

func TestCastI64Slice(t *testing.T) {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[0:], 42)
	binary.LittleEndian.PutUint64(buf[8:], 99)
	result := castI64Slice(buf, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(42), result[0])
	assert.Equal(t, int64(99), result[1])
}

func TestCastF32Slice(t *testing.T) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(3.14))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(2.71))
	result := castF32Slice(buf, 2)
	assert.Len(t, result, 2)
	assert.InDelta(t, 3.14, result[0], 0.001)
	assert.InDelta(t, 2.71, result[1], 0.001)
}

func TestAnimTargetToIR(t *testing.T) {
	assert.Equal(t, ir.TargetTranslation, animTargetToIR(animTargetT))
	assert.Equal(t, ir.TargetRotation, animTargetToIR(animTargetR))
	assert.Equal(t, ir.TargetScale, animTargetToIR(animTargetS))
	assert.Equal(t, ir.TargetTranslation, animTargetToIR("Unknown"))
}

func TestExtractASCIIVersion(t *testing.T) {
	data := []byte("; FBX 7.4.0 project file\n; more stuff")
	ver := extractASCIIVersion(data)
	assert.Equal(t, "7400", ver)

	data2 := []byte("; FBX 6.1 project file\n")
	ver2 := extractASCIIVersion(data2)
	assert.Equal(t, "6100", ver2)

	data3 := []byte("no fbx header")
	ver3 := extractASCIIVersion(data3)
	assert.Equal(t, "", ver3)
}

func TestConvertMaterial(t *testing.T) {
	p70 := fbxNode{
		name: nodeProperties70,
		children: []fbxNode{
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: fbxPropDiffuseColor}, {}, {}, {},
					{floatVal: 0.8}, {floatVal: 0.2}, {floatVal: 0.1},
				},
			},
			{
				name: nodeP,
				properties: []fbxProp{
					{strVal: fbxPropEmissiveColor}, {}, {}, {},
					{floatVal: 0.5}, {floatVal: 0.5}, {floatVal: 0.0},
				},
			},
		},
	}
	node := fbxNode{
		name: "Material",
		properties: []fbxProp{
			{intVal: 50},
			{strVal: "Material::MyMat"},
		},
		children: []fbxNode{p70},
	}
	mat := convertMaterial(&node)
	assert.NotNil(t, mat)
	assert.InDelta(t, 0.8, mat.BaseColorFactor[0], 0.001)
	assert.InDelta(t, 0.2, mat.BaseColorFactor[1], 0.001)
	assert.InDelta(t, 0.5, mat.EmissiveFactor[0], 0.001)
}

func TestFbxFlagsToInterp(t *testing.T) {
	assert.Equal(t, ir.InterpolationStep, fbxFlagsToInterp(0x2))
	assert.Equal(t, ir.InterpolationCubicSpline, fbxFlagsToInterp(0x8))
	assert.Equal(t, ir.InterpolationCubicSpline, fbxFlagsToInterp(0xA))
	assert.Equal(t, ir.InterpolationLinear, fbxFlagsToInterp(0x0))
}

func TestAppendUnique(t *testing.T) {
	s := []string{"a", "b"}
	s2 := appendUnique(s, "b")
	assert.Len(t, s2, 2)
	s3 := appendUnique(s, "c")
	assert.Len(t, s3, 3)
}

func TestExtractASCIIIDB(t *testing.T) {
	line := []byte(`Geometry: 123, "Geometry::Mesh", "Mesh" {`)
	id := extractASCIIIDB(line)
	assert.Equal(t, int64(123), id)

	assert.Equal(t, int64(0), extractASCIIIDB([]byte("nocolon")))
}

func TestExtractSubType(t *testing.T) {
	node := fbxNode{
		properties: []fbxProp{{}, {}, {strVal: "Cluster"}},
	}
	assert.Equal(t, "Cluster", extractSubType(&node))

	node2 := fbxNode{properties: []fbxProp{{}}}
	assert.Equal(t, "", extractSubType(&node2))
}

func TestExtractName(t *testing.T) {
	node := fbxNode{
		properties: []fbxProp{{}, {strVal: "MyObject"}},
	}
	assert.Equal(t, "MyObject", extractName(&node))

	node2 := fbxNode{properties: []fbxProp{{}}}
	assert.Equal(t, formatName, extractName(&node2))
}

func TestWireASCIITextures(t *testing.T) {
	asset := &ir.Asset{
		Materials: []*ir.Material{
			{Name: "Mat0"},
		},
		Textures: []*ir.Texture{
			{Name: "Tex0"},
			{Name: "Tex1"},
			{Name: "Tex2"},
		},
	}
	conns := []asciiConnection{
		{child: 10, parent: 20, propName: fbxPropDiffuseColor},
		{child: 11, parent: 20, propName: fbxPropAmbientColor},
		{child: 12, parent: 20, propName: fbxPropSpecularColor},
	}
	texIDMap := map[int64]int{10: 0, 11: 1, 12: 2}
	matIDMap := map[int64]int{20: 0}

	wireASCIITextures(asset, conns, matIDMap, texIDMap)
	assert.NotNil(t, asset.Materials[0].BaseColorTexture)
	assert.Equal(t, 0, asset.Materials[0].BaseColorTexture.TextureIndex)
	assert.Equal(t, 1, asset.Materials[0].Properties[fbxPropAmbientTexture])
	assert.Equal(t, 2, asset.Materials[0].Properties[fbxPropSpecularTexture])
}

func TestBuildASCIIHierarchy(t *testing.T) {
	meshes := []*ir.Mesh{{Name: "Mesh0"}}
	geoIDs := []int64{100}
	models := []asciiModelInfo{{id: 200, name: "Model0"}}
	conns := []asciiConnection{{child: 100, parent: 200}}

	nodes := buildASCIIHierarchy(meshes, geoIDs, models, conns)
	assert.Len(t, nodes, 1)
	assert.Equal(t, 0, nodes[0].MeshIndex)
	assert.Equal(t, "Mesh0", nodes[0].Name)
}

func TestResolveASCIIMorphTargets(t *testing.T) {
	asset := &ir.Asset{
		Meshes: []*ir.Mesh{
			{
				Name: "M",
				Primitives: []ir.Primitive{{
					Data: ir.MeshData{VertexCount: 2},
				}},
			},
		},
	}
	res := asciiParseResult{
		shapes: []asciiShape{
			{id: 1, name: "Smile", deltas: [][3]float32{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}}},
		},
	}
	resolveASCIIMorphTargets(asset, res)
	assert.Len(t, asset.Meshes[0].Primitives[0].MorphTargets, 1)
	assert.Equal(t, "Smile", asset.Meshes[0].Primitives[0].MorphTargets[0].Name)
}

func TestResolveLayerIndex(t *testing.T) {
	tests := []struct {
		name         string
		le           layerElement
		polyVertIdx  int
		controlPtIdx int
		want         int
	}{
		{
			name:         "IndexToDirect_within_range",
			le:           layerElement{refMode: refIndexToDirect, indices: []int32{5, 10, 15}},
			polyVertIdx:  1,
			controlPtIdx: 99,
			want:         10,
		},
		{
			name:         "IndexToDirect_fallback",
			le:           layerElement{refMode: refIndexToDirect, indices: []int32{5}},
			polyVertIdx:  3,
			controlPtIdx: 7,
			want:         7,
		},
		{
			name:         "ByPolygonVertex",
			le:           layerElement{mapMode: mapByPolygonVtx, data: []float64{1}},
			polyVertIdx:  4,
			controlPtIdx: 2,
			want:         4,
		},
		{
			name:         "ByVertex",
			le:           layerElement{mapMode: mapByVertex, data: []float64{1}},
			polyVertIdx:  4,
			controlPtIdx: 2,
			want:         2,
		},
		{
			name:         "EmptyMapMode_defaults_to_ByPolygonVertex",
			le:           layerElement{mapMode: "", data: []float64{1}},
			polyVertIdx:  6,
			controlPtIdx: 3,
			want:         6,
		},
		{
			name:         "UnknownMapMode_defaults_to_polyVertIdx",
			le:           layerElement{mapMode: "ByEdge", data: []float64{1}},
			polyVertIdx:  8,
			controlPtIdx: 1,
			want:         8,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveLayerIndex(&tt.le, tt.polyVertIdx, tt.controlPtIdx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveConnections(t *testing.T) {
	asset := &ir.Asset{
		Nodes: []ir.Node{
			{Name: "Root", MeshIndex: ir.NoIndex, CameraIndex: ir.NoIndex, LightIndex: ir.NoIndex},
		},
		Materials: []*ir.Material{
			{Name: "Mat0", BaseColorFactor: defaultBaseColor, AlphaMode: ir.AlphaOpaque},
		},
		Meshes: []*ir.Mesh{
			{Name: "Mesh0", Primitives: []ir.Primitive{{MaterialIndex: ir.NoIndex}}},
		},
		Textures: []*ir.Texture{
			{Name: "Tex0", ImageIndex: 0},
			{Name: "Tex1", ImageIndex: 0},
			{Name: "Tex2", ImageIndex: 0},
		},
		Images: []*ir.ImageAsset{{Name: "Image0"}},
		Cameras: []*ir.Camera{
			{Name: "Cam0"},
		},
		Lights: []*ir.Light{
			{Name: "Light0"},
		},
	}

	modelMap := map[int64]int{1: 0}
	geoMap := map[int64]int{10: 0}
	matMap := map[int64]int{20: 0}
	camMap := map[int64]int{30: 0}
	lightMap := map[int64]int{40: 0}
	texMap := map[int64]int{50: 0, 51: 1, 52: 2}
	videoMap := map[int64][]byte{60: {0xFF, 0xD8}}

	conns := []connection{
		{childID: 10, parentID: 1},                                 // geo -> model
		{childID: 20, parentID: 1},                                 // mat -> model
		{childID: 30, parentID: 1},                                 // cam -> model
		{childID: 40, parentID: 1},                                 // light -> model
		{childID: 50, parentID: 20, propName: fbxPropDiffuseColor}, // tex -> mat
		{childID: 51, parentID: 20, propName: fbxPropAmbientColor},
		{childID: 52, parentID: 20, propName: fbxPropSpecularColor},
		{childID: 60, parentID: 50}, // video -> tex
	}

	resolveConnections(asset, conns, geoMap, matMap, modelMap, texMap, camMap, lightMap, videoMap, nil)

	assert.Equal(t, 0, asset.Nodes[0].MeshIndex)
	assert.Equal(t, 0, asset.Nodes[0].CameraIndex)
	assert.Equal(t, 0, asset.Nodes[0].LightIndex)
	assert.NotNil(t, asset.Materials[0].BaseColorTexture)
	assert.Equal(t, 0, asset.Materials[0].BaseColorTexture.TextureIndex)
	assert.Equal(t, 1, asset.Materials[0].Properties[fbxPropAmbientTexture])
	assert.Equal(t, 2, asset.Materials[0].Properties[fbxPropSpecularTexture])
	assert.Len(t, asset.Images, 1)
	assert.Equal(t, []byte{0xFF, 0xD8}, asset.Images[asset.Textures[0].ImageIndex].Compressed)
	assert.Len(t, asset.RootNodes, 1)
}
