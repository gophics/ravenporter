package dae

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
)

func TestDAEBasics(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"ProbeValid", func(t *testing.T) {
			d := &Decoder{}
			assert.True(t, d.Probe(bytes.NewReader([]byte(`<?xml version="1.0"?><COLLADA>`))))
		}},
		{"ProbeRejectsNonDAE", func(t *testing.T) {
			d := &Decoder{}
			assert.False(t, d.Probe(bytes.NewReader([]byte("glTF\x02\x00\x00\x00"))))
		}},
		{"ParseUpAxis", func(t *testing.T) {
			assert.Equal(t, ir.YUp, parseUpAxis("Y_UP"))
			assert.Equal(t, ir.ZUp, parseUpAxis("Z_UP"))
			assert.Equal(t, ir.YUp, parseUpAxis(""))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.check(t) })
	}
}

func TestResolveInterpolation(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		expect ir.Interpolation
	}{
		{"Linear", []string{"LINEAR"}, ir.InterpolationLinear},
		{"Bezier", []string{"BEZIER"}, ir.InterpolationCubicSpline},
		{"Hermite", []string{"HERMITE"}, ir.InterpolationCubicSpline},
		{"Cardinal", []string{"CARDINAL"}, ir.InterpolationCubicSpline},
		{"Step", []string{"STEP"}, ir.InterpolationStep},
		{"Empty", nil, ir.InterpolationLinear},
		{"Unknown", []string{"CUSTOM"}, ir.InterpolationLinear},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sources []source
			ref := ""
			if len(tt.values) > 0 {
				ref = "#interp-src"
				src := source{ID: "interp-src"}
				data := ""
				for i, v := range tt.values {
					if i > 0 {
						data += " "
					}
					data += v
				}
				src.FloatArray.Data = data
				src.FloatArray.Count = len(tt.values)
				sources = append(sources, src)
			}

			got := resolveInterpolation(sources, ref)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestConvertMorphController(t *testing.T) {
	tests := []struct {
		name        string
		ctrl        xmlController
		meshes      []*ir.Mesh
		geoMap      map[string][][3]float32
		wantTargets int
		wantWeights int
	}{
		{
			"WithTargetsAndWeights",
			xmlController{
				Morph: xmlMorph{
					Source: "#base-geo",
					Sources: []source{
						{ID: "targets", FloatArray: floatArray{Data: "shape1 shape2", Count: 2}},
						{ID: "weights", FloatArray: floatArray{Data: "0.5 0.8", Count: 2}},
					},
					Targets: xmlMorphTargets{Inputs: []input{
						{Semantic: "MORPH_TARGET", Source: "#targets"},
						{Semantic: "MORPH_WEIGHT", Source: "#weights"},
					}},
				},
			},
			[]*ir.Mesh{{Name: "base-geo", Primitives: []ir.Primitive{{Mode: ir.Triangles}}}},
			map[string][][3]float32{"shape1": {{1, 0, 0}}, "shape2": {{0, 1, 0}}},
			2, 2,
		},
		{
			"NoTargets",
			xmlController{Morph: xmlMorph{Sources: nil}},
			[]*ir.Mesh{{Name: "mesh", Primitives: []ir.Primitive{{Mode: ir.Triangles}}}},
			nil,
			0, 0,
		},
		{
			"NoMatchingMesh",
			xmlController{
				Morph: xmlMorph{
					Source: "#nonexistent",
					Sources: []source{
						{ID: "targets", FloatArray: floatArray{Data: "shape1", Count: 1}},
					},
					Targets: xmlMorphTargets{Inputs: []input{
						{Semantic: "MORPH_TARGET", Source: "#targets"},
					}},
				},
			},
			[]*ir.Mesh{{Name: "other", Primitives: []ir.Primitive{{Mode: ir.Triangles}}}},
			nil,
			0, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{Meshes: tt.meshes}
			convertMorphController(context.Background(), &tt.ctrl, asset, tt.geoMap)

			if tt.wantTargets == 0 {
				for _, m := range asset.Meshes {
					for _, p := range m.Primitives {
						assert.Empty(t, p.MorphTargets)
					}
				}
				return
			}

			require.NotEmpty(t, asset.Meshes)
			prim := asset.Meshes[0].Primitives[0]
			assert.Len(t, prim.MorphTargets, tt.wantTargets)
			assert.Len(t, asset.Meshes[0].MorphWeights, tt.wantWeights)

			if tt.wantTargets > 0 {
				assert.Equal(t, "shape1", prim.MorphTargets[0].Name)
				assert.Len(t, prim.MorphTargets[0].Positions, 1)
			}
		})
	}
}

func colladaWrap(meshBody string) string {
	return `<?xml version="1.0"?>
<COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema" version="1.4.1">
  <asset><up_axis>Y_UP</up_axis></asset>
  <library_geometries>
    <geometry id="geo" name="Mesh">
      <mesh>
        <source id="pos"><float_array id="pos-a" count="12">0 0 0 1 0 0 0 1 0 1 1 0</float_array>
          <technique_common><accessor source="#pos-a" count="4" stride="3"><param name="X" type="float"/><param name="Y" type="float"/><param name="Z" type="float"/></accessor></technique_common>
        </source>
        <vertices id="verts"><input semantic="POSITION" source="#pos"/></vertices>
` + meshBody + `
      </mesh>
    </geometry>
  </library_geometries>
  <library_visual_scenes>
    <visual_scene id="Scene" name="Scene">
      <node name="Node" type="NODE">
        <instance_geometry url="#geo"/>
      </node>
    </visual_scene>
  </library_visual_scenes>
  <scene><instance_visual_scene url="#Scene"/></scene>
</COLLADA>`
}

func TestDAEPrimitiveTypes(t *testing.T) {
	tests := []struct {
		name        string
		meshBody    string
		wantMode    ir.PrimitiveMode
		wantVerts   int
		wantIndices int
	}{
		{
			name:        "tristrips 4 verts to 2 triangles",
			meshBody:    `<tristrips count="2"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2 3</p></tristrips>`,
			wantMode:    ir.Triangles,
			wantVerts:   4,
			wantIndices: 6,
		},
		{
			name:        "trifans 4 verts to 2 triangles",
			meshBody:    `<trifans count="2"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2 3</p></trifans>`,
			wantMode:    ir.Triangles,
			wantVerts:   4,
			wantIndices: 6,
		},
		{
			name:        "linestrips 3 verts to 2 line segments",
			meshBody:    `<linestrips count="1"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2</p></linestrips>`,
			wantMode:    ir.Lines,
			wantVerts:   4,
			wantIndices: 4,
		},
		{
			name:        "polygons quad to 2 triangles",
			meshBody:    `<polygons count="1"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2 3</p></polygons>`,
			wantMode:    ir.Triangles,
			wantVerts:   4,
			wantIndices: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml := colladaWrap(tt.meshBody)
			d := &Decoder{}
			asset, err := d.Decode(bytes.NewReader([]byte(xml)), detect.DecodeOptions{})
			require.NoError(t, err)
			require.NotNil(t, asset)
			require.Len(t, asset.Meshes, 1)
			require.NotEmpty(t, asset.Meshes[0].Primitives)

			prim := asset.Meshes[0].Primitives[0]
			assert.Equal(t, tt.wantMode, prim.Mode)
			assert.GreaterOrEqual(t, prim.Data.VertexCount, 1)
			assert.Len(t, prim.Data.Indices, tt.wantIndices)
		})
	}
}

func TestDAEDecodeAll(t *testing.T) {
	tests := []struct {
		name    string
		inputFn func() ([]byte, error)
		check   func(t *testing.T, asset *ir.Asset)
	}{
		{
			"Triangle",
			func() ([]byte, error) { return os.ReadFile("testdata/triangle.dae") },
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.FormatDAE, asset.Metadata.SourceFormat)
				assert.Equal(t, "1.4.1", asset.Metadata.SourceVersion)
				assert.Equal(t, ir.YUp, asset.UpAxis)
				assert.InDelta(t, 1.0, asset.Unit, 0.01)
				require.Len(t, asset.Meshes, 1)
				assert.Equal(t, "Triangle", asset.Meshes[0].Name)
				prim := asset.Meshes[0].Primitives[0]
				assert.Equal(t, ir.Triangles, prim.Mode)
				assert.Equal(t, 3, prim.Data.VertexCount)
				require.Len(t, prim.Data.Positions, 3)
				require.Len(t, prim.Data.Indices, 3)
				require.Len(t, asset.Materials, 1)
				assert.Equal(t, "Red", asset.Materials[0].Name)
				assert.InDelta(t, float32(1.0), asset.Materials[0].BaseColorFactor[0], 0.01)
				require.Len(t, asset.Nodes, 1)
				assert.Equal(t, "TriNode", asset.Nodes[0].Name)
			},
		},
		{
			"MultiUV",
			func() ([]byte, error) {
				return []byte(`<?xml version="1.0"?>
<COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema" version="1.4.1">
  <asset><up_axis>Y_UP</up_axis></asset>
  <library_geometries>
    <geometry id="geo" name="Plane">
      <mesh>
        <source id="pos"><float_array id="pos-a" count="9">0 0 0 1 0 0 0 1 0</float_array>
          <technique_common><accessor source="#pos-a" count="3" stride="3"><param name="X" type="float"/><param name="Y" type="float"/><param name="Z" type="float"/></accessor></technique_common>
        </source>
        <source id="uv0"><float_array id="uv0-a" count="6">0 0 1 0 0 1</float_array>
          <technique_common><accessor source="#uv0-a" count="3" stride="2"><param name="S" type="float"/><param name="T" type="float"/></accessor></technique_common>
        </source>
        <source id="uv1"><float_array id="uv1-a" count="6">0.5 0.5 1.5 0.5 0.5 1.5</float_array>
          <technique_common><accessor source="#uv1-a" count="3" stride="2"><param name="S" type="float"/><param name="T" type="float"/></accessor></technique_common>
        </source>
        <vertices id="verts"><input semantic="POSITION" source="#pos"/></vertices>
        <polylist count="1">
          <input semantic="VERTEX" source="#verts" offset="0"/>
          <input semantic="TEXCOORD" source="#uv0" offset="1" set="0"/>
          <input semantic="TEXCOORD" source="#uv1" offset="2" set="1"/>
          <vcount>3</vcount>
          <p>0 0 0 1 1 1 2 2 2</p>
        </polylist>
      </mesh>
    </geometry>
  </library_geometries>
  <library_visual_scenes><visual_scene id="Scene" name="Scene"><node name="PlaneNode" type="NODE"><instance_geometry url="#geo"/></node></visual_scene></library_visual_scenes>
  <scene><instance_visual_scene url="#Scene"/></scene>
</COLLADA>`), nil
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Meshes, 1)
				prim := asset.Meshes[0].Primitives[0]
				require.True(t, prim.Data.HasUVs(), "TexCoord0 should be present")
				require.NotNil(t, prim.Data.TexCoord1, "TexCoord1 should be present")
				assert.InDelta(t, float32(1.0), prim.Data.TexCoord0[1][0], 0.01)
				assert.InDelta(t, float32(1.5), prim.Data.TexCoord1[1][0], 0.01)
				assert.InDelta(t, float32(0.5), prim.Data.TexCoord1[0][0], 0.01)
			},
		},
		{
			"Lines",
			func() ([]byte, error) {
				return []byte(colladaWrap(`<lines count="1"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1</p></lines>`)), nil
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Meshes, 1)
				assert.Equal(t, ir.Lines, asset.Meshes[0].Primitives[0].Mode)
			},
		},
		{
			"Cameras",
			func() ([]byte, error) {
				return []byte(`<?xml version="1.0"?>
<COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema" version="1.4.1">
  <asset><up_axis>Y_UP</up_axis></asset>
  <library_cameras>
    <camera id="cam" name="Camera">
      <optics><technique_common><perspective>
        <yfov><float>60</float></yfov><znear><float>0.1</float></znear><zfar><float>1000</float></zfar><aspect_ratio><float>1.77</float></aspect_ratio>
      </perspective></technique_common></optics>
    </camera>
  </library_cameras>
  <library_geometries><geometry id="geo" name="Mesh"><mesh><source id="pos"><float_array id="pos-a" count="9">0 0 0 1 0 0 0 1 0</float_array><technique_common><accessor source="#pos-a" count="3" stride="3"><param name="X" type="float"/><param name="Y" type="float"/><param name="Z" type="float"/></accessor></technique_common></source><vertices id="verts"><input semantic="POSITION" source="#pos"/></vertices><triangles count="1"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2</p></triangles></mesh></geometry></library_geometries>
  <library_visual_scenes><visual_scene id="Scene" name="Scene"><node name="CamNode" type="NODE"><instance_camera url="#cam"/></node><node name="MeshNode" type="NODE"><instance_geometry url="#geo"/></node></visual_scene></library_visual_scenes>
  <scene><instance_visual_scene url="#Scene"/></scene>
</COLLADA>`), nil
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Cameras, 1)
				assert.Equal(t, "Camera", asset.Cameras[0].Name)
				assert.InDelta(t, 60.0, asset.Cameras[0].Perspective.FOV*180/3.14159265, 1.0)
			},
		},
		{
			"VertexColors",
			func() ([]byte, error) {
				return []byte(`<?xml version="1.0"?>
<COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema" version="1.4.1">
  <asset><up_axis>Y_UP</up_axis></asset>
  <library_geometries><geometry id="geo" name="ColorMesh"><mesh>
    <source id="pos"><float_array id="pos-a" count="9">0 0 0 1 0 0 0 1 0</float_array><technique_common><accessor source="#pos-a" count="3" stride="3"><param name="X" type="float"/><param name="Y" type="float"/><param name="Z" type="float"/></accessor></technique_common></source>
    <source id="col"><float_array id="col-a" count="12">1 0 0 1 0 1 0 1 0 0 1 1</float_array><technique_common><accessor source="#col-a" count="3" stride="4"><param name="R" type="float"/><param name="G" type="float"/><param name="B" type="float"/><param name="A" type="float"/></accessor></technique_common></source>
    <vertices id="verts"><input semantic="POSITION" source="#pos"/></vertices>
    <triangles count="1"><input semantic="VERTEX" source="#verts" offset="0"/><input semantic="COLOR" source="#col" offset="1"/><p>0 0 1 1 2 2</p></triangles>
  </mesh></geometry></library_geometries>
  <library_visual_scenes><visual_scene id="Scene" name="Scene"><node name="Node" type="NODE"><instance_geometry url="#geo"/></node></visual_scene></library_visual_scenes>
  <scene><instance_visual_scene url="#Scene"/></scene>
</COLLADA>`), nil
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Meshes, 1)
				prim := asset.Meshes[0].Primitives[0]
				require.NotNil(t, prim.Data.Colors0)
				assert.InDelta(t, float32(1), prim.Data.Colors0[0][0], 0.01)
			},
		},
		{
			"Lights",
			func() ([]byte, error) {
				return []byte(`<?xml version="1.0"?>
<COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema" version="1.4.1">
  <asset><up_axis>Y_UP</up_axis></asset>
  <library_lights>
    <light id="pt" name="PointLight"><technique_common><point><color>1 0.5 0</color></point></technique_common></light>
    <light id="dr" name="DirLight"><technique_common><directional><color>0 1 0</color></directional></technique_common></light>
    <light id="sp" name="SpotLight"><technique_common><spot><color>0 0 1</color><falloff_angle><float>45</float></falloff_angle></spot></technique_common></light>
  </library_lights>
  <library_geometries><geometry id="geo" name="M"><mesh><source id="pos"><float_array id="pos-a" count="9">0 0 0 1 0 0 0 1 0</float_array><technique_common><accessor source="#pos-a" count="3" stride="3"><param name="X" type="float"/><param name="Y" type="float"/><param name="Z" type="float"/></accessor></technique_common></source><vertices id="verts"><input semantic="POSITION" source="#pos"/></vertices><triangles count="1"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2</p></triangles></mesh></geometry></library_geometries>
  <library_visual_scenes>
    <visual_scene id="Scene" name="Scene">
      <node name="PLNode" type="NODE"><instance_light url="#pt"/></node>
      <node name="DLNode" type="NODE"><instance_light url="#dr"/></node>
      <node name="SLNode" type="NODE"><instance_light url="#sp"/></node>
      <node name="MNode" type="NODE"><instance_geometry url="#geo"/></node>
    </visual_scene>
  </library_visual_scenes>
  <scene><instance_visual_scene url="#Scene"/></scene>
</COLLADA>`), nil
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Lights, 3)
				assert.Equal(t, "PointLight", asset.Lights[0].Name)
				assert.NotNil(t, asset.Lights[0].Point)
				assert.InDelta(t, float32(0.5), asset.Lights[0].Color[1], 0.01)
				assert.Equal(t, "DirLight", asset.Lights[1].Name)
				assert.NotNil(t, asset.Lights[1].Directional)
				assert.Equal(t, "SpotLight", asset.Lights[2].Name)
				assert.NotNil(t, asset.Lights[2].Spot)
				assert.InDelta(t, float32(0), asset.Lights[2].Color[0], 0.01)
			},
		},
		{
			"Skinning",
			func() ([]byte, error) {
				return []byte(`<?xml version="1.0"?>
<COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema" version="1.4.1">
  <asset><up_axis>Y_UP</up_axis></asset>
  <library_geometries><geometry id="geo" name="SkinnedMesh"><mesh><source id="pos"><float_array id="pos-a" count="9">0 0 0 1 0 0 0 1 0</float_array><technique_common><accessor source="#pos-a" count="3" stride="3"><param name="X" type="float"/><param name="Y" type="float"/><param name="Z" type="float"/></accessor></technique_common></source><vertices id="verts"><input semantic="POSITION" source="#pos"/></vertices><triangles count="1"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2</p></triangles></mesh></geometry></library_geometries>
  <library_controllers>
    <controller id="skin-ctrl" name="SkinCtrl">
      <skin source="#geo">
        <source id="joints"><float_array id="joints-a" count="2">Root Child</float_array><technique_common><accessor source="#joints-a" count="2" stride="1"><param name="JOINT" type="name"/></accessor></technique_common></source>
        <source id="ibm"><float_array id="ibm-a" count="32">1 0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 1 0 0 0 0 1 0 0 0 0 1 0 0 -1 0 1</float_array><technique_common><accessor source="#ibm-a" count="2" stride="16"><param name="TRANSFORM" type="float4x4"/></accessor></technique_common></source>
        <source id="wt"><float_array id="wt-a" count="3">1.0 0.5 0.5</float_array><technique_common><accessor source="#wt-a" count="3" stride="1"><param name="WEIGHT" type="float"/></accessor></technique_common></source>
        <joints><input semantic="JOINT" source="#joints"/><input semantic="INV_BIND_MATRIX" source="#ibm"/></joints>
        <vertex_weights count="3"><input semantic="JOINT" offset="0" source="#joints"/><input semantic="WEIGHT" offset="1" source="#wt"/><vcount>1 1 1</vcount><v>0 0 1 1 0 2</v></vertex_weights>
      </skin>
    </controller>
  </library_controllers>
  <library_visual_scenes><visual_scene id="Scene" name="Scene"><node name="Root" type="JOINT"><node name="Child" type="JOINT"/></node><node name="MeshNode" type="NODE"><instance_geometry url="#geo"/></node></visual_scene></library_visual_scenes>
  <scene><instance_visual_scene url="#Scene"/></scene>
</COLLADA>`), nil
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Skeletons, 1)
				assert.Equal(t, "SkinCtrl", asset.Skeletons[0].Name)
				assert.Len(t, asset.Skeletons[0].Joints, 2)
				assert.Len(t, asset.Skeletons[0].InverseBindMatrices, 2)
				require.Len(t, asset.Meshes, 1)
				prim := asset.Meshes[0].Primitives[0]
				assert.NotNil(t, prim.Data.Joints0)
				assert.NotNil(t, prim.Data.Weights0)
			},
		},
		{
			"OrthoCam",
			func() ([]byte, error) {
				return []byte(`<?xml version="1.0"?>
<COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema" version="1.4.1">
  <asset><up_axis>Y_UP</up_axis></asset>
  <library_cameras><camera id="ocam" name="OrthoCamera"><optics><technique_common><orthographic><xmag><float>10</float></xmag><ymag><float>7.5</float></ymag><znear><float>0.1</float></znear><zfar><float>100</float></zfar></orthographic></technique_common></optics></camera></library_cameras>
  <library_geometries><geometry id="geo" name="M"><mesh><source id="pos"><float_array id="pos-a" count="9">0 0 0 1 0 0 0 1 0</float_array><technique_common><accessor source="#pos-a" count="3" stride="3"><param name="X" type="float"/><param name="Y" type="float"/><param name="Z" type="float"/></accessor></technique_common></source><vertices id="verts"><input semantic="POSITION" source="#pos"/></vertices><triangles count="1"><input semantic="VERTEX" source="#verts" offset="0"/><p>0 1 2</p></triangles></mesh></geometry></library_geometries>
  <library_visual_scenes><visual_scene id="Scene" name="Scene"><node name="CamNode" type="NODE"><instance_camera url="#ocam"/></node><node name="MNode" type="NODE"><instance_geometry url="#geo"/></node></visual_scene></library_visual_scenes>
  <scene><instance_visual_scene url="#Scene"/></scene>
</COLLADA>`), nil
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Cameras, 1)
				assert.Equal(t, "OrthoCamera", asset.Cameras[0].Name)
				assert.NotNil(t, asset.Cameras[0].Orthographic)
				assert.InDelta(t, float32(10), asset.Cameras[0].Orthographic.XMag, 0.1)
				assert.InDelta(t, float32(7.5), asset.Cameras[0].Orthographic.YMag, 0.1)
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
