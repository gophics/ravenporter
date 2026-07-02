// Package spline provides basis curve/surface evaluation functions
// shared across decoders that handle free-form geometry (OBJ, FBX NURBS, USD, STEP).
package spline

import "math"

const MaxStackPts = 16

func ToHomogeneous(pos [3]float32, w float32) [4]float32 {
	return [4]float32{pos[0] * w, pos[1] * w, pos[2] * w, w}
}

func FromHomogeneous(p [4]float32) [3]float32 {
	if p[3] == 0 || p[3] == 1 {
		return [3]float32{p[0], p[1], p[2]}
	}
	inv := 1.0 / p[3]
	return [3]float32{p[0] * inv, p[1] * inv, p[2] * inv}
}

func EvalBezier(deg int, pts [][4]float32, t float32) [4]float32 {
	n := min(deg+1, len(pts), MaxStackPts)
	if n <= 0 {
		return [4]float32{}
	}
	var work [MaxStackPts][4]float32
	copy(work[:n], pts[:n])

	for r := 1; r < n; r++ {
		for i := 0; i < n-r; i++ {
			s := 1.0 - t
			for c := range 4 {
				work[i][c] = s*work[i][c] + t*work[i+1][c] //nolint:gosec // i+1 is bounded by n-r.
			}
		}
	}
	return work[0]
}

func EvalBSpline(deg int, pts [][4]float32, knots []float32, u float32) [4]float32 {
	n := len(pts)
	if n == 0 {
		return [4]float32{}
	}
	if len(knots) == 0 {
		return pts[0]
	}
	degree := min(max(deg, 0), n-1, MaxStackPts-1)
	k := degree + 1

	span := k - 1
	for span < n && span+1 < len(knots) && knots[span+1] <= u {
		span++
	}
	span = min(span, n-1)

	var d [MaxStackPts][4]float32
	for j := range k {
		idx := min(max(span-degree+j, 0), n-1)
		d[j] = pts[idx]
	}

	for r := 1; r < k; r++ {
		for j := k - 1; j >= r; j-- {
			left := min(max(span-degree+j, 0), len(knots)-1)
			right := min(left+k-r, len(knots)-1)
			denom := knots[right] - knots[left]
			if denom == 0 {
				continue
			}
			alpha := (u - knots[left]) / denom
			beta := 1.0 - alpha
			for c := range 4 {
				d[j][c] = beta*d[j-1][c] + alpha*d[j][c]
			}
		}
	}
	return d[degree] //nolint:gosec // degree is clamped to MaxStackPts-1 above.
}

// EvalCardinal evaluates a Catmull-Rom spline (s=0.5, cubic).
func EvalCardinal(pts [][4]float32, t float32) [4]float32 {
	if len(pts) < 4 { //nolint:mnd // needs 4 points
		if len(pts) > 0 {
			return pts[0]
		}
		return [4]float32{}
	}
	t2 := t * t
	t3 := t2 * t

	var result [4]float32
	for c := range 4 {
		p0, p1, p2, p3 := pts[0][c], pts[1][c], pts[2][c], pts[3][c]
		result[c] = 0.5 * ((-p0+3*p1-3*p2+p3)*t3 + //nolint:mnd // formula constant
			(2*p0-5*p1+4*p2-p3)*t2 +
			(-p0+p2)*t +
			2*p1)
	}
	return result
}

func EvalTaylor(pts [][4]float32, t float32) [4]float32 {
	if len(pts) == 0 {
		return [4]float32{}
	}
	n := len(pts)
	var result [4]float32
	for c := range 4 {
		result[c] = pts[n-1][c]
	}
	for i := n - 2; i >= 0; i-- { //nolint:mnd // formula
		for c := range 4 {
			result[c] = result[c]*t + pts[i][c]
		}
	}
	return result
}

func EvalBasisMatrix(deg int, bmat []float32, pts [][4]float32, t float32) [4]float32 {
	n := min(deg+1, len(pts), MaxStackPts)
	if n <= 0 {
		return [4]float32{}
	}
	expected := n * n
	if len(bmat) < expected {
		return pts[0]
	}

	var tv [MaxStackPts]float32
	tv[0] = 1
	for i := 1; i < n; i++ {
		tv[i] = tv[i-1] * t
	}

	var result [4]float32
	for c := range 4 {
		for i := range n {
			var coeff float32
			for j := range n {
				coeff += bmat[i*n+j] * pts[j][c]
			}
			result[c] += tv[i] * coeff
		}
	}
	return result
}

func WindingNumber(px, py float32, poly [][2]float32) int {
	n := len(poly)
	if n < 3 { //nolint:mnd // polygon must have 3 points
		return 0
	}
	wn := 0
	for i := range n {
		j := (i + 1) % n
		yi, yj := poly[i][1], poly[j][1]
		if yi <= py {
			if yj > py {
				if crossSign(poly[i], poly[j], px, py) > 0 {
					wn++
				}
			}
		} else if yj <= py {
			if crossSign(poly[i], poly[j], px, py) < 0 {
				wn--
			}
		}
	}
	return wn
}

func crossSign(a, b [2]float32, px, py float32) float32 {
	return (b[0]-a[0])*(py-a[1]) - (px-a[0])*(b[1]-a[1])
}

func ComputeSurfDims(knotsU, knotsV []float32, degU, degV, totalPts int) (nu, nv int) {
	nu = len(knotsU) - degU - 1
	nv = len(knotsV) - degV - 1

	if nu <= 0 || nv <= 0 {
		side := int(math.Sqrt(float64(totalPts)))
		if side*side != totalPts {
			return 0, 0
		}
		return side, side
	}
	return nu, nv
}
