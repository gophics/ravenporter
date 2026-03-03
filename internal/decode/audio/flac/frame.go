package flac

import (
	"context"
	"math/bits"
)

const (
	frameSyncWord  = 0xFFF8
	frameSyncMask  = 0xFFFE
	maxLPCOrder    = 32
	maxFixedOrder  = 4
	maxBlockSize   = 65535
	ricePartition2 = 0x01
	minFrameLen    = 6
	maxSyncScan    = 1 << 16

	channelLeftSide  = 0x08
	channelRightSide = 0x09
	channelMidSide   = 0x0A

	blockSizeGetByte = 6
	blockSizeGetWord = 7
	srGetKHz         = 12
	srGetHz          = 13
	srGetDHz         = 14

	sfTypeConstant   = 0
	sfTypeVerbatim   = 1
	sfFixedMin       = 8
	sfFixedMax       = 12
	sfLPCMin         = 32
	sfLPCOrderOffset = 31

	stereoThreshold = 8
	bitsPerByte     = 8
	frameCRCSize    = 16

	utf8TwoBytes    = 0xC0
	utf8TwoByteMask = 0xE0
	utf8ThreeBytes  = 0xE0
	utf8ThreeMask   = 0xF0
	utf8FourBytes   = 0xF0
	utf8FourMask    = 0xF8
	utf8FiveBytes   = 0xF8
	utf8FiveMask    = 0xFC
	utf8SixBytes    = 0xFC
	utf8SixMask     = 0xFE

	bitBufMax   = 56
	bitBufWidth = 64
	msbBit      = 63

	// Frame header field widths (FLAC format spec §frame_header).
	bitsSync      = 14
	bitBlockSize  = 4
	bitSampleRate = 4
	bitChanAssign = 4
	bitBPS        = 3
	bitSfType     = 6
	bitBlockWord  = 16
	bitLPCPrecis  = 4
	bitLPCShift   = 5
	bitResMethod  = 2
	bitPartOrder  = 4
	bitEscBPS     = 5
	bitCRC16      = 16

	// Sample rate multiplier constants.
	srMultKHz = 1000
	srMultDHz = 10

	// UTF-8 continuation byte counts for skip.
	utf8Cont2 = 16
	utf8Cont3 = 24
	utf8Cont4 = 32
	utf8Cont5 = 40
	utf8Cont6 = 48

	// Minimum channel count for decorrelation.
	minDecorrChans = 2

	ctxCheckInterval = 1024
)

var fixedCoeffs = [5][]int32{
	{},
	{1},
	{2, -1},
	{3, -3, 1},
	{4, -6, 4, -1},
}

var blockSizeTable = [16]int{
	0, 192, 576, 1152, 2304, 4608,
	0, 0,
	256, 512, 1024, 2048, 4096, 8192, 16384, 32768,
}

var sampleRateTable = [16]int{
	0, 88200, 176400, 192000,
	8000, 16000, 22050, 24000,
	32000, 44100, 48000, 96000,
	0, 0, 0, 0,
}

var bpsTable = [8]int{0, 8, 12, 0, 16, 20, 24, 0}

type frameDecoder struct {
	subBuf   [8][]int32
	residBuf []int32
}

type bitReader struct {
	data []byte
	pos  int
	bits uint64
	n    int
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (br *bitReader) reset(data []byte) {
	br.data = data
	br.pos = 0
	br.bits = 0
	br.n = 0
}

func (br *bitReader) refill() {
	for br.n <= bitBufMax && br.pos < len(br.data) {
		br.bits |= uint64(br.data[br.pos]) << (bitBufMax - br.n)
		br.n += bitsPerByte
		br.pos++
	}
}

func (br *bitReader) read(n int) uint32 {
	if n == 0 {
		return 0
	}
	br.refill()
	val := uint32(br.bits >> (bitBufWidth - n)) //nolint:gosec
	br.bits <<= n
	br.n -= n
	return val
}

func (br *bitReader) readSigned(n int) int32 {
	if n == 0 {
		return 0
	}
	v := br.read(n)
	if v&(1<<(n-1)) != 0 {
		v |= ^uint32(0) << n
	}
	return int32(v) //nolint:gosec
}

func (br *bitReader) readUnary() int {
	count := 0
	for {
		if br.n == 0 {
			br.refill()
			if br.n == 0 {
				return count
			}
		}
		lz := bits.LeadingZeros64(br.bits)
		if lz < br.n {
			count += lz
			br.bits <<= (lz + 1)
			br.n -= (lz + 1)
			return count
		}
		count += br.n
		br.bits = 0
		br.n = 0
	}
}

func (br *bitReader) alignByte() {
	skip := br.n % bitsPerByte
	if skip > 0 {
		br.read(skip)
	}
}

func decodeFrames(sysCtx context.Context, data []byte, info flacInfo) []float32 {
	if info.channels == 0 || info.bitsPerSample == 0 {
		return nil
	}

	totalSamples := info.totalSamples * info.channels
	if totalSamples == 0 {
		totalSamples = len(data) * bitsPerByte / info.bitsPerSample
	}
	samples := make([]float32, 0, totalSamples)
	maxVal := float32(int(1) << (info.bitsPerSample - 1))

	var fd frameDecoder
	var br bitReader

	misses := 0
	for off := 0; off < len(data)-1; off++ {
		if off%ctxCheckInterval == 0 {
			if err := sysCtx.Err(); err != nil {
				return samples
			}
		}
		sync := uint16(data[off])<<bitsPerByte | uint16(data[off+1])
		if sync&frameSyncMask != frameSyncWord {
			misses++
			if misses > maxSyncScan {
				break
			}
			continue
		}
		br.reset(data[off:])
		consumed := fd.decodeFrame(&br, info, maxVal, &samples)
		if consumed <= 0 {
			misses++
			if misses > maxSyncScan {
				break
			}
			continue
		}
		misses = 0
		off += consumed - 1
	}
	return samples
}

type frameHeader struct {
	blockSize   int
	sampleRate  int
	channels    int
	channelMode int
	bps         int
}

func (fd *frameDecoder) decodeFrame(br *bitReader, info flacInfo, maxVal float32, dst *[]float32) int { //nolint:cyclop
	if len(br.data) < minFrameLen {
		return 0
	}

	_ = br.read(bitsSync)
	if br.read(1) != 0 {
		return 0
	}
	_ = br.read(1)

	blockSizeCode := br.read(bitBlockSize)
	sampleRateCode := br.read(bitSampleRate)
	channelAssignment := br.read(bitChanAssign)
	bpsCode := br.read(bitBPS)
	_ = br.read(1)

	skipUTF8Number(br)

	var hdr frameHeader
	hdr.blockSize = blockSizeTable[blockSizeCode]
	switch blockSizeCode {
	case blockSizeGetByte:
		hdr.blockSize = int(br.read(bitsPerByte)) + 1
	case blockSizeGetWord:
		hdr.blockSize = int(br.read(bitBlockWord)) + 1
	}

	hdr.sampleRate = sampleRateTable[sampleRateCode]
	switch sampleRateCode {
	case srGetKHz:
		hdr.sampleRate = int(br.read(bitsPerByte)) * srMultKHz
	case srGetHz:
		hdr.sampleRate = int(br.read(bitBlockWord))
	case srGetDHz:
		hdr.sampleRate = int(br.read(bitBlockWord)) * srMultDHz
	}

	hdr.channelMode = int(channelAssignment)
	if channelAssignment < stereoThreshold {
		hdr.channels = int(channelAssignment) + 1
	} else {
		hdr.channels = 2
	}

	hdr.bps = bpsTable[bpsCode]
	if hdr.bps == 0 {
		hdr.bps = info.bitsPerSample
	}

	_ = br.read(bitsPerByte)

	if hdr.blockSize <= 0 || hdr.blockSize > maxBlockSize || hdr.channels == 0 {
		return 0
	}

	fd.ensureBuffers(hdr.blockSize, hdr.channels)

	for ch := range hdr.channels {
		bps := hdr.bps
		if needsExtraBit(hdr.channelMode, ch) {
			bps++
		}
		if !fd.decodeSubframeInto(br, fd.subBuf[ch], bps) {
			return 0
		}
	}

	decorrelate(fd.subBuf[:hdr.channels], hdr.channelMode)

	br.alignByte()
	_ = br.read(frameCRCSize)

	for i := range hdr.blockSize {
		for ch := range hdr.channels {
			*dst = append(*dst, float32(fd.subBuf[ch][i])/maxVal)
		}
	}
	return br.pos
}

func (fd *frameDecoder) ensureBuffers(blockSize, channels int) {
	for ch := range channels {
		if cap(fd.subBuf[ch]) < blockSize {
			fd.subBuf[ch] = make([]int32, blockSize)
		} else {
			fd.subBuf[ch] = fd.subBuf[ch][:blockSize]
		}
	}
	if cap(fd.residBuf) < blockSize {
		fd.residBuf = make([]int32, blockSize)
	} else {
		fd.residBuf = fd.residBuf[:blockSize]
	}
}

func (fd *frameDecoder) decodeSubframeInto(br *bitReader, dst []int32, bps int) bool {
	_ = br.read(1)
	sfType := br.read(bitSfType)
	hasWasted := br.read(1) != 0
	wasted := 0
	if hasWasted {
		wasted = br.readUnary() + 1
		bps -= wasted
	}

	blockSize := len(dst)
	var ok bool
	switch {
	case sfType == sfTypeConstant:
		ok = decodeConstantInto(br, dst, bps)
	case sfType == sfTypeVerbatim:
		ok = decodeVerbatimInto(br, dst, bps)
	case sfType >= sfFixedMin && sfType <= sfFixedMax:
		ok = fd.decodeFixedInto(br, dst, bps, int(sfType-sfFixedMin))
	case sfType >= sfLPCMin:
		ok = fd.decodeLPCInto(br, dst, bps, int(sfType-sfLPCOrderOffset))
	default:
		return false
	}
	if !ok {
		return false
	}

	if hasWasted {
		for i := range blockSize {
			dst[i] <<= wasted
		}
	}
	return true
}

func decodeConstantInto(br *bitReader, dst []int32, bps int) bool {
	val := br.readSigned(bps)
	for i := range dst {
		dst[i] = val
	}
	return true
}

func decodeVerbatimInto(br *bitReader, dst []int32, bps int) bool {
	for i := range dst {
		dst[i] = br.readSigned(bps)
	}
	return true
}

func (fd *frameDecoder) decodeFixedInto(br *bitReader, dst []int32, bps, order int) bool {
	if order > maxFixedOrder {
		return false
	}
	blockSize := len(dst)
	for i := range order {
		dst[i] = br.readSigned(bps)
	}

	if !fd.decodeResidualsInto(br, blockSize, order) {
		return false
	}

	coeffs := fixedCoeffs[order]
	for i := order; i < blockSize; i++ {
		var predicted int32
		for j, c := range coeffs {
			predicted += c * dst[i-1-j]
		}
		dst[i] = predicted + fd.residBuf[i-order]
	}
	return true
}

func (fd *frameDecoder) decodeLPCInto(br *bitReader, dst []int32, bps, order int) bool {
	if order > maxLPCOrder || order <= 0 {
		return false
	}
	blockSize := len(dst)
	for i := range order {
		dst[i] = br.readSigned(bps)
	}

	precision := int(br.read(bitLPCPrecis)) + 1
	shift := br.readSigned(bitLPCShift)

	var coeffs [maxLPCOrder]int32
	for i := range order {
		coeffs[i] = br.readSigned(precision)
	}

	if !fd.decodeResidualsInto(br, blockSize, order) {
		return false
	}

	resid := fd.residBuf
	switch order {
	case maxFixedOrder:
		c0, c1, c2, c3 := int64(coeffs[0]), int64(coeffs[1]), int64(coeffs[2]), int64(coeffs[3])
		for i := maxFixedOrder; i < blockSize; i++ {
			p := c0*int64(dst[i-1]) + c1*int64(dst[i-2]) + c2*int64(dst[i-3]) + c3*int64(dst[i-4])
			dst[i] = applyLPCShift(p, shift) + resid[i-maxFixedOrder]
		}
	case bitsPerByte:
		c := [bitsPerByte]int64{
			int64(coeffs[0]), int64(coeffs[1]), int64(coeffs[2]), int64(coeffs[3]),
			int64(coeffs[4]), int64(coeffs[5]), int64(coeffs[6]), int64(coeffs[7]),
		}
		for i := bitsPerByte; i < blockSize; i++ {
			p := c[0]*int64(dst[i-1]) + c[1]*int64(dst[i-2]) +
				c[2]*int64(dst[i-3]) + c[3]*int64(dst[i-4]) +
				c[4]*int64(dst[i-5]) + c[5]*int64(dst[i-6]) +
				c[6]*int64(dst[i-7]) + c[7]*int64(dst[i-8])
			dst[i] = applyLPCShift(p, shift) + resid[i-bitsPerByte]
		}
	case sfFixedMax:
		c := [sfFixedMax]int64{
			int64(coeffs[0]), int64(coeffs[1]), int64(coeffs[2]), int64(coeffs[3]),
			int64(coeffs[4]), int64(coeffs[5]), int64(coeffs[6]), int64(coeffs[7]),
			int64(coeffs[8]), int64(coeffs[9]), int64(coeffs[10]), int64(coeffs[11]),
		}
		for i := sfFixedMax; i < blockSize; i++ {
			p := c[0]*int64(dst[i-1]) + c[1]*int64(dst[i-2]) +
				c[2]*int64(dst[i-3]) + c[3]*int64(dst[i-4]) +
				c[4]*int64(dst[i-5]) + c[5]*int64(dst[i-6]) +
				c[6]*int64(dst[i-7]) + c[7]*int64(dst[i-8]) +
				c[8]*int64(dst[i-9]) + c[9]*int64(dst[i-10]) +
				c[10]*int64(dst[i-11]) + c[11]*int64(dst[i-12])
			dst[i] = applyLPCShift(p, shift) + resid[i-sfFixedMax]
		}
	default:
		for i := order; i < blockSize; i++ {
			var predicted int64
			for j := range order {
				predicted += int64(coeffs[j]) * int64(dst[i-1-j]) //nolint:gosec
			}
			dst[i] = applyLPCShift(predicted, shift) + resid[i-order]
		}
	}
	return true
}

func applyLPCShift(predicted int64, shift int32) int32 {
	if shift >= 0 {
		return int32(predicted >> shift) //nolint:gosec
	}
	return int32(predicted << -shift) //nolint:gosec
}

func (fd *frameDecoder) decodeResidualsInto(br *bitReader, blockSize, predictorOrder int) bool {
	method := br.read(bitResMethod)
	if method > ricePartition2 {
		return false
	}

	paramBits := 4
	if method == ricePartition2 {
		paramBits = 5
	}
	escCode := (1 << paramBits) - 1

	partitionOrder := int(br.read(bitPartOrder))
	numPartitions := 1 << partitionOrder
	idx := 0

	for p := range numPartitions {
		count := blockSize / numPartitions
		if p == 0 {
			count -= predictorOrder
		}

		param := int(br.read(paramBits))
		if param == escCode {
			bitsPerSample := int(br.read(bitEscBPS))
			for range count {
				fd.residBuf[idx] = br.readSigned(bitsPerSample)
				idx++
			}
			continue
		}

		for range count {
			q := br.readUnary()
			r := int32(0)
			if param > 0 {
				r = int32(br.read(param)) //nolint:gosec
			}
			val := int32(q)<<param | r //nolint:gosec
			if val&1 != 0 {
				val = -(val >> 1) - 1
			} else {
				val >>= 1
			}
			fd.residBuf[idx] = val
			idx++
		}
	}
	return true
}

func needsExtraBit(mode, ch int) bool {
	switch mode {
	case channelLeftSide:
		return ch == 1
	case channelRightSide:
		return ch == 0
	case channelMidSide:
		return ch == 1
	}
	return false
}

func skipUTF8Number(br *bitReader) {
	first := br.read(bitsPerByte)
	switch {
	case first&0x80 == 0:
	case first&utf8TwoByteMask == utf8TwoBytes:
		_ = br.read(bitsPerByte)
	case first&utf8ThreeMask == utf8ThreeBytes:
		_ = br.read(utf8Cont2)
	case first&utf8FourMask == utf8FourBytes:
		_ = br.read(utf8Cont3)
	case first&utf8FiveMask == utf8FiveBytes:
		_ = br.read(utf8Cont4)
	case first&utf8SixMask == utf8SixBytes:
		_ = br.read(utf8Cont5)
	default:
		_ = br.read(utf8Cont6)
	}
}

func decorrelate(subframes [][]int32, mode int) {
	if len(subframes) < minDecorrChans {
		return
	}
	left, right := subframes[0], subframes[1]
	switch mode {
	case channelLeftSide:
		for i := range left {
			right[i] = left[i] - right[i]
		}
	case channelRightSide:
		for i := range right {
			left[i] += right[i]
		}
	case channelMidSide:
		for i := range left {
			mid := left[i]
			side := right[i]
			mid = mid<<1 | side&1
			left[i] = (mid + side) >> 1
			right[i] = (mid - side) >> 1
		}
	}
}
