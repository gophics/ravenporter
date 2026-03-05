package mp3

import "math"

type sideInfo struct {
	mainDataBegin int
	granules      [granules][maxChannels]granuleInfo
}

type granuleInfo struct {
	part23Len      int
	bigValues      int
	globalGain     int
	sfCompress     int
	windowSwitch   bool
	blockType      int
	mixedBlock     bool
	tableSelect    [3]int
	subBlockGain   [3]float64
	region0Count   int
	region1Count   int
	preFlag        int
	sfScale        int
	count1TableSel int
}

type frameDecoder struct {
	synth     synthesizer
	overlap   [maxChannels][subbands][samplesPerSB]float64
	reservoir []byte
	mainBuf   []byte
}

func newFrameDecoder() *frameDecoder {
	return &frameDecoder{
		reservoir: make([]byte, 0, reservoirCap),
		mainBuf:   make([]byte, 0, reservoirCap),
	}
}

func (fd *frameDecoder) reset() {
	fd.synth.reset()
	fd.overlap = [maxChannels][subbands][samplesPerSB]float64{}
	fd.reservoir = fd.reservoir[:0]
	fd.mainBuf = fd.mainBuf[:0]
}

func (fd *frameDecoder) decodeFrame(data []byte, info *mp3Info, out []float32) (consumed, written int) { //nolint:cyclop,funlen
	frameLen := computeFrameLen(info.sampleRate, info.bitrate)
	if len(data) < frameLen || frameLen == 0 {
		return 0, 0
	}

	sideLen := sideInfoStereo
	if info.channels == 1 {
		sideLen = sideInfoMono
	}
	if info.version != versionMPEG1 {
		if info.channels == 1 {
			sideLen = sideInfoMonoLSF
		} else {
			sideLen = sideInfoStereoLSF
		}
	}

	crcLen := 0
	if info.hasCRC {
		crcLen = 2
	}

	if frameLen < frameHeaderLen+crcLen+sideLen {
		return frameLen, 0
	}

	sideData := data[frameHeaderLen+crcLen : frameHeaderLen+crcLen+sideLen]

	if info.hasCRC {
		expectedCRC := uint16(data[crcByte0])<<crcShift | uint16(data[crcByte1])
		if !checkCRC(data[2], data[3], sideData, expectedCRC) {
			return frameLen, 0 // Drop frame on CRC mismatch
		}
	}

	si := parseSideInfo(sideData, info)

	mainData := data[frameHeaderLen+crcLen+sideLen : frameLen]

	fd.mainBuf = append(fd.mainBuf[:0], fd.reservoir...)
	fd.mainBuf = append(fd.mainBuf, mainData...)

	if si.mainDataBegin > len(fd.reservoir) {
		fd.updateReservoir(mainData)
		return frameLen, 0
	}

	startOff := len(fd.reservoir) - si.mainDataBegin
	br2 := newBitReaderFromBytes(fd.mainBuf[startOff:])

	outIdx := 0
	srIdx := sfBandIndex(info.sampleRate)

	nGr := granules
	if info.version != versionMPEG1 {
		nGr = 1 // MPEG-2 LSF has only 1 granule
	}

	for gr := range nGr {
		var freqs [maxChannels][freqLines]float64
		var scalefactors [maxChannels][39]int

		for ch := range info.channels {
			gi := &si.granules[gr][ch]

			readScalefactors(&br2, gi, scalefactors[ch][:], info) //nolint:gosec
			if ch < maxChannels {
				huffDecode(&br2, gi, freqs[ch][:], srIdx, info)
				requantize(gi, freqs[ch][:], scalefactors[ch][:], srIdx, info)
			}
		}

		if info.channels == maxChannels {
			stereoProcess(freqs[:], &si.granules[gr][0], &si.granules[gr][1], scalefactors[0][:], scalefactors[1][:], info, srIdx)
		}

		for ch := range info.channels {
			gi := &si.granules[gr][ch]
			if gi.blockType != blockTypeShort || gi.mixedBlock {
				aliasReduce(freqs[ch][:])
			}

			var subbandSamples [subbands][samplesPerSB]float64
			imdctGranule(gi, freqs[ch][:], fd.overlap[ch][:], subbandSamples[:])

			for s := range samplesPerSB {
				var sbSamples [subbands]float64
				for sb := range subbands {
					sbSamples[sb] = subbandSamples[sb][s] //nolint:gosec
				}
				n := fd.synth.processSamples(sbSamples, ch, out, outIdx)
				outIdx += n
			}
		}
	}

	fd.updateReservoir(mainData)
	return frameLen, outIdx
}

func (fd *frameDecoder) updateReservoir(mainData []byte) {
	fd.reservoir = append(fd.reservoir[:0], mainData...)
	const maxReservoir = 512
	if len(fd.reservoir) > maxReservoir {
		fd.reservoir = fd.reservoir[len(fd.reservoir)-maxReservoir:]
	}
}

func computeFrameLen(sr, br int) int {
	if sr == 0 || br == 0 {
		return 0
	}
	return 144 * br * 1000 / sr
}

func parseSideInfo(data []byte, info *mp3Info) sideInfo {
	br := newBitReaderFromBytes(data)
	var si sideInfo

	lsf := info.version != versionMPEG1
	if lsf {
		si.mainDataBegin = br.readBits(bitsMainDataBeginLSF)
		if info.channels == 1 {
			br.readBits(bitsPrivateMonoLSF)
		} else {
			br.readBits(bitsPrivateStereoLSF)
		}
	} else {
		si.mainDataBegin = br.readBits(bitsMainDataBegin)
		if info.channels == 1 {
			br.readBits(bitsPrivateMono)
		} else {
			br.readBits(bitsPrivateStereo)
		}
		for range info.channels {
			for range scfsiBands {
				br.readBits(1)
			}
		}
	}

	nGr := granules
	if lsf {
		nGr = 1
	}

	for gr := range nGr {
		for ch := range info.channels {
			gi := &si.granules[gr][ch]
			gi.part23Len = br.readBits(bitsPart23Len)
			gi.bigValues = br.readBits(bitsBigValues)
			gi.globalGain = br.readBits(bitsGlobalGain)
			if lsf {
				gi.sfCompress = br.readBits(bitsSfCompressLSF)
			} else {
				gi.sfCompress = br.readBits(bitsSfCompress)
			}
			gi.windowSwitch = br.readBits(1) == 1

			if gi.windowSwitch {
				gi.blockType = br.readBits(bitsBlockType)
				gi.mixedBlock = br.readBits(1) == 1
				gi.tableSelect[0] = br.readBits(bitsTableSelect)
				gi.tableSelect[1] = br.readBits(bitsTableSelect)
				gi.subBlockGain[0] = float64(br.readBits(bitsSubBlockGain))
				gi.subBlockGain[1] = float64(br.readBits(bitsSubBlockGain))
				gi.subBlockGain[2] = float64(br.readBits(bitsSubBlockGain))
				if gi.blockType == blockTypeShort {
					gi.region0Count = region0Short
					gi.region1Count = region1Short
				} else {
					gi.region0Count = region0Mixed
					gi.region1Count = region1Mixed
				}
			} else {
				gi.tableSelect[0] = br.readBits(bitsTableSelect)
				gi.tableSelect[1] = br.readBits(bitsTableSelect)
				gi.tableSelect[2] = br.readBits(bitsTableSelect)
				gi.region0Count = br.readBits(bitsRegion0Count)
				gi.region1Count = br.readBits(bitsRegion1Count)
			}

			gi.preFlag = br.readBits(1)
			gi.sfScale = br.readBits(1)
			gi.count1TableSel = br.readBits(1)
		}
	}
	return si
}

type bitReader struct {
	data []byte
	pos  int
}

func newBitReaderFromBytes(data []byte) bitReader {
	return bitReader{data: data}
}

func (b *bitReader) readBits(n int) int {
	val := 0
	for range n {
		byteIdx := b.pos / bitsPerByte
		bitIdx := topBitShift - b.pos%bitsPerByte
		if byteIdx < len(b.data) {
			val = (val << 1) | int((b.data[byteIdx]>>uint(bitIdx))&1) //nolint:gosec
		} else {
			val <<= 1
		}
		b.pos++
	}
	return val
}

func (b *bitReader) readBit() int {
	return b.readBits(1)
}

func readScalefactors(br *bitReader, gi *granuleInfo, sf []int, info *mp3Info) {
	lsf := info.version != versionMPEG1
	var slen1, slen2, slen3, slen4 int

	if lsf {
		comp := gi.sfCompress
		if gi.windowSwitch && gi.blockType == blockTypeShort {
			hi := comp >> lsfSlenShift
			lo := comp & modeExtMask
			if comp < lsfCompRange0 {
				slen1, slen2, slen3, slen4 = hi/lsfDiv5, hi%lsfDiv5, (comp>>lsfSlenShift2)&modeExtMask, lo
			} else if comp < lsfCompRange1 {
				comp -= lsfCompRange0
				hi = comp >> lsfSlenShift
				lo = comp & modeExtMask
				slen1, slen2, slen3, slen4 = hi/lsfDiv5, hi%lsfDiv5, (comp>>lsfSlenShift2)&modeExtMask, lo
			} else {
				comp -= lsfCompRange1
				slen1, slen2, slen3, slen4 = comp/lsfDiv3, comp%lsfDiv3, 0, 0
			}
		} else {
			hi := comp >> lsfSlenShift
			lo := comp & modeExtMask
			if comp < lsfCompRange0 {
				slen1, slen2, slen3, slen4 = hi/lsfDiv5, hi%lsfDiv5, (comp>>lsfSlenShift2)&modeExtMask, lo
			} else if comp < lsfCompRange1 {
				comp -= lsfCompRange0
				hi = comp >> lsfSlenShift
				lo = comp & modeExtMask
				slen1, slen2, slen3, slen4 = hi/lsfDiv5, hi%lsfDiv5, (comp>>lsfSlenShift2)&modeExtMask, lo
				gi.preFlag = 1
			} else {
				comp -= lsfCompRange1
				slen1, slen2, slen3, slen4 = comp/lsfDiv3, comp%lsfDiv3, 0, 0
				gi.preFlag = 1
			}
		}

		if gi.windowSwitch && gi.blockType == blockTypeShort {
			for i := range sfShortBands {
				var bits int
				if i < sfZone3 {
					bits = slen1
				} else if i < sfZone6 {
					bits = slen2
				} else if i < sfZone9 {
					bits = slen3
				} else {
					bits = slen4
				}
				sf[i] = br.readBits(bits)
			}
		} else {
			for i := range sfZone21 {
				var bits int
				if i < sfZone6 {
					bits = slen1
				} else if i < sfZone11 {
					bits = slen2
				} else if i < sfZone16 {
					bits = slen3
				} else {
					bits = slen4
				}
				sf[i] = br.readBits(bits)
			}
		}
		return
	}

	if gi.sfCompress >= len(sfLenTable) {
		return
	}
	slen1 = sfLenTable[gi.sfCompress][0]
	slen2 = sfLenTable[gi.sfCompress][1]

	if gi.windowSwitch && gi.blockType == blockTypeShort {
		for i := range sfShortBands {
			if i < sfSplitBand {
				sf[i] = br.readBits(slen1)
			} else {
				sf[i] = br.readBits(slen2)
			}
		}
		return
	}

	for i := range sfLongBands {
		if i < sfSplitBand {
			sf[i] = br.readBits(slen1)
		} else {
			sf[i] = br.readBits(slen2)
		}
	}
	for i := sfLongBands; i < sfLongTail; i++ {
		sf[i] = br.readBits(slen2)
	}
}

func huffDecode(br *bitReader, gi *granuleInfo, freq []float64, srIdx int, info *mp3Info) { //nolint:cyclop,funlen
	for i := range freq {
		freq[i] = 0
	}

	bitsRead := 0
	maxBits := gi.part23Len
	idx := 0

	r0 := gi.region0Count + 1
	r1 := r0 + gi.region1Count + 1
	var bands []int
	if info.version != versionMPEG1 {
		bands = sfBandLongLSF[srIdx][:]
	} else {
		bands = sfBandLong[srIdx][:]
	}

	bound0, bound1 := freqLines, freqLines
	if r0 < len(bands) {
		bound0 = bands[r0]
	}
	if r1 < len(bands) {
		bound1 = bands[r1]
	}

	maxBigValues := gi.bigValues * maxChannels
	regionBounds := [3]int{bound0, bound1, maxBigValues}
	for i := range 3 {
		if regionBounds[i] > maxBigValues {
			regionBounds[i] = maxBigValues
		}
	}

	for region := range 3 {
		tblIdx := gi.tableSelect[region]
		regionEnd := regionBounds[region] //nolint:gosec
		if tblIdx == 0 || tblIdx >= len(huffmanDescs) {
			continue
		}
		desc := &huffmanDescs[tblIdx]
		if desc.table == nil || desc.treelen == 0 {
			continue
		}
		for idx < regionEnd && bitsRead < maxBits {
			x, y, bits := huffDecodeValue(br, desc)
			bitsRead += bits

			if idx < freqLines {
				freq[idx] = float64(x)
			}
			idx++
			if idx < freqLines {
				freq[idx] = float64(y)
			}
			idx++
		}
	}

	count1Idx := count1TableBase + gi.count1TableSel
	if count1Idx >= len(huffmanDescs) {
		return
	}
	desc := &huffmanDescs[count1Idx]
	if desc.table == nil || desc.treelen == 0 {
		return
	}

	for bitsRead < maxBits && idx+3 < freqLines {
		v, w, x, y, bits := huffDecodeQuad(br, desc)
		bitsRead += bits

		freq[idx] = float64(v)
		freq[idx+1] = float64(w)
		freq[idx+2] = float64(x)
		freq[idx+3] = float64(y)
		idx += count1Stride
	}
}

func huffDecodeValue(br *bitReader, desc *huffDesc) (x, y, bits int) {
	point := 0
	bitsUsed := 0

	for desc.table[point]&huffNodeMask != 0 {
		bit := br.readBit()
		bitsUsed++

		if bit != 0 {
			for desc.table[point]&huffLoByte >= huffEscape {
				point += int(desc.table[point]) & huffLoByte
			}
			point += int(desc.table[point]) & huffLoByte
		} else {
			for desc.table[point]>>bitsPerByte >= huffEscape {
				point += int(desc.table[point]) >> bitsPerByte
			}
			point += int(desc.table[point]) >> bitsPerByte
		}

		if bitsUsed >= huffMaxBits || point >= desc.treelen {
			return 0, 0, bitsUsed
		}
	}

	x = int((desc.table[point] >> count1Stride) & huffLoNibble)
	y = int(desc.table[point] & huffLoNibble)

	if desc.linbits > 0 && x == huffLinMax {
		x += br.readBits(desc.linbits)
		bitsUsed += desc.linbits
	}
	if x != 0 && br.readBit() == 1 {
		x = -x
		bitsUsed++
	} else if x != 0 {
		bitsUsed++
	}

	if desc.linbits > 0 && y == huffLinMax {
		y += br.readBits(desc.linbits)
		bitsUsed += desc.linbits
	}
	if y != 0 && br.readBit() == 1 {
		y = -y
		bitsUsed++
	} else if y != 0 {
		bitsUsed++
	}

	return x, y, bitsUsed
}

func huffDecodeQuad(br *bitReader, desc *huffDesc) (v, w, x, y, bits int) {
	point := 0
	bitsUsed := 0

	for desc.table[point]&huffNodeMask != 0 {
		bit := br.readBit()
		bitsUsed++

		if bit != 0 {
			for desc.table[point]&huffLoByte >= huffEscape {
				point += int(desc.table[point]) & huffLoByte
			}
			point += int(desc.table[point]) & huffLoByte
		} else {
			for desc.table[point]>>bitsPerByte >= huffEscape {
				point += int(desc.table[point]) >> bitsPerByte
			}
			point += int(desc.table[point]) >> bitsPerByte
		}

		if bitsUsed >= huffMaxBits || point >= desc.treelen {
			return 0, 0, 0, 0, bitsUsed
		}
	}

	val := int(desc.table[point])
	v = (val >> imdctShortWindows) & 1
	w = (val >> maxChannels) & 1
	x = (val >> 1) & 1
	y = val & 1

	if v != 0 {
		bitsUsed++
		if br.readBit() == 1 {
			v = -1
		}
	}
	if w != 0 {
		bitsUsed++
		if br.readBit() == 1 {
			w = -1
		}
	}
	if x != 0 {
		bitsUsed++
		if br.readBit() == 1 {
			x = -1
		}
	}
	if y != 0 {
		bitsUsed++
		if br.readBit() == 1 {
			y = -1
		}
	}

	return v, w, x, y, bitsUsed
}

func requantize(gi *granuleInfo, freq []float64, sf []int, srIdx int, info *mp3Info) {
	gain := float64(gi.globalGain) - globalGainOffset
	sfShift := 0.5
	if gi.sfScale != 0 {
		sfShift = 1.0
	}

	lsf := info.version != versionMPEG1
	var bands []int
	if lsf {
		bands = sfBandLongLSF[srIdx][:]
	} else {
		bands = sfBandLong[srIdx][:]
	}

	for band := range len(bands) - 1 {
		sfVal := float64(sf[band]) * sfShift
		pre := 0.0
		if gi.preFlag != 0 && band < len(pretab) {
			pre = float64(pretab[band]) * sfShift
		}
		exponent := gain/requantExpDivisor - sfVal - pre
		multiplier := math.Exp2(exponent)

		start := bands[band]
		end := bands[band+1]
		for i := start; i < end && i < freqLines; i++ {
			val := freq[i]
			if val == 0 {
				continue
			}
			if val > 0 {
				intVal := int(val)
				if intVal >= 0 && intVal < len(reqPowTab) {
					freq[i] = multiplier * reqPowTab[intVal] //nolint:gosec
				} else {
					freq[i] = multiplier * math.Pow(val, requantPow) //nolint:gosec
				}
			} else {
				intVal := int(-val)
				if intVal >= 0 && intVal < len(reqPowTab) {
					freq[i] = -multiplier * reqPowTab[intVal] //nolint:gosec
				} else {
					freq[i] = -multiplier * math.Pow(-val, requantPow) //nolint:gosec
				}
			}
		}
	}
}

func stereoProcess(freqs [][freqLines]float64, gi0, _ *granuleInfo, _, sf1 []int, info *mp3Info, srIdx int) {
	if len(freqs) < maxChannels {
		return
	}

	isBound := freqLines
	if info.mode == modeJointStereo && (info.modeExt&1) != 0 {
		isBound = 0
		for i := freqLines - 1; i >= 0; i-- {
			if freqs[1][i] != 0 { //nolint:gosec
				isBound = i + 1
				break
			}
		}
	}

	msActive := info.mode == modeJointStereo && (info.modeExt&modeExtMS) != 0

	var bands []int
	if gi0.windowSwitch && gi0.blockType == blockTypeShort {
		if info.version != versionMPEG1 {
			bands = sfBandShortLSF[srIdx][:]
		} else {
			bands = sfBandShort[srIdx][:]
		}
	} else {
		if info.version != versionMPEG1 {
			bands = sfBandLongLSF[srIdx][:]
		} else {
			bands = sfBandLong[srIdx][:]
		}
	}

	for band := range len(bands) - 1 {
		start := bands[band]
		end := bands[band+1]
		if start >= freqLines {
			break
		}
		// Clamp band end to total frequency lines
		if end > freqLines {
			end = freqLines
		}

		isRatioLeft := 1.0
		isRatioRight := 1.0
		isMode := false

		if start >= isBound {
			isMode = true
			isPos := sf1[band]
			if isPos != isEscapePos && isPos >= 0 && isPos < isRatioTabSize {
				// ISO 11172-3 Intensity Stereo ratio mapping:
				// The right channel scalefactor indicates the pan position.
				isRatioLeft = isRatioTable[isPos]
				isRatioRight = 1.0 / isRatioLeft
				sum := isRatioLeft + isRatioRight
				isRatioLeft /= sum
				isRatioRight /= sum
			}
		}

		for i := start; i < end; i++ {
			if isMode {
				// Apply spatialized scaling from left channel energy into both
				m := freqs[0][i]
				freqs[0][i] = m * isRatioLeft
				freqs[1][i] = m * isRatioRight //nolint:gosec
			} else if msActive {
				m := freqs[0][i]
				s := freqs[1][i] //nolint:gosec
				freqs[0][i] = (m + s) * sqrt2Inv
				freqs[1][i] = (m - s) * sqrt2Inv //nolint:gosec
			}
		}
	}
}

func aliasReduce(freq []float64) {
	for sb := 1; sb < subbands; sb++ {
		base := sb * samplesPerSB
		for i := range 8 {
			a := base - 1 - i
			b := base + i

			fa := freq[a]
			fb := freq[b]
			cs := csCoeff[i]
			ca := caCoeff[i]

			freq[a] = fa*cs - fb*ca
			freq[b] = fb*cs + fa*ca
		}
	}
}

func imdctGranule(gi *granuleInfo, freq []float64, overlap, out [][samplesPerSB]float64) {
	for sb := range subbands {
		var input [samplesPerSB]float64
		copy(input[:], freq[sb*samplesPerSB:(sb+1)*samplesPerSB])

		winType := blockTypeLong
		if gi.windowSwitch {
			winType = gi.blockType
		}

		if gi.blockType == blockTypeShort && gi.windowSwitch {
			imdctShort(input[:], overlap[sb][:], out[sb][:])
		} else {
			imdctLong(input[:], overlap[sb][:], out[sb][:], winType)
		}
	}
}

func imdctLong(input, overlap, output []float64, winType int) {
	var raw [imdctLongN]float64
	_ = input[17] // bce
	win := &imdctWin[winType]

	for i := range imdctLongN {
		cos := &imdctCosLong[i]
		sum := 0.0
		for k := range samplesPerSB {
			sum += input[k] * cos[k]
		}
		raw[i] = sum * win[i] //nolint:gosec
	}

	for i := range samplesPerSB {
		output[i] = raw[i] + overlap[i] //nolint:gosec
	}
	copy(overlap[:samplesPerSB], raw[samplesPerSB:])
}

func imdctShort(input, overlap, output []float64) {
	var accumulated [imdctLongN]float64

	for win := range imdctShortWindows {
		var raw [imdctShortN]float64
		for i := range imdctShortN {
			sum := 0.0
			cos := &imdctCosShort[i]
			for k := range imdctShortHalf {
				sum += input[win*imdctShortHalf+k] * cos[k]
			}
			raw[i] = sum * imdctWin[blockTypeShort][i] //nolint:gosec
		}

		off := win*imdctShortHalf + imdctShortHalf
		maxI := imdctShortN
		if off+maxI > imdctLongN {
			maxI = imdctLongN - off
		}
		for i := 0; i < maxI; i++ {
			accumulated[off+i] += raw[i]
		}
	}

	for i := range samplesPerSB {
		output[i] = accumulated[i] + overlap[i] //nolint:gosec
	}
	copy(overlap[:samplesPerSB], accumulated[samplesPerSB:])
}

func checkCRC(b2, b3 byte, sideInfo []byte, expected uint16) bool {
	crc := uint16(crcInit)
	for _, b := range []byte{b2, b3} {
		crc ^= uint16(b) << crcShift
		for range bitsPerByte {
			if crc&crcTop != 0 {
				crc = (crc << 1) ^ crcPoly
			} else {
				crc <<= 1
			}
		}
	}
	for _, b := range sideInfo {
		crc ^= uint16(b) << crcShift
		for range bitsPerByte {
			if crc&crcTop != 0 {
				crc = (crc << 1) ^ crcPoly
			} else {
				crc <<= 1
			}
		}
	}
	return crc == expected
}
