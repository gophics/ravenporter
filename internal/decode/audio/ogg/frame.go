package ogg

import (
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	audioPacketBit = 0x01
)

func decodePacket(fd *frameDecoder, packet []byte) error {
	setup := fd.setup
	if len(packet) == 0 || (packet[0]&audioPacketBit) == 1 {
		return nil
	}

	br := newBitReader(packet)

	if br.readBit() != 0 {
		return decutil.DecodeErr(ir.FormatOGG, "packet type is not audio", nil)
	}

	modeNumber := int(br.readBits(ilog(len(setup.modes) - 1)))
	if modeNumber >= len(setup.modes) {
		return decutil.DecodeErr(ir.FormatOGG, "invalid mode number", nil)
	}
	mode := setup.modes[modeNumber]
	mapping := setup.mappings[mode.mapping]

	blockSize := setup.blocksize0
	if mode.blockflag == 1 {
		blockSize = setup.blocksize1
		br.readBits(2) //nolint:mnd // prev/next window flags per Vorbis I §4.3.1
	}

	for i := range setup.channels {
		for j := range blockSize / windowHalf {
			fd.channelData[i][j] = 0
		}
	}

	for i := range setup.channels {
		submapNum := 0
		if mapping.submaps > 1 {
			submapNum = mapping.mux[i]
		}

		floor := setup.floors[mapping.submapFloor[submapNum]]
		fd.noResidue[i] = true
		if floor.floorType == 1 {
			fd.noResidue[i] = !decodeFloor1(br, setup, &floor, fd.channelData[i], blockSize)
		}
	}

	for i := range mapping.submaps {
		chInBundle := 0
		doNotDecode := false
		fd.chToDecode = fd.chToDecode[:0]

		for j := range setup.channels {
			if mapping.mux[j] == i {
				chInBundle++
				if fd.noResidue[j] {
					doNotDecode = true
				} else {
					fd.chToDecode = append(fd.chToDecode, j)
				}
			}
		}

		if chInBundle > 0 && !doNotDecode {
			residue := setup.residues[mapping.submapResidue[i]]
			decodeResidue(br, fd, &residue, fd.chToDecode)
		}
	}

	for i := mapping.couplingSteps - 1; i >= 0; i-- {
		magOut := fd.channelData[mapping.magnitude[i]]
		angOut := fd.channelData[mapping.angle[i]]

		for j := range blockSize / windowHalf {
			m := magOut[j]
			a := angOut[j]

			if m > 0 {
				if a > 0 {
					magOut[j] = m
					angOut[j] = m - a
				} else {
					angOut[j] = m
					magOut[j] = m + a
				}
			} else {
				if a > 0 {
					magOut[j] = m
					angOut[j] = m + a
				} else {
					angOut[j] = m
					magOut[j] = m - a
				}
			}
		}
	}

	synthesizeChannels(fd, setup, blockSize)

	return nil
}

func synthesizeChannels(fd *frameDecoder, setup *vorbisSetup, blockSize int) {
	for i := range setup.channels {
		imdct := fd.synth.imdct0
		if blockSize == setup.blocksize1 {
			imdct = fd.synth.imdct1
		}
		synthIMDCT(fd.imdctBuf, fd.channelData[i], blockSize, imdct)

		window := fd.synth.window0
		if blockSize == setup.blocksize1 {
			window = fd.synth.window1
		}
		applyWindow(fd.imdctBuf, window, blockSize)

		if fd.prevSize > 0 {
			overlap := min(fd.prevSize, blockSize)
			overlapBuf := fd.overlapBuf[:overlap]
			synthOverlapAdd(overlapBuf, fd.prevBlock[i], fd.imdctBuf, fd.prevSize, blockSize)
			fd.pcmOut[i] = append(fd.pcmOut[i], overlapBuf...)
		}

		copy(fd.prevBlock[i][:blockSize], fd.imdctBuf[:blockSize])
	}
	fd.prevSize = blockSize
}
