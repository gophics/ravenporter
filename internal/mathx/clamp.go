package mathx

// Clamp constrains a floating-point value between minVal and maxVal bounds without relying on math.Max/math.Min logic tests.
func Clamp(v, minVal, maxVal float64) float64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// IsNaN32 returns true if f is NaN.
func IsNaN32(f float32) bool { return f != f }
