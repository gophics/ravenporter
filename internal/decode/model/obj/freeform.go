package obj

import (
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/internal/spline"
	"github.com/gophics/ravenporter/ir"
)

const (
	defaultCurveRes = 32
	defaultSurfRes  = 16
)

type csType int

const (
	csBezier csType = iota
	csBSpline
	csCardinal
	csTaylor
	csBasisMatrix
)

type freeformState struct {
	typ      csType
	rational bool
	degU     int
	degV     int
	bmatU    []float32
	bmatV    []float32
	stepU    int
	stepV    int
}

type curve3D struct {
	u0, u1  float32
	ctrlIdx []int
	knotsU  []float32
	state   freeformState
}

type curve2D struct {
	ctrlIdx []int
	knotsU  []float32
	state   freeformState
}

type surfVert struct{ pos, uv, norm int }

type surface struct {
	s0, s1, t0, t1 float32
	ctrlPts        []surfVert
	knotsU, knotsV []float32
	trims          []trimLoop
	holes          []trimLoop
	state          freeformState
}

type trimSeg struct {
	u0, u1   float32
	curv2Idx int
}

type trimLoop struct{ segments []trimSeg }

func evalCurve(state freeformState, pts [][4]float32, knots []float32, t float32) [4]float32 {
	switch state.typ {
	case csBezier:
		return spline.EvalBezier(state.degU, pts, t)
	case csBSpline:
		return spline.EvalBSpline(state.degU, pts, knots, t)
	case csCardinal:
		return spline.EvalCardinal(pts, t)
	case csTaylor:
		return spline.EvalTaylor(pts, t)
	case csBasisMatrix:
		return spline.EvalBasisMatrix(state.degU, state.bmatU, pts, t)
	}
	return [4]float32{}
}

const maxStackCtrlPts = 16

func gatherCtrlPts(indices []int, positions [][3]float32, _ bool) [][4]float32 {
	n := len(indices)
	var stack [maxStackCtrlPts][4]float32
	var pts [][4]float32
	if n <= maxStackCtrlPts {
		pts = stack[:n]
	} else {
		pts = make([][4]float32, n)
	}
	for i, idx := range indices {
		ri := idx - 1
		if idx < 0 {
			ri = len(positions) + idx
		}
		if ri < 0 || ri >= len(positions) {
			continue
		}
		pts[i] = spline.ToHomogeneous(positions[ri], 1.0)
	}
	return pts
}

func tessellate3DCurve(c *curve3D, data *objData) [][3]float32 {
	pts := gatherCtrlPts(c.ctrlIdx, data.positions, c.state.rational)
	if len(pts) == 0 {
		return nil
	}

	res := defaultCurveRes
	result := make([][3]float32, 0, res+1)

	for i := 0; i <= res; i++ {
		t := c.u0 + float32(i)/float32(res)*(c.u1-c.u0)
		p := evalCurve(c.state, pts, c.knotsU, t)
		result = append(result, spline.FromHomogeneous(p))
	}
	return result
}

func buildCurveMesh(parsed *objData, c curve3D) *ir.Mesh {
	pts := tessellate3DCurve(&c, parsed)
	if len(pts) < 2 { //nolint:mnd // 2 points to form a line
		return nil
	}

	indices := make([]uint32, 0, len(pts)*2) //nolint:mnd // 2 indices per line
	for i := 0; i < len(pts)-1; i++ {
		indices = append(indices, uint32(i), uint32(i+1)) //nolint:gosec // bounded
	}

	return &ir.Mesh{
		Name: "freeform_curve",
		Primitives: []ir.Primitive{{
			Mode:          ir.Lines,
			MaterialIndex: ir.NoIndex,
			Data: ir.MeshData{
				VertexCount: len(pts),
				Positions:   pts,
				Indices:     indices,
			},
		}},
	}
}

func gatherSurfCtrlPts(verts []surfVert, positions [][3]float32, _ bool) [][4]float32 {
	n := len(verts)
	var stack [maxStackCtrlPts][4]float32
	var pts [][4]float32
	if n <= maxStackCtrlPts {
		pts = stack[:n]
	} else {
		pts = make([][4]float32, n)
	}
	for i, sv := range verts {
		ri := sv.pos - 1
		if sv.pos < 0 {
			ri = len(positions) + sv.pos
		}
		if ri < 0 || ri >= len(positions) {
			continue
		}
		pts[i] = spline.ToHomogeneous(positions[ri], 1.0)
	}
	return pts
}

func evalSurface(state freeformState, grid [][4]float32, nu, nv int,
	knotsU, knotsV []float32, u, v float32, row, col [][4]float32) [3]float32 {
	for j := range nv {
		copy(row[:nu], grid[j*nu:(j+1)*nu])
		col[j] = evalCurve(state, row[:nu], knotsU, u)
	}

	vState := state
	vState.degU = state.degV
	if state.bmatV != nil {
		vState.bmatU = state.bmatV
	}
	p := evalCurve(vState, col[:nv], knotsV, v)
	return spline.FromHomogeneous(p)
}

func evalTrimCurve(c *curve2D, paramVerts [][3]float32) [][2]float32 {
	if len(c.ctrlIdx) == 0 {
		return nil
	}
	n := len(c.ctrlIdx)
	var stack [maxStackCtrlPts][4]float32
	var pts [][4]float32
	if n <= maxStackCtrlPts {
		pts = stack[:n]
	} else {
		pts = make([][4]float32, n)
	}
	for i, idx := range c.ctrlIdx {
		ri := idx - 1
		if idx < 0 {
			ri = len(paramVerts) + idx
		}
		if ri < 0 || ri >= len(paramVerts) {
			continue
		}
		pv := paramVerts[ri]
		w := pv[2]
		if w == 0 {
			w = 1
		}
		pts[i] = [4]float32{pv[0] * w, pv[1] * w, 0, w}
	}

	res := defaultCurveRes
	result := make([][2]float32, 0, res+1)
	for i := 0; i <= res; i++ {
		t := float32(i) / float32(res)
		p := evalCurve(c.state, pts, c.knotsU, t)
		p3 := spline.FromHomogeneous(p)
		result = append(result, [2]float32{p3[0], p3[1]}) //nolint:gosec // 2D projection
	}
	return result
}

func isInsideTrims(u, v float32, s *surface, curves2D []curve2D, paramVerts [][3]float32) bool {
	if len(s.trims) == 0 {
		return true
	}

	inside := false
	for _, tl := range s.trims {
		poly := buildTrimPoly(tl, curves2D, paramVerts)
		if spline.WindingNumber(u, v, poly) != 0 {
			inside = true
			break
		}
	}
	if !inside {
		return false
	}

	for _, hl := range s.holes {
		poly := buildTrimPoly(hl, curves2D, paramVerts)
		if spline.WindingNumber(u, v, poly) != 0 {
			return false
		}
	}
	return true
}

func buildTrimPoly(tl trimLoop, curves2D []curve2D, paramVerts [][3]float32) [][2]float32 {
	var poly [][2]float32
	for _, seg := range tl.segments {
		idx := seg.curv2Idx - 1
		if idx < 0 || idx >= len(curves2D) {
			continue
		}
		pts := evalTrimCurve(&curves2D[idx], paramVerts)
		poly = append(poly, pts...)
	}
	return poly
}

func buildSurfaceMesh(parsed *objData, s surface) *ir.Mesh {
	pts := gatherSurfCtrlPts(s.ctrlPts, parsed.positions, s.state.rational)
	if len(pts) == 0 {
		return nil
	}

	nu, nv := spline.ComputeSurfDims(s.knotsU, s.knotsV, s.state.degU, s.state.degV, len(pts))
	if nu <= 0 || nv <= 0 {
		return nil
	}

	resU, resV := defaultSurfRes, defaultSurfRes
	gridW, gridH := resU+1, resV+1

	positions := make([][3]float32, 0, gridW*gridH)
	rowBuf := make([][4]float32, nu)
	colBuf := make([][4]float32, nv)
	normals := make([][3]float32, 0, gridW*gridH)
	uvs := make([][2]float32, 0, gridW*gridH)
	validIdx := make([]int, gridW*gridH)

	count := 0
	for jj := range gridH {
		v := s.t0 + float32(jj)/float32(resV)*(s.t1-s.t0)
		for ii := range gridW {
			u := s.s0 + float32(ii)/float32(resU)*(s.s1-s.s0)
			gi := jj*gridW + ii

			if !isInsideTrims(u, v, &s, parsed.curves2D, parsed.paramVerts) {
				validIdx[gi] = -1
				continue
			}

			pos := evalSurface(s.state, pts, nu, nv, s.knotsU, s.knotsV, u, v, rowBuf, colBuf)
			positions = append(positions, pos)
			uvs = append(uvs, [2]float32{
				(u - s.s0) / (s.s1 - s.s0),
				(v - s.t0) / (s.t1 - s.t0),
			})
			validIdx[gi] = count
			count++
		}
	}

	if count < 3 { //nolint:mnd // 3 points form a face
		return nil
	}

	computeSurfNormals(positions, &normals, validIdx, gridW, gridH)

	var indices []uint32
	for jj := range resV {
		for ii := range resU {
			i00 := validIdx[jj*gridW+ii]
			i10 := validIdx[jj*gridW+ii+1]
			i01 := validIdx[(jj+1)*gridW+ii]
			i11 := validIdx[(jj+1)*gridW+ii+1]

			if i00 >= 0 && i10 >= 0 && i01 >= 0 {
				indices = append(indices,
					uint32(i00), uint32(i10), uint32(i01)) //nolint:gosec // bounded
			}
			if i10 >= 0 && i11 >= 0 && i01 >= 0 {
				indices = append(indices,
					uint32(i10), uint32(i11), uint32(i01)) //nolint:gosec // bounded
			}
		}
	}

	if len(indices) == 0 {
		return nil
	}

	return &ir.Mesh{
		Name: "freeform_surface",
		Primitives: []ir.Primitive{{
			Mode:          ir.Triangles,
			MaterialIndex: ir.NoIndex,
			Data: ir.MeshData{
				VertexCount: len(positions),
				Positions:   positions,
				Normals:     normals,
				TexCoord0:   uvs,
				Indices:     indices,
			},
		}},
	}
}

func computeSurfNormals(positions [][3]float32, normals *[][3]float32, validIdx []int, gridW, gridH int) {
	*normals = (*normals)[:0]
	for range positions {
		*normals = append(*normals, [3]float32{0, 1, 0})
	}

	for jj := range gridH - 1 {
		for ii := range gridW - 1 {
			ic := validIdx[jj*gridW+ii]
			iRight := validIdx[jj*gridW+ii+1]
			iUp := validIdx[(jj+1)*gridW+ii]
			if ic < 0 || iRight < 0 || iUp < 0 {
				continue
			}

			du := mathx.Sub3(positions[iRight], positions[ic])
			dv := mathx.Sub3(positions[iUp], positions[ic])
			n := mathx.Cross3(du, dv)
			n = mathx.Normalize3(n)
			(*normals)[ic] = n
		}
	}
}
