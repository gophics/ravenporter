package ogg

import "math"

const (
	windowHalf    = 2
	halfOffset    = 0.5
	twiddleOffset = 0.125
)

type vorbisSynth struct {
	window0 []float32
	window1 []float32
	imdct0  *imdctState
	imdct1  *imdctState
}

func newSynth(blockSize0, blockSize1 int) *vorbisSynth {
	s := &vorbisSynth{
		window0: make([]float32, blockSize0),
		window1: make([]float32, blockSize1),
		imdct0:  newIMDCT(blockSize0),
		imdct1:  newIMDCT(blockSize1),
	}
	precomputeWindow(s.window0, blockSize0)
	precomputeWindow(s.window1, blockSize1)
	return s
}

func precomputeWindow(w []float32, n int) {
	for i := range n {
		v := math.Sin((float64(i) + halfOffset) / float64(n) * math.Pi)
		w[i] = float32(math.Sin((math.Pi / windowHalf) * v * v))
	}
}

func synthOverlapAdd(out, prev, curr []float32, prevSize, currSize int) {
	overlap := min(prevSize, currSize)
	halfPrev := prevSize / windowHalf

	for i := range overlap {
		out[i] = prev[halfPrev+i-(overlap/windowHalf)] + curr[i]
	}
}

type imdctState struct {
	n          int
	halfN      int
	quarterN   int
	twiddle    []complex128
	fftTwiddle []complex128
	buf        []complex128
}

func newIMDCT(n int) *imdctState {
	halfN := n / windowHalf
	quarterN := n / (windowHalf * windowHalf)
	s := &imdctState{
		n:        n,
		halfN:    halfN,
		quarterN: quarterN,
		twiddle:  make([]complex128, quarterN),
		buf:      make([]complex128, quarterN),
	}

	for k := range quarterN {
		angle := float64(windowHalf) * math.Pi / float64(n) * (float64(k) + twiddleOffset)
		s.twiddle[k] = complex(math.Cos(angle), -math.Sin(angle))
	}

	s.fftTwiddle = precomputeFFTTwiddles(quarterN)
	return s
}

func precomputeFFTTwiddles(n int) []complex128 {
	tw := make([]complex128, n)
	for size := 2; size <= n; size <<= 1 {
		halfSize := size >> 1
		angleStep := -float64(windowHalf) * math.Pi / float64(size)
		for k := range halfSize {
			angle := angleStep * float64(k)
			tw[size/windowHalf+k] = complex(math.Cos(angle), math.Sin(angle))
		}
	}
	return tw
}

func synthIMDCT(out, in []float32, n int, st *imdctState) {
	halfN := st.halfN
	quarterN := st.quarterN
	buf := st.buf

	for k := range quarterN {
		re := float64(in[2*k])
		im := float64(in[halfN-1-2*k])
		tw := st.twiddle[k]
		buf[k] = complex(
			re*real(tw)-im*imag(tw),
			re*imag(tw)+im*real(tw),
		)
	}

	fftInPlace(buf, quarterN, st.fftTwiddle)

	for k := range quarterN {
		tw := st.twiddle[k]
		v := buf[k]
		buf[k] = complex(
			real(v)*real(tw)-imag(v)*imag(tw),
			real(v)*imag(tw)+imag(v)*real(tw),
		)
	}

	for k := range quarterN {
		out[2*k] = float32(-imag(buf[k]))
		out[halfN-1-2*k] = float32(real(buf[k]))
	}
	for k := range quarterN {
		out[halfN+2*k] = -out[halfN-1-2*k]
		out[n-1-2*k] = out[2*k]
	}
}

func fftInPlace(buf []complex128, n int, twiddles []complex128) {
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			buf[i], buf[j] = buf[j], buf[i]
		}
	}

	for size := 2; size <= n; size <<= 1 {
		halfSize := size >> 1
		twBase := size / windowHalf
		for start := 0; start < n; start += size {
			for k := range halfSize {
				w := twiddles[twBase+k]
				u := buf[start+k]
				v := buf[start+k+halfSize] * w
				buf[start+k] = u + v
				buf[start+k+halfSize] = u - v
			}
		}
	}
}

func applyWindow(block, window []float32, n int) {
	for i := range n {
		block[i] *= window[i]
	}
}
