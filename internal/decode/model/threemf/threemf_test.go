package threemf_test

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/hpinc/go3mf"
	"github.com/hpinc/go3mf/materials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/threemf"
	"github.com/gophics/ravenporter/ir"
)

func TestProbe(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"ValidPK", append([]byte("PK\x03\x04"), []byte("...[Content_Types].xml...")...), true},
		{"InvalidMagic", []byte("glTF"), false},
		{"TooShort", []byte("P"), false},
		{"Empty", nil, false},
	}
	dec := &threemf.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dec.Probe(bytes.NewReader(tt.data)))
		})
	}
}

func TestDecodeRejectsJunk(t *testing.T) {
	dec := &threemf.Decoder{}
	_, err := dec.Decode(bytes.NewReader([]byte("not a zip")), detect.DecodeOptions{})
	require.Error(t, err)
}

func TestUnitScale(t *testing.T) {
	tests := []struct {
		name string
		unit go3mf.Units
		want float64
	}{
		{"Micrometer", go3mf.UnitMicrometer, 0.000001},
		{"Millimeter", go3mf.UnitMillimeter, 0.001},
		{"Centimeter", go3mf.UnitCentimeter, 0.01},
		{"Inch", go3mf.UnitInch, 0.0254},
		{"Foot", go3mf.UnitFoot, 0.3048},
		{"Meter", go3mf.UnitMeter, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := minimalModel("u", [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, []go3mf.Triangle{{V1: 0, V2: 1, V3: 2}})
			model.Units = tt.unit
			asset := threemf.ConvertModelForTest(model)
			assert.InDelta(t, tt.want, asset.Unit, 0.0001)
		})
	}
}

func minimalModel(name string, verts [][3]float32, tris []go3mf.Triangle) *go3mf.Model {
	pts := make([]go3mf.Point3D, len(verts))
	for i, v := range verts {
		pts[i] = go3mf.Point3D(v)
	}
	return &go3mf.Model{
		Units: go3mf.UnitMillimeter,
		Resources: go3mf.Resources{
			Objects: []*go3mf.Object{{
				ID:   1,
				Name: name,
				Mesh: &go3mf.Mesh{
					Vertices:  go3mf.Vertices{Vertex: pts},
					Triangles: go3mf.Triangles{Triangle: tris},
				},
			}},
		},
	}
}

func TestThreeMFConvertAll(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *go3mf.Model
		check func(t *testing.T, asset *ir.Asset)
	}{
		{
			"MinimalModel",
			func() *go3mf.Model {
				return minimalModel("Cube", [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, []go3mf.Triangle{{V1: 0, V2: 1, V3: 2}})
			},
			func(t *testing.T, asset *ir.Asset) {
				assert.Equal(t, ir.Format3MF, asset.Metadata.SourceFormat)
				require.Len(t, asset.Meshes, 1)
				assert.Equal(t, "Cube", asset.Meshes[0].Name)
				prim := asset.Meshes[0].Primitives[0]
				assert.Equal(t, 3, prim.Data.VertexCount)
				assert.Len(t, prim.Data.Indices, 3)
				assert.Equal(t, [3]float32{1, 0, 0}, prim.Data.Positions[1])
				assert.Equal(t, uint32(0), prim.Data.Indices[0])
				require.Len(t, asset.Nodes, 1)
				assert.Equal(t, "Cube", asset.Nodes[0].Name)
				assert.Equal(t, 0, asset.Nodes[0].MeshIndex)
			},
		},
		{
			"VertexColors",
			func() *go3mf.Model {
				cg := &materials.ColorGroup{
					ID:     10,
					Colors: []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}},
				}
				return &go3mf.Model{
					Units: go3mf.UnitMillimeter,
					Resources: go3mf.Resources{
						Assets: []go3mf.Asset{cg},
						Objects: []*go3mf.Object{{
							ID:   1,
							Name: "ColorTri",
							Mesh: &go3mf.Mesh{
								Vertices:  go3mf.Vertices{Vertex: []go3mf.Point3D{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}},
								Triangles: go3mf.Triangles{Triangle: []go3mf.Triangle{{V1: 0, V2: 1, V3: 2, PID: 10, P1: 0, P2: 1, P3: 2}}},
							},
						}},
					},
				}
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Meshes, 1)
				prim := asset.Meshes[0].Primitives[0]
				require.Len(t, prim.Data.Colors0, 3)
				assert.InDelta(t, float32(1), prim.Data.Colors0[0][0], 0.01)
				assert.InDelta(t, float32(0), prim.Data.Colors0[0][1], 0.01)
				assert.InDelta(t, float32(1), prim.Data.Colors0[1][1], 0.01)
				assert.InDelta(t, float32(1), prim.Data.Colors0[2][2], 0.01)
			},
		},
		{
			"Materials",
			func() *go3mf.Model {
				bm := &go3mf.BaseMaterials{
					ID:        5,
					Materials: []go3mf.Base{{Name: "Red", Color: color.RGBA{255, 0, 0, 255}}},
				}
				return &go3mf.Model{
					Units: go3mf.UnitMillimeter,
					Resources: go3mf.Resources{
						Assets: []go3mf.Asset{bm},
						Objects: []*go3mf.Object{{
							ID:   1,
							Name: "MatMesh",
							PID:  5,
							Mesh: &go3mf.Mesh{
								Vertices:  go3mf.Vertices{Vertex: []go3mf.Point3D{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}},
								Triangles: go3mf.Triangles{Triangle: []go3mf.Triangle{{V1: 0, V2: 1, V3: 2}}},
							},
						}},
					},
				}
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Materials, 1)
				assert.Equal(t, "Red", asset.Materials[0].Name)
				assert.InDelta(t, float32(1.0), asset.Materials[0].BaseColorFactor[0], 0.01)
				assert.Equal(t, 0, asset.Meshes[0].Primitives[0].MaterialIndex)
			},
		},
		{
			"ComponentObject",
			func() *go3mf.Model {
				return &go3mf.Model{
					Units: go3mf.UnitMillimeter,
					Resources: go3mf.Resources{
						Objects: []*go3mf.Object{
							{
								ID:   1,
								Name: "Base",
								Mesh: &go3mf.Mesh{
									Vertices:  go3mf.Vertices{Vertex: []go3mf.Point3D{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}},
									Triangles: go3mf.Triangles{Triangle: []go3mf.Triangle{{V1: 0, V2: 1, V3: 2}}},
								},
							},
							{
								ID:   2,
								Name: "Assembly",
								Components: &go3mf.Components{
									Component: []*go3mf.Component{{ObjectID: 1}},
								},
							},
						},
					},
				}
			},
			func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			"NilMeshObject",
			func() *go3mf.Model {
				return &go3mf.Model{
					Units: go3mf.UnitMillimeter,
					Resources: go3mf.Resources{
						Objects: []*go3mf.Object{
							{
								ID:   1,
								Name: "NoMesh",
							},
						},
					},
				}
			},
			func(t *testing.T, asset *ir.Asset) {
				assert.Empty(t, asset.Meshes)
			},
		},
		{
			"Textures",
			func() *go3mf.Model {
				tex := &materials.Texture2D{
					ID:   20,
					Path: "/3D/Textures/diffuse.png",
				}
				tg := &materials.Texture2DGroup{
					ID:        30,
					TextureID: 20,
					Coords:    []materials.TextureCoord{{0, 0}, {1, 0}, {0.5, 1}},
				}
				return &go3mf.Model{
					Units: go3mf.UnitMillimeter,
					Resources: go3mf.Resources{
						Assets: []go3mf.Asset{tex, tg},
						Objects: []*go3mf.Object{{
							ID:   1,
							Name: "TexMesh",
							Mesh: &go3mf.Mesh{
								Vertices:  go3mf.Vertices{Vertex: []go3mf.Point3D{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}},
								Triangles: go3mf.Triangles{Triangle: []go3mf.Triangle{{V1: 0, V2: 1, V3: 2, PID: 30, P1: 0, P2: 1, P3: 2}}},
							},
						}},
					},
				}
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Meshes, 1)
				prim := asset.Meshes[0].Primitives[0]
				require.Len(t, prim.Data.TexCoord0, 3)
				assert.InDelta(t, float32(1), prim.Data.TexCoord0[1][0], 0.01)
				require.Len(t, asset.Textures, 1)
				require.Len(t, asset.Images, 1)
				assert.Equal(t, "/3D/Textures/diffuse.png", asset.Images[asset.Textures[0].ImageIndex].SourcePath)
			},
		},
		{
			"DefaultUnitScale",
			func() *go3mf.Model {
				model := minimalModel("u", [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, []go3mf.Triangle{{V1: 0, V2: 1, V3: 2}})
				model.Units = go3mf.Units(99)
				return model
			},
			func(t *testing.T, asset *ir.Asset) {
				assert.InDelta(t, 0.001, asset.Unit, 0.0001)
			},
		},
		{
			"EmptyMeshName",
			func() *go3mf.Model {
				return minimalModel("", [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, []go3mf.Triangle{{V1: 0, V2: 1, V3: 2}})
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Meshes, 1)
				assert.Equal(t, "3mf", asset.Meshes[0].Name)
			},
		},
		{
			"ColorOutOfBounds",
			func() *go3mf.Model {
				cg := &materials.ColorGroup{
					ID:     10,
					Colors: []color.RGBA{{255, 0, 0, 255}},
				}
				return &go3mf.Model{
					Units: go3mf.UnitMillimeter,
					Resources: go3mf.Resources{
						Assets: []go3mf.Asset{cg},
						Objects: []*go3mf.Object{{
							ID:   1,
							Name: "OOB",
							Mesh: &go3mf.Mesh{
								Vertices:  go3mf.Vertices{Vertex: []go3mf.Point3D{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}},
								Triangles: go3mf.Triangles{Triangle: []go3mf.Triangle{{V1: 0, V2: 1, V3: 2, PID: 10, P1: 0, P2: 99, P3: 99}}},
							},
						}},
					},
				}
			},
			func(t *testing.T, asset *ir.Asset) {
				require.Len(t, asset.Meshes, 1)
				prim := asset.Meshes[0].Primitives[0]
				require.Len(t, prim.Data.Colors0, 3)
				assert.InDelta(t, float32(1), prim.Data.Colors0[0][0], 0.01)
				assert.InDelta(t, float32(1), prim.Data.Colors0[1][0], 0.01)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.setup()
			asset := threemf.ConvertModelForTest(model)
			tt.check(t, asset)
		})
	}
}
