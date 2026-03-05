package mp3

import "github.com/gophics/ravenporter/internal/mathx"

type synthesizer struct {
	buf  [maxChannels][synthBufLen]float64
	slot [maxChannels]int
}

func (s *synthesizer) reset() {
	s.buf = [maxChannels][synthBufLen]float64{}
	s.slot = [maxChannels]int{}
}

func (s *synthesizer) processSamples(samples [subbands]float64, ch int, out []float32, outIdx int) int {
	offset := s.slot[ch] * subbands * synthStride
	s.slot[ch] = (s.slot[ch] + 1) % synthSlots

	var matrixOut [64]float64
	_ = samples[31]

	for i := range 64 {
		sum := 0.0
		row := &synthCos[i]
		for k := range 32 {
			sum += samples[k] * row[k] //nolint:gosec // k < 32 == len(row)
		}
		matrixOut[i] = sum //nolint:gosec // i < 64 == len(matrixOut)
	}

	for i := range 64 {
		s.buf[ch][(offset+i)&1023] = matrixOut[i] //nolint:gosec // i < 64
	}

	written := 0
	_ = synthWindow[511]

	for j := range 32 {
		sum := 0.0
		for i := range 16 {
			bufIdx := (offset + synthMatrixN*i + j) & synthBufMask
			winIdx := j + subbands*i
			sum += s.buf[ch][bufIdx] * synthWindow[winIdx]
		}
		pos := outIdx + written
		if pos < len(out) {
			out[pos] = float32(mathx.Clamp(sum, -1.0, 1.0))
			written++
		}
	}
	return written
}
