package obj_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/obj"
	"github.com/gophics/ravenporter/ir"
)

func decodeString(t *testing.T, src string) *ir.Asset {
	t.Helper()
	dec := &obj.Decoder{}
	opts := detect.DecodeOptions{}
	opts.Sanitize()
	scene, err := dec.Decode(strings.NewReader(src), opts)
	require.NoError(t, err)
	require.NotNil(t, scene)
	return scene
}

func TestFreeformParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, asset *ir.Asset)
	}{
		{
			name: "BezierCurve",
			input: `v 0 0 0
v 1 1 0
v 2 1 0
v 3 0 0
cstype bezier
deg 3
curv 0.0 1.0 1 2 3 4
parm u 0.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
				found := false
				for _, m := range asset.Meshes {
					for _, p := range m.Primitives {
						if p.Mode == ir.Lines {
							found = true
							assert.Greater(t, p.Data.VertexCount, 2)
						}
					}
				}
				assert.True(t, found, "expected a Lines primitive from bezier curve")
			},
		},
		{
			name: "BSplineCurve",
			input: `v 0 0 0
v 0.5 1 0
v 1.5 1 0
v 2 0 0
cstype bspline
deg 3
curv 0.0 1.0 1 2 3 4
parm u 0.0 0.0 0.0 0.0 1.0 1.0 1.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
				found := false
				for _, m := range asset.Meshes {
					for _, p := range m.Primitives {
						if p.Mode == ir.Lines {
							found = true
						}
					}
				}
				assert.True(t, found, "expected Lines primitive from bspline curve")
			},
		},
		{
			name: "RationalBSplineCurve",
			input: `v 0 0 0
v 1 1 0
v 2 0 0
cstype rat bspline
deg 2
curv 0.0 1.0 1 2 3
parm u 0.0 0.0 0.0 1.0 1.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			name: "BezierSurface",
			input: `v 0 0 0
v 1 0 0
v 2 0 0
v 3 0 0
v 0 0 1
v 1 1 1
v 2 1 1
v 3 0 1
v 0 0 2
v 1 1 2
v 2 1 2
v 3 0 2
v 0 0 3
v 1 0 3
v 2 0 3
v 3 0 3
cstype bezier
deg 3 3
surf 0.0 1.0 0.0 1.0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16
parm u 0.0 1.0
parm v 0.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
				found := false
				for _, m := range asset.Meshes {
					for _, p := range m.Primitives {
						if p.Mode != ir.Triangles {
							continue
						}
						found = true
						assert.Greater(t, p.Data.VertexCount, 3)
						assert.Greater(t, len(p.Data.Indices), 0)
						assert.Equal(t, len(p.Data.Normals), p.Data.VertexCount)
						assert.Equal(t, len(p.Data.TexCoord0), p.Data.VertexCount)
					}
				}
				assert.True(t, found, "expected Triangles primitive from bezier surface")
			},
		},
		{
			name: "BSplineSurface",
			input: `v 0 0 0
v 1 0 0
v 2 0 0
v 3 0 0
v 0 0 1
v 1 1 1
v 2 1 1
v 3 0 1
v 0 0 2
v 1 1 2
v 2 1 2
v 3 0 2
v 0 0 3
v 1 0 3
v 2 0 3
v 3 0 3
cstype bspline
deg 3 3
surf 0.0 1.0 0.0 1.0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16
parm u 0.0 0.0 0.0 0.0 1.0 1.0 1.0 1.0
parm v 0.0 0.0 0.0 0.0 1.0 1.0 1.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			name: "MixedPolyAndFreeform",
			input: `v 0 0 0
v 1 0 0
v 0 1 0
v 1 1 0
f 1 2 3
cstype bezier
deg 1
curv 0.0 1.0 1 2
parm u 0.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 2)
				hasTri := false
				hasLine := false
				for _, m := range asset.Meshes {
					for _, p := range m.Primitives {
						if p.Mode == ir.Triangles {
							hasTri = true
						}
						if p.Mode == ir.Lines {
							hasLine = true
						}
					}
				}
				assert.True(t, hasTri, "expected uace mesh")
				assert.True(t, hasLine, "expected curve mesh")
			},
		},
		{
			name:  "LineContinuation",
			input: "v 0 0 0\nv 1 0 0\nv 2 0 0\nv 3 0 0\ncstype bezier\ndeg 3\ncurv 0.0 1.0 \\\n1 2 \\\n3 4\nparm u 0.0 1.0\nend\n",
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			name: "NegativeIndices",
			input: `v 0 0 0
v 1 1 0
v 2 0 0
cstype bezier
deg 2
curv 0.0 1.0 -3 -2 -1
parm u 0.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			name: "TrimmedSurface",
			input: `v 0 0 0
v 1 0 0
v 0 0 1
v 1 0 1
vp 0.1 0.1
vp 0.9 0.1
vp 0.9 0.9
vp 0.1 0.9
cstype bezier
deg 1
curv2 1 2
parm u 0.0 1.0
end
curv2 2 3
parm u 0.0 1.0
end
curv2 3 4
parm u 0.0 1.0
end
curv2 4 1
parm u 0.0 1.0
end
deg 1 1
surf 0.0 1.0 0.0 1.0 1 2 3 4
parm u 0.0 1.0
parm v 0.0 1.0
trim 0.0 1.0 1 0.0 1.0 2 0.0 1.0 3 0.0 1.0 4
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			name: "CardinalCurve",
			input: `v 0 0 0
v 1 1 0
v 2 0 0
v 3 1 0
cstype cardinal
deg 3
curv 0.0 1.0 1 2 3 4
parm u 0.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			name: "TaylorCurve",
			input: `v 1 0 0
v 0 1 0
v -1 0 0
cstype taylor
deg 2
curv 0.0 1.0 1 2 3
parm u 0.0 1.0
end
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
		{
			name: "ParameterVertices",
			input: `v 0 0 0
v 1 0 0
v 0 1 0
vp 0.5
vp 0.5 0.5
vp 0.5 0.5 2.0
f 1 2 3
`,
			check: func(t *testing.T, asset *ir.Asset) {
				require.GreaterOrEqual(t, len(asset.Meshes), 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := &obj.Decoder{}
			opts := detect.DecodeOptions{}
			opts.Sanitize()
			scene, err := dec.Decode(strings.NewReader(tc.input), opts)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.check != nil {
				tc.check(t, scene)
			}
		})
	}
}

func TestEvalBezierEndpoints(t *testing.T) {
	pts := [][4]float32{
		{0, 0, 0, 1},
		{1, 2, 0, 1},
		{3, 2, 0, 1},
		{4, 0, 0, 1},
	}
	tests := []struct {
		name         string
		t            float32
		wantX, wantY float32
	}{
		{"t=0 start", 0.0, 0.0, 0.0},
		{"t=1 end", 1.0, 4.0, 0.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Access internal via integration test: parse a curve and check positions
			src := "v 0 0 0\nv 1 2 0\nv 3 2 0\nv 4 0 0\n" +
				"cstype bezier\ndeg 3\ncurv 0.0 1.0 1 2 3 4\nparm u 0.0 1.0\nend\n"
			asset := decodeString(t, src)
			require.GreaterOrEqual(t, len(asset.Meshes), 1)

			var positions [][3]float32
			for _, m := range asset.Meshes {
				for _, p := range m.Primitives {
					if p.Mode == ir.Lines {
						positions = p.Data.Positions
					}
				}
			}
			require.NotEmpty(t, positions)

			_ = pts // referenced for documentation

			uirst := positions[0]
			last := positions[len(positions)-1]
			assert.InDelta(t, 0.0, uirst[0], 0.01, "uirst X")
			assert.InDelta(t, 0.0, uirst[1], 0.01, "uirst Y")
			assert.InDelta(t, 4.0, last[0], 0.01, "last X")
			assert.InDelta(t, 0.0, last[1], 0.01, "last Y")
		})
	}
}

func TestSurfaceCorners(t *testing.T) {
	src := `v 0 0 0
v 1 0 0
v 0 0 1
v 1 0 1
cstype bezier
deg 1 1
surf 0.0 1.0 0.0 1.0 1 2 3 4
parm u 0.0 1.0
parm v 0.0 1.0
end
`
	asset := decodeString(t, src)
	require.GreaterOrEqual(t, len(asset.Meshes), 1)

	var positions [][3]float32
	for _, m := range asset.Meshes {
		for _, p := range m.Primitives {
			if p.Mode == ir.Triangles {
				positions = p.Data.Positions
			}
		}
	}
	require.NotEmpty(t, positions)

	hasCorner := func(x, y, z float32) bool {
		for _, p := range positions {
			if math.Abs(float64(p[0]-x)) < 0.01 && math.Abs(float64(p[1]-y)) < 0.01 && math.Abs(float64(p[2]-z)) < 0.01 {
				return true
			}
		}
		return false
	}

	assert.True(t, hasCorner(0, 0, 0), "corner (0,0,0)")
	assert.True(t, hasCorner(1, 0, 0), "corner (1,0,0)")
	assert.True(t, hasCorner(0, 0, 1), "corner (0,0,1)")
	assert.True(t, hasCorner(1, 0, 1), "corner (1,0,1)")
}

func TestSurfaceNormalsAndUVs(t *testing.T) {
	src := `v 0 0 0
v 1 0 0
v 0 0 1
v 1 0 1
cstype bezier
deg 1 1
surf 0.0 1.0 0.0 1.0 1 2 3 4
parm u 0.0 1.0
parm v 0.0 1.0
end
`
	asset := decodeString(t, src)
	require.GreaterOrEqual(t, len(asset.Meshes), 1)

	for _, m := range asset.Meshes {
		for _, p := range m.Primitives {
			if p.Mode == ir.Triangles {
				assert.Equal(t, len(p.Data.Normals), p.Data.VertexCount, "normals count must match vertices")
				assert.Equal(t, len(p.Data.TexCoord0), p.Data.VertexCount, "UV count must match vertices")

				for _, uv := range p.Data.TexCoord0 {
					assert.GreaterOrEqual(t, uv[0], float32(0.0))
					assert.LessOrEqual(t, uv[0], float32(1.0))
					assert.GreaterOrEqual(t, uv[1], float32(0.0))
					assert.LessOrEqual(t, uv[1], float32(1.0))
				}
			}
		}
	}
}

func TestMultipleCurvesAndSurfaces(t *testing.T) {
	src := `v 0 0 0
v 1 0 0
v 2 0 0
v 3 0 0
v 0 0 1
v 1 1 1
v 2 1 1
v 3 0 1
v 0 0 2
v 1 1 2
v 2 1 2
v 3 0 2
v 0 0 3
v 1 0 3
v 2 0 3
v 3 0 3
cstype bezier
deg 3
curv 0.0 1.0 1 2 3 4
parm u 0.0 1.0
end
curv 0.0 1.0 5 6 7 8
parm u 0.0 1.0
end
deg 3 3
surf 0.0 1.0 0.0 1.0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16
parm u 0.0 1.0
parm v 0.0 1.0
end
`
	asset := decodeString(t, src)
	curves := 0
	surfaces := 0
	for _, m := range asset.Meshes {
		for _, p := range m.Primitives {
			if p.Mode == ir.Lines {
				curves++
			}
			if p.Mode == ir.Triangles {
				surfaces++
			}
		}
	}
	assert.Equal(t, 2, curves, "expected 2 curve meshes")
	assert.Equal(t, 1, surfaces, "expected 1 surface mesh")
}
