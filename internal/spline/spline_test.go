package spline

import (
	"math"
	"testing"
)

func h(x, y, z float32) [4]float32 { return [4]float32{x, y, z, 1} }

func assertNear(t *testing.T, label string, got, want float32, tol float64) {
	t.Helper()
	if math.Abs(float64(got-want)) > tol {
		t.Errorf("%s: got %v, want %v (tol %v)", label, got, want, tol)
	}
}

func TestToFromHomogeneous(t *testing.T) {
	tests := []struct {
		name string
		pos  [3]float32
		w    float32
		want [4]float32
		back [3]float32
	}{
		{"unit weight", [3]float32{1, 2, 3}, 1, [4]float32{1, 2, 3, 1}, [3]float32{1, 2, 3}},
		{"weight 2", [3]float32{1, 2, 3}, 2, [4]float32{2, 4, 6, 2}, [3]float32{1, 2, 3}},
		{"weight 0.5", [3]float32{4, 6, 8}, 0.5, [4]float32{2, 3, 4, 0.5}, [3]float32{4, 6, 8}},
		{"zero weight", [3]float32{1, 2, 3}, 0, [4]float32{0, 0, 0, 0}, [3]float32{0, 0, 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ToHomogeneous(tc.pos, tc.w)
			if got != tc.want {
				t.Errorf("ToHomogeneous(%v, %v) = %v, want %v", tc.pos, tc.w, got, tc.want)
			}
			back := FromHomogeneous(got)
			for i := range 3 {
				assertNear(t, "round-trip", back[i], tc.back[i], 1e-5)
			}
		})
	}
}

func TestFromHomogeneousW1(t *testing.T) {
	p := [4]float32{5, 6, 7, 1}
	got := FromHomogeneous(p)
	want := [3]float32{5, 6, 7}
	if got != want {
		t.Errorf("FromHomogeneous(w=1) = %v, want %v", got, want)
	}
}

func TestEvalBezier(t *testing.T) {
	tests := []struct {
		name         string
		deg          int
		pts          [][4]float32
		t            float32
		wantX, wantY float32
	}{
		{"linear t=0", 1, [][4]float32{h(0, 0, 0), h(4, 0, 0)}, 0.0, 0, 0},
		{"linear t=1", 1, [][4]float32{h(0, 0, 0), h(4, 0, 0)}, 1.0, 4, 0},
		{"linear t=0.5", 1, [][4]float32{h(0, 0, 0), h(4, 0, 0)}, 0.5, 2, 0},
		{"quadratic t=0", 2, [][4]float32{h(0, 0, 0), h(2, 4, 0), h(4, 0, 0)}, 0.0, 0, 0},
		{"quadratic t=1", 2, [][4]float32{h(0, 0, 0), h(2, 4, 0), h(4, 0, 0)}, 1.0, 4, 0},
		{"quadratic t=0.5", 2, [][4]float32{h(0, 0, 0), h(2, 4, 0), h(4, 0, 0)}, 0.5, 2, 2},
		{"cubic t=0", 3, [][4]float32{h(0, 0, 0), h(1, 2, 0), h(3, 2, 0), h(4, 0, 0)}, 0.0, 0, 0},
		{"cubic t=1", 3, [][4]float32{h(0, 0, 0), h(1, 2, 0), h(3, 2, 0), h(4, 0, 0)}, 1.0, 4, 0},
		{"cubic midpoint", 3, [][4]float32{h(0, 0, 0), h(1, 2, 0), h(3, 2, 0), h(4, 0, 0)}, 0.5, 2, 1.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalBezier(tc.deg, tc.pts, tc.t)
			p := FromHomogeneous(got)
			assertNear(t, "X", p[0], tc.wantX, 1e-4)
			assertNear(t, "Y", p[1], tc.wantY, 1e-4)
		})
	}
}

func TestEvalBSpline(t *testing.T) {
	pts := [][4]float32{h(0, 0, 0), h(1, 1, 0), h(2, 0, 0), h(3, 1, 0)}
	knots := []float32{0, 0, 0, 0, 1, 1, 1, 1}
	tests := []struct {
		name         string
		t            float32
		wantX, wantY float32
	}{
		{"t=0 start", 0.0, 0, 0},
		{"t=1 end", 1.0, 3, 1},
		{"t=0.5 mid", 0.5, 1.5, 0.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalBSpline(3, pts, knots, tc.t)
			p := FromHomogeneous(got)
			assertNear(t, "X", p[0], tc.wantX, 1e-3)
			assertNear(t, "Y", p[1], tc.wantY, 1e-3)
		})
	}
}

func TestEvalCardinal(t *testing.T) {
	pts := [][4]float32{h(0, 0, 0), h(1, 2, 0), h(2, 0, 0), h(3, 2, 0)}
	tests := []struct {
		name  string
		t     float32
		wantX float32
	}{
		{"t=0 passes through P1", 0.0, 1},
		{"t=1 passes through P2", 1.0, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalCardinal(pts, tc.t)
			p := FromHomogeneous(got)
			assertNear(t, "X", p[0], tc.wantX, 1e-4)
		})
	}
}

func TestEvalCardinalEdgeCases(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := EvalCardinal(nil, 0.5)
		if got != ([4]float32{}) {
			t.Errorf("expected zero, got %v", got)
		}
	})
	t.Run("single point", func(t *testing.T) {
		p := [4]float32{1, 2, 3, 1}
		got := EvalCardinal([][4]float32{p}, 0.5)
		if got != p {
			t.Errorf("expected %v, got %v", p, got)
		}
	})
}

func TestEvalTaylor(t *testing.T) {
	// Taylor operates on raw [4]float32 via Horner: result = c0 + c1*t + c2*t^2
	// Use raw coefficients (not homogeneous points) for predictable output.
	pts := [][4]float32{{1, 0, 0, 0}, {2, 0, 0, 0}, {3, 0, 0, 0}}
	tests := []struct {
		name  string
		t     float32
		wantX float32
	}{
		{"t=0", 0.0, 1},
		{"t=1", 1.0, 6}, // 1 + 2 + 3 = 6
		{"t=0.5", 0.5, 2.75},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalTaylor(pts, tc.t)
			assertNear(t, "X", got[0], tc.wantX, 1e-4)
		})
	}
}

func TestEvalTaylorEmpty(t *testing.T) {
	got := EvalTaylor(nil, 0.5)
	if got != ([4]float32{}) {
		t.Errorf("expected zero, got %v", got)
	}
}

func TestEvalBasisMatrix(t *testing.T) {
	// Identity-like 2x2 matrix: f(t) = (1-t)*P0 + t*P1
	bmat := []float32{1, 0, -1, 1}
	pts := [][4]float32{h(0, 0, 0), h(4, 0, 0)}
	tests := []struct {
		name  string
		t     float32
		wantX float32
	}{
		{"t=0", 0.0, 0},
		{"t=1", 1.0, 4},
		{"t=0.5", 0.5, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalBasisMatrix(1, bmat, pts, tc.t)
			p := FromHomogeneous(got)
			assertNear(t, "X", p[0], tc.wantX, 1e-4)
		})
	}
}

func TestEvalBasisMatrixShortMatrix(t *testing.T) {
	pts := [][4]float32{h(5, 0, 0), h(10, 0, 0)}
	got := EvalBasisMatrix(1, []float32{1}, pts, 0.5)
	p := FromHomogeneous(got)
	assertNear(t, "fallback to P0", p[0], 5, 1e-4)
}

func TestWindingNumber(t *testing.T) {
	square := [][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	tests := []struct {
		name   string
		px, py float32
		poly   [][2]float32
		inside bool
	}{
		{"center inside", 0.5, 0.5, square, true},
		{"outside right", 2, 0.5, square, false},
		{"outside above", 0.5, 2, square, false},
		{"outside left", -1, 0.5, square, false},
		{"on boundary", 0, 0, square, true},
		{"degenerate 2pts", 0.5, 0.5, [][2]float32{{0, 0}, {1, 1}}, false},
		{"triangle center", 0.3, 0.3, [][2]float32{{0, 0}, {1, 0}, {0, 1}}, true},
		{"triangle outside", 0.8, 0.8, [][2]float32{{0, 0}, {1, 0}, {0, 1}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wn := WindingNumber(tc.px, tc.py, tc.poly)
			gotInside := wn != 0
			if gotInside != tc.inside {
				t.Errorf("WindingNumber(%v, %v) = %d, inside=%v, want inside=%v",
					tc.px, tc.py, wn, gotInside, tc.inside)
			}
		})
	}
}

func TestComputeSurfDims(t *testing.T) {
	tests := []struct {
		name         string
		knotsU       []float32
		knotsV       []float32
		degU, degV   int
		total        int
		wantU, wantV int
	}{
		{
			"from knots",
			[]float32{0, 0, 0, 0, 1, 1, 1, 1},
			[]float32{0, 0, 0, 0, 1, 1, 1, 1},
			3, 3, 16, 4, 4,
		},
		{
			"sqrt fallback 4",
			[]float32{0, 1}, []float32{0, 1},
			1, 1, 4, 2, 2,
		},
		{
			"sqrt fallback 9",
			nil, nil,
			1, 1, 9, 3, 3,
		},
		{
			"non-square fallback",
			nil, nil,
			1, 1, 6, 0, 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nu, nv := ComputeSurfDims(tc.knotsU, tc.knotsV, tc.degU, tc.degV, tc.total)
			if nu != tc.wantU || nv != tc.wantV {
				t.Errorf("got (%d, %d), want (%d, %d)", nu, nv, tc.wantU, tc.wantV)
			}
		})
	}
}

func BenchmarkEvalBezierCubic(b *testing.B) {
	pts := [][4]float32{h(0, 0, 0), h(1, 2, 0), h(3, 2, 0), h(4, 0, 0)}
	b.ReportAllocs()
	for b.Loop() {
		EvalBezier(3, pts, 0.5)
	}
}

func BenchmarkEvalBSplineCubic(b *testing.B) {
	pts := [][4]float32{h(0, 0, 0), h(1, 1, 0), h(2, 0, 0), h(3, 1, 0)}
	knots := []float32{0, 0, 0, 0, 1, 1, 1, 1}
	b.ReportAllocs()
	for b.Loop() {
		EvalBSpline(3, pts, knots, 0.5)
	}
}

func BenchmarkEvalCardinal(b *testing.B) {
	pts := [][4]float32{h(0, 0, 0), h(1, 2, 0), h(2, 0, 0), h(3, 2, 0)}
	b.ReportAllocs()
	for b.Loop() {
		EvalCardinal(pts, 0.5)
	}
}

func BenchmarkWindingNumber(b *testing.B) {
	square := [][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	b.ReportAllocs()
	for b.Loop() {
		WindingNumber(0.5, 0.5, square)
	}
}
