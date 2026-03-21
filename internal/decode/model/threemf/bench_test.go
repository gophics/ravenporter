package threemf_test

import (
	"image/color"
	"testing"

	"github.com/hpinc/go3mf"

	"github.com/gophics/ravenporter/internal/decode/model/threemf"
)

func BenchmarkConvertModel(b *testing.B) {
	model := &go3mf.Model{
		Units: go3mf.UnitMillimeter,
		Resources: go3mf.Resources{
			Assets: []go3mf.Asset{
				&go3mf.BaseMaterials{
					ID:        1,
					Materials: []go3mf.Base{{Name: "mat", Color: color.RGBA{200, 100, 50, 255}}},
				},
			},
			Objects: []*go3mf.Object{{
				ID:   1,
				Name: "bench",
				PID:  1,
				Mesh: &go3mf.Mesh{
					Vertices: go3mf.Vertices{
						Vertex: []go3mf.Point3D{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}},
					},
					Triangles: go3mf.Triangles{
						Triangle: []go3mf.Triangle{
							{V1: 0, V2: 1, V3: 2},
							{V1: 1, V2: 3, V3: 2},
						},
					},
				},
			}},
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		threemf.ConvertModelForTest(model)
	}
}
