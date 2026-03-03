package wav

import (
	"io"

	"github.com/gophics/ravenporter/internal/binread"
)

const (
	imaBlockHdrSize  = 4
	imaNibbleMask    = 0x0F
	imaNibbleHigh    = 4
	imaStepTableSize = 89
	imaIndexMax      = 88
	imaSampleMax     = 32767
	imaSampleMin     = -32768
)

var imaStepTable = [imaStepTableSize]int{
	7, 8, 9, 10, 11, 12, 13, 14,
	16, 17, 19, 21, 23, 25, 28, 31,
	34, 37, 41, 45, 50, 55, 60, 66,
	73, 80, 88, 97, 107, 118, 130, 143,
	157, 173, 190, 209, 230, 253, 279, 307,
	337, 371, 408, 449, 494, 544, 598, 658,
	724, 796, 876, 963, 1060, 1166, 1282, 1411,
	1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024,
	3327, 3660, 4026, 4428, 4871, 5358, 5894, 6484,
	7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899,
	15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794,
	32767,
}

var imaIndexTable = [16]int{
	-1, -1, -1, -1, 2, 4, 6, 8,
	-1, -1, -1, -1, 2, 4, 6, 8,
}

type imaState struct {
	predictor int
	index     int
}

func decodeIMANibble(nibble int, s *imaState) int16 {
	step := imaStepTable[s.index]

	diff := step >> 3 //nolint:mnd // ADPCM spec diff formula
	if nibble&1 != 0 {
		diff += step >> 2 //nolint:mnd // ADPCM spec diff formula
	}
	if nibble&2 != 0 {
		diff += step >> 1
	}
	if nibble&4 != 0 {
		diff += step
	}
	if nibble&8 != 0 {
		s.predictor -= diff
	} else {
		s.predictor += diff
	}

	if s.predictor > imaSampleMax {
		s.predictor = imaSampleMax
	} else if s.predictor < imaSampleMin {
		s.predictor = imaSampleMin
	}

	s.index += imaIndexTable[nibble]
	if s.index < 0 {
		s.index = 0
	} else if s.index > imaIndexMax {
		s.index = imaIndexMax
	}

	return int16(s.predictor) //nolint:gosec // clamped above
}

func decodeIMABlock(block []byte, channels, samplesPerBlock int, states []imaState, dst []float32) []float32 {
	hdrLen := imaBlockHdrSize * channels
	if len(block) < hdrLen {
		return dst
	}

	off := 0
	for ch := range channels {
		sample := int(int16(binread.ReadU16LE(block[off:]))) //nolint:gosec
		idx := int(block[off+2])
		if idx > imaIndexMax {
			idx = imaIndexMax
		}
		states[ch].predictor = sample
		states[ch].index = idx
		off += imaBlockHdrSize
	}

	for ch := range channels {
		dst = append(dst, float32(states[ch].predictor)/imaSampleMax)
	}

	samplesDecoded := 1
	dataOff := hdrLen

	if channels == 1 {
		for dataOff < len(block) && samplesDecoded < samplesPerBlock {
			b := block[dataOff]
			dataOff++

			lo := int(b & imaNibbleMask)
			dst = append(dst, float32(decodeIMANibble(lo, &states[0]))/imaSampleMax)
			samplesDecoded++
			if samplesDecoded >= samplesPerBlock {
				break
			}

			hi := int(b >> imaNibbleHigh)
			dst = append(dst, float32(decodeIMANibble(hi, &states[0]))/imaSampleMax)
			samplesDecoded++
		}
		return dst
	}

	return decodeIMABlockStereo(block[hdrLen:], channels, samplesPerBlock, states, dst)
}

const imaStereoChunkSamples = 8

func decodeIMABlockStereo(data []byte, channels, samplesPerBlock int, states []imaState, dst []float32) []float32 {
	samplesDecoded := 1
	off := 0

	for samplesDecoded < samplesPerBlock && off < len(data) {
		for ch := range channels {
			nibbleBytes := imaStereoChunkSamples / 2 //nolint:mnd // chunk samples divided by bytes per nibble
			if off+nibbleBytes > len(data) {
				return dst
			}
			for i := range nibbleBytes {
				b := data[off+i]
				lo := int(b & imaNibbleMask)
				hi := int(b >> imaNibbleHigh)

				loSample := float32(decodeIMANibble(lo, &states[ch])) / imaSampleMax
				hiSample := float32(decodeIMANibble(hi, &states[ch])) / imaSampleMax
				dst = append(dst, loSample, hiSample)
			}
			off += nibbleBytes
		}
		samplesDecoded += imaStereoChunkSamples
	}

	return dst
}

func decodeIMAADPCMSamples(hdr *wavHeader) ([]float32, error) {
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
	states := make([]imaState, channels)

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
		dst = decodeIMABlock(block, channels, int(hdr.samplesPerBlock), states, dst)
	}
	return dst, nil
}

const imaFmtMinSize = 20

func parseIMAADPCMFmt(buf []byte, hdr *wavHeader) error {
	if len(buf) < imaFmtMinSize {
		return wavErr(errUnsupported)
	}
	hdr.samplesPerBlock = binread.ReadU16LE(buf[fmtSamplesPerBlockOff:])
	return nil
}
