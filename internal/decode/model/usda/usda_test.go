package usda_test

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/usda"
	"github.com/gophics/ravenporter/ir"
)

//go:embed testdata/multi_mesh.usda
var multiMeshUSDA []byte

//go:embed testdata/minimal.usdc
var minimalUSDC []byte

//go:embed testdata/comprehensive.usda
var comprehensiveUSDA []byte

//go:embed testdata/comprehensive.usdc
var comprehensiveUSDC []byte

//go:embed testdata/test.usdz
var testUSDZ []byte

//go:embed testdata/enriched.usdc
var enrichedUSDC []byte

const triangleUSDA = "#usda 1.0\n" +
	"(\n" +
	"    upAxis = \"Z\"\n" +
	"    metersPerUnit = 0.01\n" +
	")\n" +
	"\n" +
	"def Mesh \"Cube\" {\n" +
	"    point3f[] points = [(1, 0, 0), (0, 1, 0), (0, 0, 1)]\n" +
	"    int[] faceVertexIndices = [0, 1, 2]\n" +
	"    normal3f[] normals = [(0, 0, 1), (0, 0, 1), (0, 0, 1)]\n" +
	"}\n"

const texcoordsUSDA = "#usda 1.0\n" +
	"\n" +
	"def Mesh \"Tri\" {\n" +
	"    point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n" +
	"    int[] faceVertexIndices = [0, 1, 2]\n" +
	"    texCoord2f[] primvars:st = [(0, 0), (1, 0), (0.5, 1)]\n" +
	"}\n"

func TestUSDAProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"ValidUSDA", []byte(triangleUSDA), true},
		{"ValidUSDC", minimalUSDC, true},
		{"ValidUSDZ", testUSDZ, true},
		{"InvalidMagic", []byte("not usda data"), false},
		{"Empty", []byte(""), false},
		{"TooShort", []byte("#us"), false},
	}
	dec := &usda.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dec.Probe(bytes.NewReader(tt.data)))
		})
	}
}

func TestUSDAMeta(t *testing.T) {
	dec := &usda.Decoder{}
	assert.Contains(t, dec.Extensions(), ".usda")
	assert.Contains(t, dec.Extensions(), ".usd")
	assert.Contains(t, dec.Extensions(), ".usdc")
	assert.Equal(t, "USDA", dec.FormatName())
}

func TestUSDADecodeRejectsOversizedUSDZEntry(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)

	file, err := zw.Create("scene.usda")
	require.NoError(t, err)
	_, err = file.Write([]byte("#usda 1.0\n" + strings.Repeat("def Scope \"A\" {}\n", 2048)))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	dec := &usda.Decoder{}
	_, err = dec.Decode(bytes.NewReader(archive.Bytes()), detect.DecodeOptions{MaxFileSize: 256})
	require.Error(t, err)
	assert.ErrorContains(t, err, "file exceeds MaxFileSize limit")
}

func TestUSDADecodeAll(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, sc *ir.Asset)
	}{
		{"Basic Triangle", triangleUSDA, false, func(t *testing.T, sc *ir.Asset) {
			assert.Equal(t, ir.FormatUSD, sc.Metadata.SourceFormat)
			assert.Equal(t, ir.ZUp, sc.UpAxis)
			assert.InDelta(t, 0.01, sc.Unit, 1e-6)
			require.Len(t, sc.Meshes, 1)
			assert.Equal(t, "Cube", sc.Meshes[0].Name)
			prim := sc.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			require.Len(t, prim.Data.Positions, 3)
			require.Len(t, prim.Data.Indices, 3)
			assert.InDelta(t, 1.0, prim.Data.Positions[0][0], 1e-5)
			assert.Equal(t, uint32(0), prim.Data.Indices[0])
			require.Len(t, prim.Data.Normals, 3)
			for _, n := range prim.Data.Normals {
				length := n[0]*n[0] + n[1]*n[1] + n[2]*n[2]
				assert.InDelta(t, 1.0, length, 0.01)
			}
		}},
		{"Multiple Meshes", string(multiMeshUSDA), false, func(t *testing.T, sc *ir.Asset) {
			require.Len(t, sc.Meshes, 2)
			assert.Equal(t, "MeshA", sc.Meshes[0].Name)
			assert.Equal(t, "MeshB", sc.Meshes[1].Name)
			assert.Equal(t, ir.YUp, sc.UpAxis)
		}},
		{"Texture Coordinates", texcoordsUSDA, false, func(t *testing.T, sc *ir.Asset) {
			require.Len(t, sc.Meshes, 1)
			prim := sc.Meshes[0].Primitives[0]
			require.Len(t, prim.Data.TexCoord0, 3)
			assert.InDelta(t, float32(0.5), prim.Data.TexCoord0[2][0], 1e-5)
			assert.InDelta(t, float32(1), prim.Data.TexCoord0[2][1], 1e-5)
		}},
		{"Empty On Junk", "not usda", false, func(t *testing.T, sc *ir.Asset) {
			assert.Empty(t, sc.Meshes)
		}},
		{"Camera Prim", "#usda 1.0\n\ndef Camera \"MyCam\" {\n    float focalLength = 35\n    float horizontalAperture = 36\n    float verticalAperture = 24\n    float2 clippingRange = (0.1, 1000)\n}\n", false, func(t *testing.T, sc *ir.Asset) {
			require.Len(t, sc.Cameras, 1)
			cam := sc.Cameras[0]
			assert.Equal(t, "MyCam", cam.Name)
			require.NotNil(t, cam.Perspective)
			assert.InDelta(t, 0.1, cam.Perspective.Near, 0.01)
			assert.InDelta(t, 1000.0, cam.Perspective.Far, 0.01)
			assert.True(t, cam.Perspective.FOV > 0)
			assert.InDelta(t, 1.5, cam.Perspective.Aspect, 0.01)
			require.Len(t, sc.Nodes, 1)
			assert.Equal(t, 0, sc.Nodes[0].CameraIndex)
		}},
	}

	dec := &usda.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := dec.Decode(bytes.NewReader([]byte(tt.input)), detect.DecodeOptions{})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, sc)
				}
			}
		})
	}
}

func TestUSDADecodeLightPrims(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  string
		wantColor [3]float32
	}{
		{
			"DistantLight",
			"#usda 1.0\n\ndef DistantLight \"Sun\" {\n    color3f inputs:color = (1, 0.9, 0.8)\n    float inputs:intensity = 500\n}\n",
			"directional",
			[3]float32{1, 0.9, 0.8},
		},
		{
			"SphereLight",
			"#usda 1.0\n\ndef SphereLight \"Lamp\" {\n    float inputs:intensity = 100\n}\n",
			"point",
			[3]float32{1, 1, 1},
		},
		{
			"DiskLight",
			"#usda 1.0\n\ndef DiskLight \"Spot\" {\n    float inputs:intensity = 200\n    float inputs:shaping:cone:angle = 45\n}\n",
			"spot",
			[3]float32{1, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &usda.Decoder{}
			sc, err := dec.Decode(bytes.NewReader([]byte(tt.input)), detect.DecodeOptions{})
			require.NoError(t, err)
			require.Len(t, sc.Lights, 1)

			light := sc.Lights[0]
			assert.Equal(t, tt.wantColor, light.Color)
			assert.True(t, light.Intensity > 0)

			switch tt.wantType {
			case "directional":
				assert.NotNil(t, light.Directional)
			case "point":
				assert.NotNil(t, light.Point)
			case "spot":
				assert.NotNil(t, light.Spot)
				assert.True(t, light.Spot.OuterConeAngle > 0)
			}

			require.Len(t, sc.Nodes, 1)
			assert.Equal(t, 0, sc.Nodes[0].LightIndex)
		})
	}
}

const xformUSDA = "#usda 1.0\n" +
	"\n" +
	"def Xform \"Root\" {\n" +
	"    double3 xformOp:translate = (1, 2, 3)\n" +
	"    double3 xformOp:scale = (0.5, 0.5, 0.5)\n" +
	"\n" +
	"    def Mesh \"Child\" {\n" +
	"        point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n" +
	"        int[] faceVertexIndices = [0, 1, 2]\n" +
	"    }\n" +
	"}\n"

func TestUSDADecodeXformPrim(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(xformUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.True(t, len(sc.Nodes) >= 2)
	xform := sc.Nodes[1]
	assert.Equal(t, "Root", xform.Name)
	assert.Equal(t, [3]float32{1, 2, 3}, xform.Transform.Translation)
	assert.Equal(t, [3]float32{0.5, 0.5, 0.5}, xform.Transform.Scale)

	require.Len(t, sc.Meshes, 1)
	assert.Equal(t, "Child", sc.Meshes[0].Name)
}

const quadUSDA = "#usda 1.0\n" +
	"\n" +
	"def Mesh \"Quad\" {\n" +
	"    point3f[] points = [(0, 0, 0), (1, 0, 0), (1, 1, 0), (0, 1, 0)]\n" +
	"    int[] faceVertexIndices = [0, 1, 2, 3]\n" +
	"    int[] faceVertexCounts = [4]\n" +
	"}\n"

func TestUSDADecodeQuadTriangulation(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(quadUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	prim := sc.Meshes[0].Primitives[0]
	require.Len(t, prim.Data.Positions, 4)
	require.Len(t, prim.Data.Indices, 6)
	assert.Equal(t, []uint32{0, 1, 2, 0, 2, 3}, prim.Data.Indices)
}

const multiLineArrayUSDA = "#usda 1.0\n" +
	"\n" +
	"def Mesh \"Multi\" {\n" +
	"    point3f[] points = [\n" +
	"        (0, 0, 0),\n" +
	"        (1, 0, 0),\n" +
	"        (0, 1, 0)\n" +
	"    ]\n" +
	"    int[] faceVertexIndices = [\n" +
	"        0, 1, 2\n" +
	"    ]\n" +
	"}\n"

func TestUSDADecodeMultiLineArray(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(multiLineArrayUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	prim := sc.Meshes[0].Primitives[0]
	require.Len(t, prim.Data.Positions, 3)
	require.Len(t, prim.Data.Indices, 3)
}

const yUpUSDA = "#usda 1.0\n" +
	"\n" +
	"def Mesh \"YUpMesh\" {\n" +
	"    point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n" +
	"    int[] faceVertexIndices = [0, 1, 2]\n" +
	"}\n"

func TestUSDADecodeDefaultUpAxis(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(yUpUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, ir.YUp, sc.UpAxis)
	assert.InDelta(t, 1.0, sc.Unit, 1e-6)
}

func TestUSDADecodeComprehensive(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader(comprehensiveUSDA), detect.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, ir.ZUp, sc.UpAxis)
	assert.InDelta(t, 0.01, sc.Unit, 1e-6)

	require.True(t, len(sc.Meshes) >= 4)

	tri := sc.Meshes[0]
	assert.Equal(t, "Triangle", tri.Name)
	require.Len(t, tri.Primitives, 1)
	assert.Len(t, tri.Primitives[0].Data.Positions, 3)
	assert.Len(t, tri.Primitives[0].Data.Normals, 3)
	assert.Len(t, tri.Primitives[0].Data.TexCoord0, 3)
	assert.Len(t, tri.Primitives[0].Data.Indices, 3)

	quad := sc.Meshes[1]
	assert.Equal(t, "Quad", quad.Name)
	require.Len(t, quad.Primitives, 1)
	assert.Len(t, quad.Primitives[0].Data.Positions, 4)
	assert.Len(t, quad.Primitives[0].Data.Indices, 6)

	require.True(t, len(sc.Cameras) >= 2)
	cam := sc.Cameras[0]
	assert.Equal(t, "MainCam", cam.Name)
	require.NotNil(t, cam.Perspective)
	assert.True(t, cam.Perspective.FOV > 0)
	assert.True(t, cam.Perspective.Aspect > 0)
	assert.InDelta(t, 0.1, cam.Perspective.Near, 0.01)
	assert.InDelta(t, 1000, cam.Perspective.Far, 0.1)

	require.Len(t, sc.Lights, 4)

	sun := sc.Lights[0]
	assert.Equal(t, "Sun", sun.Name)
	require.NotNil(t, sun.Directional)
	assert.InDelta(t, 5.0, sun.Intensity, 0.01)
	assert.InDelta(t, 0.95, sun.Color[1], 0.01)

	lamp := sc.Lights[1]
	assert.Equal(t, "Lamp", lamp.Name)
	require.NotNil(t, lamp.Point)
	assert.InDelta(t, 100, lamp.Intensity, 0.01)

	spot := sc.Lights[2]
	assert.Equal(t, "Spot", spot.Name)
	require.NotNil(t, spot.Spot)
	assert.InDelta(t, 200, spot.Intensity, 0.01)
	assert.True(t, spot.Spot.OuterConeAngle > 0)

	panel := sc.Lights[3]
	assert.Equal(t, "Panel", panel.Name)
	require.NotNil(t, panel.Point)

	require.Len(t, sc.Materials, 1)
	assert.Equal(t, "WoodMat", sc.Materials[0].Name)
	assert.InDelta(t, 0.6, sc.Materials[0].BaseColorFactor[0], 0.01)
	assert.InDelta(t, 0.8, sc.Materials[0].RoughnessFactor, 0.01)
	assert.True(t, sc.Materials[0].DoubleSided)

	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Colors0, 3)

	worldIdx := sc.FindNode("World")
	require.NotEqual(t, ir.NoIndex, worldIdx)
	assert.NotEmpty(t, sc.Nodes[worldIdx].Children)

	helpersIdx := sc.FindNode("Helpers")
	require.NotEqual(t, ir.NoIndex, helpersIdx)

	require.True(t, len(sc.Cameras) >= 2)
	assert.NotNil(t, sc.Cameras[0].Perspective)

	var orthoIdx int
	for i, c := range sc.Cameras {
		if c.Name == "OrthoCam" {
			orthoIdx = i
		}
	}
	assert.Equal(t, "OrthoCam", sc.Cameras[orthoIdx].Name)
	require.NotNil(t, sc.Cameras[orthoIdx].Orthographic)
	assert.InDelta(t, 10, sc.Cameras[orthoIdx].Orthographic.XMag, 0.01)

	require.Len(t, sc.Skeletons, 1)
	assert.Equal(t, "Skel", sc.Skeletons[0].Name)
	assert.Len(t, sc.Skeletons[0].Joints, 3)

	boxIdx := sc.FindNode("Box")
	require.NotEqual(t, ir.NoIndex, boxIdx)
	assert.True(t, sc.Nodes[boxIdx].MeshIndex != ir.NoIndex)

	ballIdx := sc.FindNode("Ball")
	require.NotEqual(t, ir.NoIndex, ballIdx)
	assert.True(t, sc.Nodes[ballIdx].MeshIndex != ir.NoIndex)
}

func TestUSDCDecodeComprehensive(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader(comprehensiveUSDC), detect.DecodeOptions{})
	require.NoError(t, err)

	require.True(t, len(sc.Meshes) >= 2)
	assert.Equal(t, "Triangle", sc.Meshes[0].Name)
	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Positions, 3)
	assert.Equal(t, "Quad", sc.Meshes[1].Name)
	assert.Len(t, sc.Meshes[1].Primitives[0].Data.Positions, 4)

	require.Len(t, sc.Cameras, 1)
	assert.Equal(t, "MainCam", sc.Cameras[0].Name)
	require.NotNil(t, sc.Cameras[0].Perspective)
	assert.True(t, sc.Cameras[0].Perspective.FOV > 0)

	require.Len(t, sc.Lights, 3)
	assert.Equal(t, "Sun", sc.Lights[0].Name)
	assert.Equal(t, "Lamp", sc.Lights[1].Name)
	assert.Equal(t, "Spot", sc.Lights[2].Name)

	require.True(t, len(sc.Nodes) >= 7)

	worldIdx := sc.FindNode("World")
	require.NotEqual(t, ir.NoIndex, worldIdx)
	assert.NotEmpty(t, sc.Nodes[worldIdx].Children)
}

func TestUSDCDecodeEnriched(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader(enrichedUSDC), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, ir.FormatUSD, sc.Metadata.SourceFormat)
}

func TestUSDADecodeUSDZ(t *testing.T) {
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader(testUSDZ), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	assert.Equal(t, "ZipMesh", sc.Meshes[0].Name)
	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Positions, 3)

	require.Len(t, sc.Images, 1)
	assert.Equal(t, "textures/albedo.png", sc.Images[0].Name)
	assert.Equal(t, ir.ImagePNG, sc.Images[0].Format)
	assert.NotEmpty(t, sc.Images[0].Compressed)
}

func TestUSDAExtensionsIncludeUSDZ(t *testing.T) {
	dec := &usda.Decoder{}
	assert.Contains(t, dec.Extensions(), ".usdz")
}

func TestUSDADecodeBasisCurves(t *testing.T) {
	const curvesUSDA = "#usda 1.0\n\n" +
		"def BasisCurves \"Wire\" {\n" +
		"    point3f[] points = [(0, 0, 0), (1, 1, 0), (2, 0, 0), (3, 1, 0)]\n" +
		"    int[] curveVertexCounts = [4]\n" +
		"}\n"

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(curvesUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	prim := sc.Meshes[0].Primitives[0]
	assert.Equal(t, ir.Lines, prim.Mode)
	assert.Len(t, prim.Data.Positions, 4)
	assert.Len(t, prim.Data.Indices, 6)
}

func TestUSDADecodePointsPrim(t *testing.T) {
	const pointsUSDA = "#usda 1.0\n\n" +
		"def Points \"Dots\" {\n" +
		"    point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n" +
		"}\n"

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(pointsUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	prim := sc.Meshes[0].Primitives[0]
	assert.Equal(t, ir.Points, prim.Mode)
	assert.Len(t, prim.Data.Positions, 3)
}

func TestUSDADecodeDisplayOpacity(t *testing.T) {
	const opacityUSDA = "#usda 1.0\n\n" +
		"def Mesh \"OpMesh\" {\n" +
		"    point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n" +
		"    int[] faceVertexIndices = [0, 1, 2]\n" +
		"    color3f[] primvars:displayColor = [(1, 0, 0), (0, 1, 0), (0, 0, 1)]\n" +
		"    float[] primvars:displayOpacity = [0.5, 0.8, 1.0]\n" +
		"}\n"

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(opacityUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	colors := sc.Meshes[0].Primitives[0].Data.Colors0
	require.Len(t, colors, 3)
	assert.InDelta(t, float32(0.5), colors[0][3], 0.01)
	assert.InDelta(t, float32(0.8), colors[1][3], 0.01)
	assert.InDelta(t, float32(1.0), colors[2][3], 0.01)
	assert.InDelta(t, float32(1.0), colors[0][0], 0.01)
}

func TestUSDADecodeProceduralPrims(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Cylinder", "#usda 1.0\n\ndef Cylinder \"Cyl\" {\n    double radius = 0.5\n    double height = 2\n}\n"},
		{"Cone", "#usda 1.0\n\ndef Cone \"C\" {\n    double radius = 0.5\n    double height = 1\n}\n"},
		{"Capsule", "#usda 1.0\n\ndef Capsule \"Cap\" {\n    double radius = 0.3\n    double height = 1\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &usda.Decoder{}
			sc, err := dec.Decode(bytes.NewReader([]byte(tt.input)), detect.DecodeOptions{})
			require.NoError(t, err)
			require.Len(t, sc.Meshes, 1)
			prim := sc.Meshes[0].Primitives[0]
			assert.Equal(t, ir.Triangles, prim.Mode)
			assert.True(t, len(prim.Data.Positions) > 0)
			assert.True(t, len(prim.Data.Indices) > 0)
		})
	}
}

func TestUSDADecodeSkelBinding(t *testing.T) {
	const skelUSDA = "#usda 1.0\n\n" +
		"def Skeleton \"Skel\" {\n" +
		"    uniform token[] joints = [\"Root\", \"Root/Arm\", \"Root/Arm/Hand\"]\n" +
		"}\n\n" +
		"def Mesh \"SkinnedMesh\" {\n" +
		"    point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n" +
		"    int[] faceVertexIndices = [0, 1, 2]\n" +
		"    rel skel:skeleton = </Skel>\n" +
		"    int[] primvars:skel:jointIndices = [0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2]\n" +
		"    float[] primvars:skel:jointWeights = [1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0]\n" +
		"}\n"

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(skelUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Skeletons, 1)
	skel := sc.Skeletons[0]
	assert.Equal(t, "Skel", skel.Name)
	assert.Len(t, skel.Joints, 3)

	rootJoint := sc.Nodes[skel.Joints[0]]
	assert.Equal(t, "Root", rootJoint.Name)
	assert.True(t, rootJoint.IsJoint)
	assert.Len(t, rootJoint.Children, 1)

	armJoint := sc.Nodes[skel.Joints[1]]
	assert.Equal(t, "Arm", armJoint.Name)
	assert.Len(t, armJoint.Children, 1)

	require.Len(t, sc.Meshes, 1)
	meshNode := sc.Nodes[sc.FindNode("SkinnedMesh")]
	assert.Equal(t, 0, meshNode.SkinIndex)
	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Joints0, 3)
	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Weights0, 3)
}

func TestUSDADecodeTexture(t *testing.T) {
	const texUSDA = "#usda 1.0\n\n" +
		"def Material \"MatTex\" {\n" +
		"    def Shader \"Surface\" {\n" +
		"        uniform token info:id = \"UsdPreviewSurface\"\n" +
		"        color3f inputs:diffuseColor.connect = </MatTex/UsdUVTexture.outputs:rgb>\n" +
		"    }\n" +
		"    def Shader \"DiffTex\" {\n" +
		"        uniform token info:id = \"UsdUVTexture\"\n" +
		"        asset inputs:file = @albedo.png@\n" +
		"        token inputs:wrapS = \"repeat\"\n" +
		"        token inputs:wrapT = \"clamp\"\n" +
		"    }\n" +
		"}\n"

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(texUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Materials, 1)
	mat := sc.Materials[0]
	require.NotNil(t, mat.BaseColorTexture)

	require.Len(t, sc.Textures, 1)
	tex := sc.Textures[0]
	assert.Equal(t, "albedo.png", sc.Images[tex.ImageIndex].SourcePath)
	assert.Equal(t, ir.WrapRepeat, tex.WrapS)
	assert.Equal(t, ir.WrapClamp, tex.WrapT)
}

func TestUSDADecodeClearcoatIOR(t *testing.T) {
	const matUSDA = "#usda 1.0\n\n" +
		"def Material \"ClearMat\" {\n" +
		"    def Shader \"Surf\" {\n" +
		"        uniform token info:id = \"UsdPreviewSurface\"\n" +
		"        float inputs:clearcoat = 0.8\n" +
		"        float inputs:clearcoatRoughness = 0.1\n" +
		"        float inputs:ior = 1.45\n" +
		"        float inputs:opacityThreshold = 0.5\n" +
		"    }\n" +
		"}\n"

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(matUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Materials, 1)
	mat := sc.Materials[0]
	require.NotNil(t, mat.Properties)
	assert.InDelta(t, float32(0.8), mat.Properties["clearcoat"], 0.01)
	assert.InDelta(t, float32(0.1), mat.Properties["clearcoatRoughness"], 0.01)
	assert.InDelta(t, float32(1.45), mat.Properties["ior"], 0.01)
	assert.InDelta(t, float32(0.5), mat.AlphaCutoff, 0.01)
	assert.Equal(t, ir.AlphaMask, mat.AlphaMode)
}

func TestUSDADecodeLeftHanded(t *testing.T) {
	const lhUSDA = "#usda 1.0\n\n" +
		"def Mesh \"LHMesh\" {\n" +
		"    point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n" +
		"    int[] faceVertexIndices = [0, 1, 2]\n" +
		"    uniform token orientation = \"leftHanded\"\n" +
		"}\n"

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(lhUSDA)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	indices := sc.Meshes[0].Primitives[0].Data.Indices
	require.Len(t, indices, 3)
	assert.Equal(t, uint32(0), indices[0])
	assert.Equal(t, uint32(2), indices[1])
	assert.Equal(t, uint32(1), indices[2])
}

func TestUSDADecodeGeomSubset(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Material \"MatA\" {\n" +
		"    def Shader \"Surface\" {\n" +
		"        uniform token info:id = \"UsdPreviewSurface\"\n" +
		"    }\n" +
		"}\n" +
		"def Material \"MatB\" {\n" +
		"    def Shader \"Surface\" {\n" +
		"        uniform token info:id = \"UsdPreviewSurface\"\n" +
		"    }\n" +
		"}\n" +
		"def Mesh \"Split\" {\n" +
		"    point3f[] points = [(0,0,0),(1,0,0),(0,1,0),(1,1,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2,1,3,2]\n" +
		"    int[] faceVertexCounts = [3,3]\n" +
		"    def GeomSubset \"sub0\" {\n" +
		"        uniform token elementType = \"face\"\n" +
		"        uniform token familyName = \"materialBind\"\n" +
		"        int[] indices = [0]\n" +
		"        rel material:binding = </MatA>\n" +
		"    }\n" +
		"    def GeomSubset \"sub1\" {\n" +
		"        uniform token elementType = \"face\"\n" +
		"        uniform token familyName = \"materialBind\"\n" +
		"        int[] indices = [1]\n" +
		"        rel material:binding = </MatB>\n" +
		"    }\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, sc.Meshes, 1)
	require.Len(t, sc.Meshes[0].Primitives, 2)
	assert.Equal(t, 0, sc.Meshes[0].Primitives[0].MaterialIndex)
	assert.Equal(t, 1, sc.Meshes[0].Primitives[1].MaterialIndex)
	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Indices, 3)
	assert.Len(t, sc.Meshes[0].Primitives[1].Data.Indices, 3)
}

func TestUSDADecodeNurbsCurves(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def NurbsCurves \"Curve\" {\n" +
		"    point3f[] points = [(0,0,0),(1,0,0),(2,1,0)]\n" +
		"    int[] curveVertexCounts = [3]\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, sc.Meshes, 1)
	assert.Equal(t, "Curve", sc.Meshes[0].Name)
	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Positions, 3)
	assert.Equal(t, ir.Lines, sc.Meshes[0].Primitives[0].Mode)
}

func TestUSDADecodeCompositionArcs(t *testing.T) {
	input := "#usda 1.0\n" +
		"(\n" +
		"    subLayers = [@./base.usda@]\n" +
		")\n" +
		"def Xform \"Root\" (\n" +
		"    references = @./ref.usda@</Model>\n" +
		"    payload = @./heavy.usda@</Geo>\n" +
		"    inherits = </Class>\n" +
		"    specializes = </Base>\n" +
		") {\n" +
		"    def Mesh \"Box\" {\n" +
		"        point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"        int[] faceVertexIndices = [0,1,2]\n" +
		"    }\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, sc.Meshes, 1)
	assert.Equal(t, "Box", sc.Meshes[0].Name)
	assert.Len(t, sc.Meshes[0].Primitives[0].Data.Positions, 3)
}

func TestUSDAInheritsResolution(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Xform \"Base\" {\n" +
		"    def Mesh \"Geo\" {\n" +
		"        point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"        int[] faceVertexIndices = [0,1,2]\n" +
		"    }\n" +
		"}\n" +
		"def Xform \"Derived\" (\n" +
		"    inherits = </Base>\n" +
		") {\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)

	baseIdx := sc.FindNode("Base")
	derivedIdx := sc.FindNode("Derived")
	require.NotEqual(t, ir.NoIndex, baseIdx)
	require.NotEqual(t, ir.NoIndex, derivedIdx)

	if sc.Nodes[baseIdx].MeshIndex != ir.NoIndex {
		assert.Equal(t, sc.Nodes[baseIdx].MeshIndex, sc.Nodes[derivedIdx].MeshIndex)
	}
}

func TestUSDASpecializesResolution(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Xform \"Base\" {\n" +
		"    def Mesh \"Geo\" {\n" +
		"        point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"        int[] faceVertexIndices = [0,1,2]\n" +
		"    }\n" +
		"}\n" +
		"def Xform \"Special\" (\n" +
		"    specializes = </Base>\n" +
		") {\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)

	baseIdx := sc.FindNode("Base")
	specialIdx := sc.FindNode("Special")
	require.NotEqual(t, ir.NoIndex, baseIdx)
	require.NotEqual(t, ir.NoIndex, specialIdx)

	if sc.Nodes[baseIdx].MeshIndex != ir.NoIndex {
		assert.Equal(t, sc.Nodes[baseIdx].MeshIndex, sc.Nodes[specialIdx].MeshIndex)
	}
}

func TestUSDADecodeSkelAnimation(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Skeleton \"Skel\" {\n" +
		"    uniform token[] joints = [\"Root\", \"Root/Arm\"]\n" +
		"    uniform matrix4d[] bindTransforms = [" +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,0,0,1))," +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,1,0,1))]\n" +
		"    uniform matrix4d[] restTransforms = [" +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,0,0,1))," +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,1,0,1))]\n" +
		"}\n" +
		"def SkelAnimation \"Anim\" {\n" +
		"    uniform token[] joints = [\"Root\", \"Root/Arm\"]\n" +
		"    float3[] translations = [(0,0,0),(0,1,0)]\n" +
		"    quatf[] rotations = [(1,0,0,0),(1,0,0,0)]\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, sc.Animations, 1)
	anim := sc.Animations[0]
	assert.Equal(t, "Anim", anim.Name)
	assert.GreaterOrEqual(t, len(anim.Channels), 2)
}

func TestUSDADecodeSkelAnimTimeSamples(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Skeleton \"Skel\" {\n" +
		"    uniform token[] joints = [\"Root\", \"Root/Arm\"]\n" +
		"    uniform matrix4d[] bindTransforms = [" +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,0,0,1))," +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,1,0,1))]\n" +
		"    uniform matrix4d[] restTransforms = [" +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,0,0,1))," +
		"((1,0,0,0),(0,1,0,0),(0,0,1,0),(0,1,0,1))]\n" +
		"}\n" +
		"def SkelAnimation \"Walk\" {\n" +
		"    uniform token[] joints = [\"Root\", \"Root/Arm\"]\n" +
		"    float3[] translations.timeSamples = {\n" +
		"        0: [(0,0,0),(0,1,0)],\n" +
		"        12: [(0,0.5,0),(0,1.5,0)],\n" +
		"        24: [(1,0,0),(1,1,0)],\n" +
		"    }\n" +
		"    quatf[] rotations.timeSamples = {\n" +
		"        0: [(1,0,0,0),(1,0,0,0)],\n" +
		"        12: [(0.707,0,0.707,0),(0.707,0,0.707,0)],\n" +
		"        24: [(0,0,1,0),(0,0,1,0)],\n" +
		"    }\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, sc.Animations, 1)
	anim := sc.Animations[0]
	assert.Equal(t, "Walk", anim.Name)
	assert.Equal(t, float64(24), anim.Duration)
	for _, ch := range anim.Channels {
		assert.Len(t, ch.Times, 3)
	}
}
func TestUSDADecodeBlendShape(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Mesh \"Face\" {\n" +
		"    point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2]\n" +
		"    int[] faceVertexCounts = [3]\n" +
		"    def BlendShape \"smile\" {\n" +
		"        uniform vector3f[] offsets = [(0,0.5,0),(0,0.3,0)]\n" +
		"        uniform int[] pointIndices = [0, 2]\n" +
		"        uniform vector3f[] normalOffsets = [(0,0,1),(0,0,1)]\n" +
		"    }\n" +
		"    def BlendShape \"frown\" {\n" +
		"        uniform vector3f[] offsets = [(0,-0.2,0)]\n" +
		"        uniform int[] pointIndices = [1]\n" +
		"    }\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, sc.Meshes, 1)
	require.Len(t, sc.Meshes[0].Primitives, 1)
	prim := sc.Meshes[0].Primitives[0]
	require.Len(t, prim.MorphTargets, 2)
	assert.Equal(t, "smile", prim.MorphTargets[0].Name)
	assert.Len(t, prim.MorphTargets[0].Positions, 2)
	assert.Len(t, prim.MorphTargets[0].Indices, 2)
	assert.Len(t, prim.MorphTargets[0].Normals, 2)
	assert.Equal(t, "frown", prim.MorphTargets[1].Name)
	assert.Len(t, prim.MorphTargets[1].Positions, 1)
}

func BenchmarkDecodeUSDA(b *testing.B) {
	data := []byte(triangleUSDA)
	dec := &usda.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func TestUSDCDecodeCrate(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"ValidCrate", minimalUSDC, false},
		{"TruncatedHeader", []byte("PXR-USDC"), true},
		{"EmptyAfterMagic", append([]byte("PXR-USDC"), make([]byte, 80)...), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &usda.Decoder{}
			sc, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, ir.FormatUSD, sc.Metadata.SourceFormat)
		})
	}
}

func TestUSDADecodeAnimatedMeshTimeSamples(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Mesh \"Anim\" {\n" +
		"    point3f[] points.timeSamples = {\n" +
		"        0: [(0, 0, 0), (1, 0, 0), (0, 1, 0)],\n" +
		"        1: [(0.1, 0, 0), (1.1, 0, 0), (0.1, 1, 0)],\n" +
		"    }\n" +
		"    int[] faceVertexIndices = [0, 1, 2]\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	prim := sc.Meshes[0].Primitives[0]
	require.Len(t, prim.Data.Positions, 3)
	assert.InDelta(t, float32(0), prim.Data.Positions[0][0], 0.01)

	require.Len(t, prim.MorphTargets, 1)
	mt := prim.MorphTargets[0]
	require.Len(t, mt.Positions, 3)
	assert.InDelta(t, float32(0.1), mt.Positions[0][0], 0.01)
}

func TestUSDADecodeVariantSets(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Mesh \"Box\" (\n" +
		"    variantSets = [\"color\", \"size\"]\n" +
		") {\n" +
		"    point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2]\n" +
		"    variantSet \"color\" = {\n" +
		"        \"red\" {\n" +
		"        }\n" +
		"        \"blue\" {\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)

	require.Len(t, sc.Meshes, 1)
	assert.Equal(t, "Box", sc.Meshes[0].Name)

	if sc.Metadata.ExtraProperties != nil {
		assert.Contains(t, sc.Metadata.ExtraProperties, "variantSets")
	}
}

func BenchmarkDecodeUSDC(b *testing.B) {
	dec := &usda.Decoder{}
	b.ReportAllocs()
	b.SetBytes(int64(len(minimalUSDC)))
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(minimalUSDC), detect.DecodeOptions{})
	}
}

func TestUSDADecodeScopeWithChildren(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Scope \"Grp\" {\n" +
		"    def Mesh \"Inside\" {\n" +
		"        point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"        int[] faceVertexIndices = [0,1,2]\n" +
		"    }\n" +
		"    def Camera \"ScopeCam\" {\n" +
		"        float focalLength = 50\n" +
		"        float horizontalAperture = 36\n" +
		"        float verticalAperture = 24\n" +
		"        float2 clippingRange = (0.1, 100)\n" +
		"    }\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)

	grpIdx := sc.FindNode("Grp")
	require.NotEqual(t, ir.NoIndex, grpIdx)
	assert.NotEmpty(t, sc.Nodes[grpIdx].Children)
	require.Len(t, sc.Meshes, 1)
	assert.Equal(t, "Inside", sc.Meshes[0].Name)
	require.Len(t, sc.Cameras, 1)
}

func TestUSDADecodeXformMatrix(t *testing.T) {
	input := "#usda 1.0\n\n" +
		"def Xform \"MatXf\" {\n" +
		"    matrix4d xformOp:transform = ((1,0,0,0),(0,1,0,0),(0,0,1,0),(5,10,15,1))\n" +
		"    def Mesh \"Child\" {\n" +
		"        point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"        int[] faceVertexIndices = [0,1,2]\n" +
		"    }\n" +
		"}\n"
	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader([]byte(input)), detect.DecodeOptions{})
	require.NoError(t, err)

	xfIdx := sc.FindNode("MatXf")
	require.NotEqual(t, ir.NoIndex, xfIdx)
	// Matrix should have translation (5,10,15) in row-major layout
	m := sc.Nodes[xfIdx].Transform.Matrix
	assert.InDelta(t, float32(5), m[12], 0.01)
	assert.InDelta(t, float32(10), m[13], 0.01)
}

func buildUSDZ(files [][2]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, _ := zw.Create(f[0])
		_, _ = w.Write([]byte(f[1]))
	}
	zw.Close()
	return buf.Bytes()
}

func TestUSDZResolveReference(t *testing.T) {
	refMesh := "#usda 1.0\n\n" +
		"def Mesh \"RefBox\" {\n" +
		"    point3f[] points = [(0,0,0),(1,0,0),(0,1,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2]\n" +
		"}\n"

	rootUSDA := "#usda 1.0\n\n" +
		"    references = @./geom/ref.usda@\n"

	archive := buildUSDZ([][2]string{
		{"root.usda", rootUSDA},
		{"geom/ref.usda", refMesh},
	})

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader(archive), detect.DecodeOptions{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sc.Meshes), 1)
	assert.Equal(t, "RefBox", sc.Meshes[0].Name)
}

func TestUSDZPayloadReference(t *testing.T) {
	payloadMesh := "#usda 1.0\n\n" +
		"def Mesh \"PayloadBox\" {\n" +
		"    point3f[] points = [(0,0,0),(2,0,0),(0,2,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2]\n" +
		"}\n"

	rootUSDA := "#usda 1.0\n\n" +
		"    payload = @./payload.usda@\n"

	archive := buildUSDZ([][2]string{
		{"root.usda", rootUSDA},
		{"payload.usda", payloadMesh},
	})

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader(archive), detect.DecodeOptions{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sc.Meshes), 1)
	assert.Equal(t, "PayloadBox", sc.Meshes[0].Name)
}

func TestUSDZParseSublayers(t *testing.T) {
	sublayerMesh := "#usda 1.0\n\n" +
		"def Mesh \"SubMesh\" {\n" +
		"    point3f[] points = [(0,0,0),(3,0,0),(0,3,0)]\n" +
		"    int[] faceVertexIndices = [0,1,2]\n" +
		"}\n"

	rootUSDA := "#usda 1.0\n(\n" +
		"    subLayers = [@./sub.usda@]\n" +
		")\n\n"

	archive := buildUSDZ([][2]string{
		{"root.usda", rootUSDA},
		{"sub.usda", sublayerMesh},
	})

	dec := &usda.Decoder{}
	sc, err := dec.Decode(bytes.NewReader(archive), detect.DecodeOptions{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sc.Meshes), 1)
	assert.Equal(t, "SubMesh", sc.Meshes[0].Name)
}
