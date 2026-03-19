package fbx

import (
	"context"
	"testing"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
)

func TestParseASCIICamera(t *testing.T) {
	src := `NodeAttribute: 123, "NodeAttribute::MyCam", "Camera" {
		TypeFlags: "Camera"
		Properties70:  {
			P: "FieldOfView", "double", "Number", "", 60.5
			P: "NearPlane", "double", "Number", "", 0.1
			P: "FarPlane", "double", "Number", "", 1000.0
	}`
	s := &decutil.LineScanner{Data: []byte(src)}
	s.Next() // consume outer node
	depth := 2
	cam := parseASCIICamera(s, &depth, "MyCam")

	assert.Equal(t, "MyCam", cam.Name)
	assert.NotNil(t, cam.Perspective)
	assert.InDelta(t, 60.5*degToRad, cam.Perspective.FOV, 0.001)
	assert.InDelta(t, 0.1, cam.Perspective.Near, 0.001)
	assert.InDelta(t, 1000.0, cam.Perspective.Far, 0.001)
}

func TestParseASCIILight(t *testing.T) {
	src := `NodeAttribute: 456, "NodeAttribute::MyLight", "Light" {
		Properties70:  {
			P: "LightType", "enum", "", "", 2
			P: "Color", "ColorRGB", "Color", "", 1.0, 0.5, 0.25
			P: "Intensity", "double", "Number", "", 200.0
			P: "InnerAngle", "double", "Number", "", 30.0
			P: "OuterAngle", "double", "Number", "", 45.0
	}`
	s := &decutil.LineScanner{Data: []byte(src)}
	s.Next() // consume outer node
	depth := 2
	light := parseASCIILight(s, &depth, "MyLight")

	assert.Equal(t, "MyLight", light.Name)
	assert.InDelta(t, 1.0, light.Color[0], 0.001)
	assert.InDelta(t, 0.5, light.Color[1], 0.001)
	assert.InDelta(t, 0.25, light.Color[2], 0.001)
	assert.InDelta(t, 200.0/100.0, light.Intensity, 0.001) // factor of 100
	assert.NotNil(t, light.Spot)
	assert.InDelta(t, 30.0*degToRad, light.Spot.InnerConeAngle, 0.001)
	assert.InDelta(t, 45.0*degToRad, light.Spot.OuterConeAngle, 0.001)
}

func TestParseASCIITexture(t *testing.T) {
	src := `Texture: 789, "Texture::Diffuse", "" {
		Type: "TextureVideoClip"
		Version: 202
		TextureName: "Texture::Diffuse"
		Properties70:  {
			P: "CurrentTextureBlendMode", "enum", "", "", 0
		}
		Media: "Video::Diffuse"
		FileName: "C:\fake\path\diffuse.png"
		RelativeFilename: "textures/diffuse.png"
	}`
	s := &decutil.LineScanner{Data: []byte(src)}
	s.Next() // consume outer node
	depth := 2
	tex := parseASCIITexture(s, &depth, 789, "Diffuse")
	assert.Equal(t, int64(789), tex.id)
	assert.Equal(t, "Diffuse", tex.name)
	assert.Equal(t, "textures/diffuse.png", tex.path)
}

func TestParseASCIIGeometry(t *testing.T) {
	src := `Geometry: 111, "Geometry::Mesh", "Mesh" {
		Vertices: *3 {
			a: 1.0,2.0,3.0
		}
		PolygonVertexIndex: *3 {
			a: 0,1,-3
		}
		LayerElementNormal: 0 {
			Normals: *3 {
				a: 0.0,1.0,0.0
			}
		}
		LayerElementUV: 0 {
			UV: *2 {
				a: 0.5,0.5
			}
	}`
	s := &decutil.LineScanner{Data: []byte(src)}
	s.Next() // consume outer node
	depth := 2
	geo := parseASCIIGeometry(s, &depth)

	assert.Len(t, geo.positions, 1) // 1 vec3
	assert.Len(t, geo.indices, 3)   // 3 indices
	assert.Len(t, geo.normals, 1)   // 1 vec3
	assert.Len(t, geo.uvs, 1)       // 1 vec2
}

func TestParseASCIIDeformer(t *testing.T) {
	src := `Deformer: 222, "Deformer::Sub", "SubDeformer" {
		Indexes: *2 {
			a: 0,1
		}
		Weights: *2 {
			a: 0.5,0.8
		}
		TransformLink: *16 {
			a: 1,0,0,0,0,1,0,0,0,0,1,0,10,20,30,1
	}`
	s := &decutil.LineScanner{Data: []byte(src)}
	s.Next() // consume outer node
	depth := 2
	def := parseASCIIDeformer(s, &depth, 222)

	assert.Equal(t, int64(222), def.id)
	assert.Equal(t, []int32{0, 1}, def.idxs)
	assert.Equal(t, []float64{0.5, 0.8}, def.weights)
	assert.Equal(t, float32(10), def.ibm[12])
	assert.Equal(t, float32(20), def.ibm[13])
}

func TestParseASCIIAnimCurve(t *testing.T) {
	src := `AnimationCurve: 333, "AnimCurve::Tx", "" {
		KeyTime: *2 {
			a: 46186158000,92372316000
		}
		KeyValueFloat: *2 {
			a: 10.5,20.5
	}`
	s := &decutil.LineScanner{Data: []byte(src)}
	s.Next() // consume outer node
	depth := 2
	curve := parseASCIIAnimCurve(s, &depth, 333)

	assert.Equal(t, int64(333), curve.id)
	assert.Equal(t, []float64{46186158000, 92372316000}, curve.keyTimes)
	assert.Equal(t, []float64{10.5, 20.5}, curve.keyVals)
}

func TestParseASCIIShape(t *testing.T) {
	src := `Geometry: 444, "Geometry::Smile", "Shape" {
		Indexes: *2 {
			a: 0,1
		}
		Vertices: *6 {
			a: 0.1,0.2,0.3, 0.4,0.5,0.6
	}`
	s := &decutil.LineScanner{Data: []byte(src)}
	s.Next() // consume outer node
	depth := 2
	shape := parseASCIIShape(s, &depth, 444, "Smile")

	assert.Equal(t, int64(444), shape.id)
	assert.Equal(t, "Smile", shape.name)
	assert.Len(t, shape.deltas, 2)
	assert.InDelta(t, 0.1, shape.deltas[0][0], 0.001)
	assert.InDelta(t, 0.6, shape.deltas[1][2], 0.001)
}

func TestResolveASCIIAnimTarget(t *testing.T) {
	assert.Equal(t, ir.TargetTranslation, resolveASCIIAnimTarget("T"))
	assert.Equal(t, ir.TargetTranslation, resolveASCIIAnimTarget("Lcl Translation"))
	assert.Equal(t, ir.TargetRotation, resolveASCIIAnimTarget("R"))
	assert.Equal(t, ir.TargetRotation, resolveASCIIAnimTarget("Lcl Rotation"))
	assert.Equal(t, ir.TargetScale, resolveASCIIAnimTarget("S"))
	assert.Equal(t, ir.TargetScale, resolveASCIIAnimTarget("Lcl Scaling"))
	assert.Equal(t, ir.TargetTranslation, resolveASCIIAnimTarget("Unknown"))
}

func TestFloatsToVec3(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6}
	out := floatsToVec3(in)
	assert.Len(t, out, 2)
	assert.Equal(t, [3]float32{1, 2, 3}, out[0])
	assert.Equal(t, [3]float32{4, 5, 6}, out[1])
}

func TestParseASCIIIntArray(t *testing.T) {
	src := "a: 10,20,30\n}"
	s := &decutil.LineScanner{Data: []byte(src)}
	result := parseASCIIIntArray(s, 0)
	assert.Equal(t, []int32{10, 20, 30}, result)
}

func TestParseASCIIFloatArray(t *testing.T) {
	src := "a: 1.5,2.5,3.5\n}"
	s := &decutil.LineScanner{Data: []byte(src)}
	result := parseASCIIFloatArray(s, 0)
	assert.Equal(t, []float64{1.5, 2.5, 3.5}, result)
}

func TestAsciiPropVal(t *testing.T) {
	line := []byte(`P: "FieldOfView", "double", "Number", "", 60.5`)
	v, ok := asciiPropVal(line)
	assert.True(t, ok)
	assert.InDelta(t, 60.5, v, 0.001)

	_, ok2 := asciiPropVal([]byte("no comma here"))
	assert.False(t, ok2)
}

func TestAsciiPropColor(t *testing.T) {
	line := []byte(`P: "Color", "ColorRGB", "Color", "", 1.0, 0.5, 0.25`)
	c := asciiPropColor(line)
	assert.InDelta(t, 1.0, c[0], 0.001)
	assert.InDelta(t, 0.5, c[1], 0.001)
	assert.InDelta(t, 0.25, c[2], 0.001)
}

func TestExtractQuotedValueB(t *testing.T) {
	line := []byte(`RelativeFilename: "textures/diffuse.png"`)
	assert.Equal(t, "textures/diffuse.png", extractQuotedValueB(line))

	assert.Equal(t, "", extractQuotedValueB([]byte("no quotes")))
	assert.Equal(t, "", extractQuotedValueB([]byte(`"unclosed`)))
}

func TestExtractASCIIName(t *testing.T) {
	line := []byte(`Model: 123, "Model::MyCube", "Mesh" {`)
	assert.Equal(t, "MyCube", extractASCIIName(line))

	line2 := []byte(`Geometry: 456, "Geometry::Body", "Mesh" {`)
	assert.Equal(t, "Body", extractASCIIName(line2))

	assert.Equal(t, defaultMeshName, extractASCIIName([]byte("no quotes")))
}

func TestParseASCIIConnections(t *testing.T) {
	src := `Connections:  {
	C: "OO",100,200
	C: "OP",300,400,"DiffuseColor"
}`
	s := &decutil.LineScanner{Data: []byte(src)}
	conns, _ := parseASCIIConnections(context.Background(), s)
	assert.Len(t, conns, 2)
	assert.Equal(t, int64(100), conns[0].child)
	assert.Equal(t, int64(200), conns[0].parent)
	assert.Equal(t, int64(300), conns[1].child)
	assert.Equal(t, int64(400), conns[1].parent)
	assert.Equal(t, "DiffuseColor", conns[1].propName)
}
