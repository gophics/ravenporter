package aiff

import (
	"context"
	"errors"
	"io"
)

const (
	ima4PacketSamples = 64
	ima4PacketBytes   = 34
	ima4InitialBytes  = 2

	maxLinear = 32768.0

	ima4MaxStreamingChunkSize = 8192
)

var imaIndexTable = [16]int{
	-1, -1, -1, -1, 2, 4, 6, 8,
	-1, -1, -1, -1, 2, 4, 6, 8,
}

var imaStepTable = [89]int{
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

func decodeIMA4(sysCtx context.Context, r io.Reader, dataSize uint32, numChannels int) []float32 {
	if numChannels == 0 {
		numChannels = 1
	}
	blockSize := ima4PacketBytes * numChannels
	if blockSize == 0 || int(dataSize) < blockSize {
		return nil
	}

	numBlocks := int(dataSize) / blockSize
	totalSamples := numBlocks * ima4PacketSamples * numChannels
	dst := make([]float32, totalSamples)

	var chBuf [ima4PacketSamples]float32

	chunkBytes := (ima4MaxStreamingChunkSize / blockSize) * blockSize
	if chunkBytes == 0 {
		chunkBytes = blockSize
	}
	var rawBuf []byte
	var stackBuf [ima4MaxStreamingChunkSize]byte
	if chunkBytes <= len(stackBuf) {
		rawBuf = stackBuf[:chunkBytes]
	} else {
		rawBuf = make([]byte, chunkBytes)
	}

	outIdx := 0
	bytesRead := 0
	for bytesRead < int(dataSize) {
		if err := sysCtx.Err(); err != nil {
			return dst[:outIdx]
		}
		toRead := min(chunkBytes, int(dataSize)-bytesRead)
		n, err := io.ReadFull(r, rawBuf[:toRead])
		if err != nil && err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if n < blockSize {
			break
		}

		blocksInChunk := n / blockSize
		for b := range blocksInChunk {
			blockOff := b * blockSize

			if numChannels == 1 {
				decodeIMA4Packet(rawBuf[blockOff:blockOff+ima4PacketBytes], dst[outIdx:outIdx+ima4PacketSamples])
				outIdx += ima4PacketSamples
				continue
			}

			for ch := range numChannels {
				packetOff := blockOff + ch*ima4PacketBytes
				decodeIMA4Packet(rawBuf[packetOff:packetOff+ima4PacketBytes], chBuf[:])

				for s := range ima4PacketSamples {
					dst[outIdx+s*numChannels+ch] = chBuf[s]
				}
			}
			outIdx += ima4PacketSamples * numChannels
		}
		bytesRead += blocksInChunk * blockSize
	}
	return dst[:outIdx]
}

func decodeIMA4Packet(packet []byte, dst []float32) {
	if len(packet) < ima4PacketBytes || len(dst) < ima4PacketSamples {
		return
	}

	predictor := int(int16(packet[0])<<8 | int16(packet[1]))
	stepIndex := int(packet[1]) & 0x7F //nolint:mnd

	if stepIndex >= len(imaStepTable) {
		stepIndex = len(imaStepTable) - 1
	}

	dst[0] = float32(predictor) / maxLinear
	sampleIdx := 1

	for dataOff := ima4InitialBytes; dataOff < ima4PacketBytes && sampleIdx < ima4PacketSamples; dataOff++ {
		b := packet[dataOff]

		for nibble := range 2 {
			var code byte
			if nibble == 0 {
				code = b & 0x0F //nolint:mnd
			} else {
				code = b >> 4 //nolint:mnd
			}

			step := imaStepTable[stepIndex]
			diff := step >> 3 //nolint:mnd
			if code&4 != 0 {
				diff += step
			}
			if code&2 != 0 {
				diff += step >> 1
			}
			if code&1 != 0 {
				diff += step >> 2 //nolint:mnd
			}

			if code&8 != 0 {
				predictor -= diff
			} else {
				predictor += diff
			}

			predictor = max(-32768, min(32767, predictor)) //nolint:mnd // 16-bit clamp

			stepIndex += imaIndexTable[code]
			stepIndex = max(0, min(len(imaStepTable)-1, stepIndex))

			if sampleIdx < ima4PacketSamples {
				dst[sampleIdx] = float32(predictor) / maxLinear
				sampleIdx++
			}
		}
	}
}
