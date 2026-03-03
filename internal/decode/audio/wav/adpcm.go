package wav

import (
	"io"

	"github.com/gophics/ravenporter/internal/binread"
)

const (
	adpcmPreambleSize   = 7
	adpcmMaxCoeffCount  = 7
	adpcmCoeffSetSize   = 2
	adpcmNibbleMask     = 0x0F
	adpcmNibbleHigh     = 4
	adpcmPredMin        = -32768
	adpcmPredMax        = 32767
	adpcmAdaptMin       = 16
	adpcmCoeffBytesBase = 4
)

var adpcmAdaptTable = [16]int{
	230, 230, 230, 230, 307, 409, 512, 614,
	768, 614, 512, 409, 307, 230, 230, 230,
}

type adpcmState struct {
	delta   int
	sample1 int
	sample2 int
	coeff1  int
	coeff2  int
}

func decodeADPCMBlock(
	block []byte, channels, samplesPerBlock int, coeffs [][2]int, states []adpcmState, dst []float32,
) []float32 {
	preambleLen := adpcmPreambleSize * channels
	if len(block) < preambleLen {
		return dst
	}

	off := 0

	for ch := range channels {
		predIdx := int(block[off])
		off++
		if predIdx < len(coeffs) {
			states[ch].coeff1 = coeffs[predIdx][0]
			states[ch].coeff2 = coeffs[predIdx][1]
		}
	}

	for ch := range channels {
		states[ch].delta = int(int16(binread.ReadU16LE(block[off:]))) //nolint:gosec
		off += 2
	}

	for ch := range channels {
		states[ch].sample1 = int(int16(binread.ReadU16LE(block[off:]))) //nolint:gosec
		off += 2
	}

	for ch := range channels {
		states[ch].sample2 = int(int16(binread.ReadU16LE(block[off:]))) //nolint:gosec
		off += 2
	}

	for ch := range channels {
		dst = append(dst, float32(states[ch].sample2)/adpcmPredMax)
	}
	for ch := range channels {
		dst = append(dst, float32(states[ch].sample1)/adpcmPredMax)
	}

	samplesDecoded := 2
	ch := 0
	for i := off; i < len(block) && samplesDecoded < samplesPerBlock; i++ {
		b := block[i]

		for nibble := range 2 {
			var code int
			if nibble == 0 {
				code = int(b >> adpcmNibbleHigh)
			} else {
				code = int(b & adpcmNibbleMask)
			}

			s := &states[ch]
			pred := (s.sample1*s.coeff1 + s.sample2*s.coeff2) / adpcmCoeffDiv
			signed := code
			if signed >= adpcmSignThreshold {
				signed -= adpcmSignRange
			}
			pred += signed * s.delta
			pred = max(adpcmPredMin, min(adpcmPredMax, pred))

			s.sample2 = s.sample1
			s.sample1 = pred
			s.delta = max(adpcmAdaptMin, s.delta*adpcmAdaptTable[code]/adpcmAdaptDiv)

			dst = append(dst, float32(pred)/adpcmPredMax)

			ch++
			if ch >= channels {
				ch = 0
				samplesDecoded++
				if samplesDecoded >= samplesPerBlock {
					break
				}
			}
		}
	}
	return dst
}

const (
	adpcmCoeffDiv      = 256
	adpcmSignThreshold = 8
	adpcmSignRange     = 16
	adpcmAdaptDiv      = 256
)

func decodeADPCMSamples(hdr *wavHeader) ([]float32, error) {
	if hdr.blockAlign == 0 || hdr.numChannels == 0 || hdr.samplesPerBlock == 0 {
		return nil, wavErr(errUnsupported)
	}

	_, err := hdr.r.Seek(hdr.dataOffset, io.SeekStart)
	if err != nil {
		return nil, wavErr(errNoData)
	}

	channels := int(hdr.numChannels)
	numBlocks := int(hdr.dataSize) / int(hdr.blockAlign)
	totalSamples := numBlocks * int(hdr.samplesPerBlock) * channels
	dst := make([]float32, 0, totalSamples)
	states := make([]adpcmState, channels)

	// Stack array for standard ADPCM blocks (common block size is 256, 512, 1024)
	var rawBuf [1024]byte
	var block []byte
	if int(hdr.blockAlign) <= len(rawBuf) {
		block = rawBuf[:hdr.blockAlign]
	} else {
		block = make([]byte, hdr.blockAlign)
	}

	for range numBlocks {
		if _, err := io.ReadFull(hdr.r, block); err != nil {
			break
		}
		dst = decodeADPCMBlock(block, channels, int(hdr.samplesPerBlock), hdr.adpcmCoeffs, states, dst)
	}
	return dst, nil
}
