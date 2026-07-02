package models

import (
	"math"

	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type calcTangentSpaceStep struct{}

func (s *calcTangentSpaceStep) Name() string      { return "CalcTangentSpace" }
func (s *calcTangentSpaceStep) Flag() core.PPFlag { return core.PPCalcTangentSpace }

func (s *calcTangentSpaceStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if p.Mode != ir.Triangles || len(p.Data.Normals) == 0 || len(p.Data.TexCoord0) == 0 {
				continue
			}
			if len(p.Data.Tangents) == 0 {
				p.Data.Tangents = make([][4]float32, len(p.Data.Positions))
				calcTangents(&p.Data)
			}
		}
	}
	return asset, nil
}

//nolint:funlen // Splitting calculation loops is messier than monolithic struct iteration
func calcTangents(d *ir.MeshData) {
	tan1 := make([][3]float32, len(d.Positions))
	tan2 := make([][3]float32, len(d.Positions))

	const vertsPerTri = 3
	triCount := len(d.Positions) / vertsPerTri
	if d.HasIndices() {
		triCount = len(d.Indices) / vertsPerTri
	}

	processTri := func(i0, i1, i2 uint32) {
		v1, v2, v3 := d.Positions[i0], d.Positions[i1], d.Positions[i2]
		w1, w2, w3 := d.TexCoord0[i0], d.TexCoord0[i1], d.TexCoord0[i2]

		x1 := v2[0] - v1[0]
		x2 := v3[0] - v1[0]
		y1 := v2[1] - v1[1]
		y2 := v3[1] - v1[1]
		z1 := v2[2] - v1[2]
		z2 := v3[2] - v1[2]

		s1 := w2[0] - w1[0]
		s2 := w3[0] - w1[0]
		t1 := w2[1] - w1[1]
		t2 := w3[1] - w1[1]

		r := 1.0 / (s1*t2 - s2*t1)
		if mathx.IsNaN32(r) || math.IsInf(float64(r), 0) {
			r = 1.0
		}

		sdir := [3]float32{(t2*x1 - t1*x2) * r, (t2*y1 - t1*y2) * r, (t2*z1 - t1*z2) * r}
		tdir := [3]float32{(s1*x2 - s2*x1) * r, (s1*y2 - s2*y1) * r, (s1*z2 - s2*z1) * r}

		tan1[i0][0] += sdir[0]
		tan1[i0][1] += sdir[1]
		tan1[i0][2] += sdir[2]
		tan1[i1][0] += sdir[0]
		tan1[i1][1] += sdir[1]
		tan1[i1][2] += sdir[2]
		tan1[i2][0] += sdir[0]
		tan1[i2][1] += sdir[1]
		tan1[i2][2] += sdir[2]

		tan2[i0][0] += tdir[0]
		tan2[i0][1] += tdir[1]
		tan2[i0][2] += tdir[2]
		tan2[i1][0] += tdir[0]
		tan2[i1][1] += tdir[1]
		tan2[i1][2] += tdir[2]
		tan2[i2][0] += tdir[0]
		tan2[i2][1] += tdir[1]
		tan2[i2][2] += tdir[2]
	}

	if d.HasIndices() {
		for i := 0; i < triCount; i++ {
			processTri(d.Indices[i*vertsPerTri], d.Indices[i*vertsPerTri+1], d.Indices[i*vertsPerTri+2])
		}
	} else {
		for i := 0; i < triCount; i++ {
			processTri(uint32(i*vertsPerTri), uint32(i*vertsPerTri+1), uint32(i*vertsPerTri+2)) //nolint:mnd // native bounds natively
		}
	}

	for i := range d.Positions {
		n := d.Normals[i]
		t := tan1[i]

		dot := n[0]*t[0] + n[1]*t[1] + n[2]*t[2]
		tanX := t[0] - n[0]*dot
		tanY := t[1] - n[1]*dot
		tanZ := t[2] - n[2]*dot

		c := mathx.Cross3(n, t)
		handedness := c[0]*tan2[i][0] + c[1]*tan2[i][1] + c[2]*tan2[i][2]
		var w float32 = 1.0
		if handedness < 0.0 {
			w = -1.0
		}

		normT := mathx.Normalize3([3]float32{tanX, tanY, tanZ})
		d.Tangents[i] = [4]float32{normT[0], normT[1], normT[2], w}
	}
}
