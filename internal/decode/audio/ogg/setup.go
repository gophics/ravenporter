package ogg

import (
	"encoding/base64"
	"math"
	"strings"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	vorbisString = "vorbis"

	headerIDType      = 0x01
	headerCommentType = 0x03
	headerSetupType   = 0x05

	minIDHeaderSize      = 30
	minCommentHeaderSize = headerStringOff + resBitsSize // 7-byte prefix + 4-byte vendor length
	minSetupHeaderSize   = 8
	headerStringOff      = 7

	blocksizeByte      = 28
	framingByte        = 29
	blocksizeLowMask   = 0x0F
	blocksizeHighShift = 4
	channelsByte       = 11
	sampleRateOff      = 12
	sampleRateEnd      = 16

	minBlocksize = 64
	maxBlocksize = 8192

	codebookSync       = 0x564342
	codebookSyncBits   = 24
	codebookDimBits    = 16
	codebookEntryBits  = 24
	codebookLenBits    = 5
	codebookLookupBits = 4
	codebookMinValBits = 32
	codebookValBitsBit = 4

	floorCountBits   = 6
	floorTypeBits    = 16
	floorPartBits    = 5
	floorClassBits   = 4
	floorDimBits     = 3
	floorSubBits     = 2
	floorBookBits    = 8
	floorMultBits    = 2
	floorRangeBitCnt = 4

	residueCountBits    = 6
	residueTypeBits     = 16
	residueFieldBits    = 24
	residueClassBits    = 6
	residueClassBookBit = 8
	residueCascadeLow   = 3
	residueCascadeHigh  = 5
	residueBookPasses   = 8
	residueBookBits     = 8

	mappingCountBits  = 6
	mappingTypeBits   = 16
	mappingSubmapBits = 4
	mappingCoupBits   = 8
	mappingMuxBits    = 4
	mappingFloorBits  = 8
	mappingResBits    = 8
	mappingTimeBits   = 8
	mappingReserved   = 2

	modeCountBits     = 6
	modeBlockBits     = 1
	modeWindowBits    = 16
	modeTransformBits = 16
	modeMappingBits   = 8

	timeCountBits = 6
	timeEntryBits = 16

	vorbisFloatMantissa = 0x1FFFFF
	vorbisFloatSign     = 0x80000000
	vorbisFloatExpMask  = 0x7FE00000
	vorbisFloatExpShift = 21
	vorbisFloatExpBias  = 788

	resBitsSize = 4
	xListBase   = 2

	metadataPicturePrefix = "METADATA_BLOCK_PICTURE="

	maxCodebookEntries    = 8192
	maxCodebookDimensions = 200
)

type vorbisSetup struct {
	channels      int
	sampleRate    int
	blocksize0    int
	blocksize1    int
	audioMetadata ir.AudioMetadata

	codebooks []vorbisCodebook
	times     []int
	floors    []vorbisFloor
	residues  []vorbisResidue
	mappings  []vorbisMapping
	modes     []vorbisMode
}

type vorbisCodebook struct {
	dimensions int
	entries    int
	lengths    []uint8
	lookupType int
	minVal     float32
	deltaVal   float32
	valBits    int
	seqP       int
	multi      []uint32
	lookupVals []float32
	huffTree   *huffNode
}

type vorbisFloor struct {
	floorType        int
	partitions       int
	partitionClass   []int
	classDimensions  []int
	classSubclasses  []int
	classMasterbooks []int
	subclassBooks    [][]int
	multiplier       int
	rangeBits        int
	xList            []int
}

type vorbisResidue struct {
	residueType     int
	begin           int
	end             int
	partitionSize   int
	classifications int
	classbook       int
	cascade         []int
	books           [][]int
}

type vorbisMapping struct {
	mappingType   int
	submaps       int
	mux           []int
	couplingSteps int
	magnitude     []int
	angle         []int
	submapFloor   []int
	submapResidue []int
}

type vorbisMode struct {
	blockflag     int
	windowtype    int
	transformtype int
	mapping       int
}

func readIdentificationHeader(packet []byte, setup *vorbisSetup) error {
	if len(packet) < minIDHeaderSize {
		return decutil.DecodeErr(ir.FormatOGG, "identification header too small", nil)
	}
	if packet[0] != headerIDType || string(packet[1:headerStringOff]) != vorbisString {
		return decutil.DecodeErr(ir.FormatOGG, "missing vorbis identification string", nil)
	}
	version := binread.ReadU32LE(packet[headerStringOff : headerStringOff+4])
	if version != 0 {
		return decutil.DecodeErr(ir.FormatOGG, "unsupported vorbis version", nil)
	}

	setup.channels = int(packet[channelsByte])
	if setup.channels == 0 {
		return decutil.DecodeErr(ir.FormatOGG, "zero channels", nil)
	}

	setup.sampleRate = int(binread.ReadU32LE(packet[sampleRateOff:sampleRateEnd]))
	if setup.sampleRate == 0 {
		return decutil.DecodeErr(ir.FormatOGG, "zero sample rate", nil)
	}

	setup.blocksize0 = 1 << (packet[blocksizeByte] & blocksizeLowMask)
	setup.blocksize1 = 1 << (packet[blocksizeByte] >> blocksizeHighShift)

	if setup.blocksize0 < minBlocksize || setup.blocksize1 > maxBlocksize {
		return decutil.DecodeErr(ir.FormatOGG, "blocksize out of allowed range [64, 8192]", nil)
	}
	if setup.blocksize0 > setup.blocksize1 {
		return decutil.DecodeErr(ir.FormatOGG, "blocksize0 > blocksize1", nil)
	}

	if packet[framingByte]&0x01 == 0 {
		return decutil.DecodeErr(ir.FormatOGG, "framing bit missing in identification header", nil)
	}

	return nil
}

func readCommentHeader(packet []byte, setup *vorbisSetup) error {
	if len(packet) < minCommentHeaderSize {
		return decutil.DecodeErr(ir.FormatOGG, "comment header too small", nil)
	}
	if packet[0] != headerCommentType || string(packet[1:headerStringOff]) != vorbisString {
		return decutil.DecodeErr(ir.FormatOGG, "missing vorbis comment string", nil)
	}

	setup.audioMetadata = decutil.ParseVorbisComment(packet[headerStringOff:])

	offset := headerStringOff
	vendorLen := int(binread.ReadU32LE(packet[offset:]))
	offset += resBitsSize + vendorLen

	if offset+resBitsSize <= len(packet) {
		commentListLen := int(binread.ReadU32LE(packet[offset:]))
		offset += resBitsSize

		for i := 0; i < commentListLen && offset+resBitsSize <= len(packet); i++ {
			cLen := int(binread.ReadU32LE(packet[offset:]))
			offset += resBitsSize
			if offset+cLen <= len(packet) {
				commentStr := string(packet[offset : offset+cLen])

				if strings.HasPrefix(strings.ToUpper(commentStr), metadataPicturePrefix) {
					b64 := commentStr[len(metadataPicturePrefix):]
					if bin, err := base64.StdEncoding.DecodeString(b64); err == nil {
						decutil.ExtractPictureComment(bin, &setup.audioMetadata)
					}
				}
				offset += cLen
			}
		}
	}

	return nil
}

func readSetupHeader(packet []byte, setup *vorbisSetup) error {
	if len(packet) < minSetupHeaderSize {
		return decutil.DecodeErr(ir.FormatOGG, "setup header too small", nil)
	}
	if packet[0] != headerSetupType || string(packet[1:headerStringOff]) != vorbisString {
		return decutil.DecodeErr(ir.FormatOGG, "missing vorbis setup string", nil)
	}

	br := newBitReader(packet[headerStringOff:])

	codebookCount := br.readBits(residueBookBits) + 1
	setup.codebooks = make([]vorbisCodebook, codebookCount)

	for i := range int(codebookCount) {
		cb := &setup.codebooks[i]

		if err := parseCodebook(br, cb); err != nil {
			return err
		}
	}

	timeCount := int(br.readBits(timeCountBits)) + 1
	setup.times = make([]int, timeCount)
	for i := range timeCount {
		setup.times[i] = int(br.readBits(timeEntryBits))
		if setup.times[i] != 0 {
			return decutil.DecodeErr(ir.FormatOGG, "nonzero time domain", nil)
		}
	}

	if err := readFloors(br, setup); err != nil {
		return err
	}

	if err := readResidues(br, setup); err != nil {
		return err
	}

	if err := readMappings(br, setup); err != nil {
		return err
	}

	modeCount := int(br.readBits(modeCountBits)) + 1
	setup.modes = make([]vorbisMode, modeCount)
	for i := range modeCount {
		md := &setup.modes[i]
		md.blockflag = int(br.readBits(modeBlockBits))
		md.windowtype = int(br.readBits(modeWindowBits))
		md.transformtype = int(br.readBits(modeTransformBits))
		md.mapping = int(br.readBits(modeMappingBits))

		if md.windowtype != 0 || md.transformtype != 0 {
			return decutil.DecodeErr(ir.FormatOGG, "invalid mode window/transform type", nil)
		}
	}

	if br.readBits(1) != 1 {
		return decutil.DecodeErr(ir.FormatOGG, "setup header framing bit missing", nil)
	}

	return nil
}

func readFloors(br *bitReader, setup *vorbisSetup) error {
	floorCount := int(br.readBits(floorCountBits)) + 1
	setup.floors = make([]vorbisFloor, floorCount)
	for i := range floorCount {
		f := &setup.floors[i]
		f.floorType = int(br.readBits(floorTypeBits))

		switch f.floorType {
		case 0:
			br.readBits(residueBookBits) // order
			br.readBits(floorTypeBits)   // rate
			br.readBits(floorTypeBits)   // barkmap
			br.readBits(floorCountBits)  // amp bits
			br.readBits(residueBookBits) // amp offset
			numBooks := int(br.readBits(floorClassBits)) + 1
			for range numBooks {
				br.readBits(residueBookBits)
			}
		case 1:
			if err := readFloorType1(br, f); err != nil {
				return err
			}
		default:
			return decutil.DecodeErr(ir.FormatOGG, "invalid floor type", nil)
		}
	}
	return nil
}

func readFloorType1(br *bitReader, f *vorbisFloor) error {
	f.partitions = int(br.readBits(floorPartBits))
	f.partitionClass = make([]int, f.partitions)
	maxClass := -1
	for j := range f.partitions {
		f.partitionClass[j] = int(br.readBits(floorClassBits))
		if f.partitionClass[j] > maxClass {
			maxClass = f.partitionClass[j]
		}
	}
	f.classDimensions = make([]int, maxClass+1)
	f.classSubclasses = make([]int, maxClass+1)
	f.classMasterbooks = make([]int, maxClass+1)
	f.subclassBooks = make([][]int, maxClass+1)
	for j := range maxClass + 1 {
		f.classDimensions[j] = int(br.readBits(floorDimBits)) + 1
		f.classSubclasses[j] = int(br.readBits(floorSubBits))
		if f.classSubclasses[j] != 0 {
			f.classMasterbooks[j] = int(br.readBits(floorBookBits))
		}
		mult := 1 << f.classSubclasses[j]
		f.subclassBooks[j] = make([]int, mult)
		for k := range mult {
			f.subclassBooks[j][k] = int(br.readBits(floorBookBits)) - 1
		}
	}
	f.multiplier = int(br.readBits(floorMultBits)) + 1
	f.rangeBits = int(br.readBits(floorRangeBitCnt))

	xListCount := 0
	for j := range f.partitions {
		xListCount += f.classDimensions[f.partitionClass[j]]
	}

	f.xList = make([]int, xListCount+xListBase)
	f.xList[0] = 0
	f.xList[1] = 1 << f.rangeBits
	for j := xListBase; j < xListCount+xListBase; j++ {
		f.xList[j] = int(br.readBits(f.rangeBits))
	}
	return nil
}

func readResidues(br *bitReader, setup *vorbisSetup) error {
	residueCount := int(br.readBits(residueCountBits)) + 1
	setup.residues = make([]vorbisResidue, residueCount)
	for i := range residueCount {
		r := &setup.residues[i]
		r.residueType = int(br.readBits(residueTypeBits))
		if r.residueType > 2 { //nolint:mnd
			return decutil.DecodeErr(ir.FormatOGG, "invalid residue type", nil)
		}

		r.begin = int(br.readBits(residueFieldBits))
		r.end = int(br.readBits(residueFieldBits))
		r.partitionSize = int(br.readBits(residueFieldBits)) + 1
		r.classifications = int(br.readBits(residueClassBits)) + 1
		r.classbook = int(br.readBits(residueClassBookBit))

		r.cascade = make([]int, r.classifications)
		for j := range r.classifications {
			highBits := 0
			lowBits := int(br.readBits(residueCascadeLow))
			if br.readBits(1) == 1 {
				highBits = int(br.readBits(residueCascadeHigh))
			}
			r.cascade[j] = (highBits << residueCascadeLow) | lowBits
		}

		r.books = make([][]int, r.classifications)
		for j := range r.classifications {
			r.books[j] = make([]int, residueBookPasses)
			for k := range residueBookPasses {
				if (r.cascade[j] & (1 << k)) != 0 {
					r.books[j][k] = int(br.readBits(residueBookBits))
				} else {
					r.books[j][k] = -1
				}
			}
		}
	}
	return nil
}

func readMappings(br *bitReader, setup *vorbisSetup) error {
	mappingCount := int(br.readBits(mappingCountBits)) + 1
	setup.mappings = make([]vorbisMapping, mappingCount)
	for i := range mappingCount {
		m := &setup.mappings[i]
		m.mappingType = int(br.readBits(mappingTypeBits))
		if m.mappingType != 0 {
			return decutil.DecodeErr(ir.FormatOGG, "invalid mapping type", nil)
		}

		m.submaps = 1
		if br.readBits(1) == 1 {
			m.submaps = int(br.readBits(mappingSubmapBits)) + 1
		}

		if br.readBits(1) == 1 {
			m.couplingSteps = int(br.readBits(mappingCoupBits)) + 1
			m.magnitude = make([]int, m.couplingSteps)
			m.angle = make([]int, m.couplingSteps)
			for j := range m.couplingSteps {
				m.magnitude[j] = int(br.readBits(ilog(setup.channels - 1)))
				m.angle[j] = int(br.readBits(ilog(setup.channels - 1)))
			}
		}

		if br.readBits(mappingReserved) != 0 {
			return decutil.DecodeErr(ir.FormatOGG, "mapping reserved bits not zero", nil)
		}

		m.mux = make([]int, setup.channels)
		if m.submaps > 1 {
			for j := range setup.channels {
				m.mux[j] = int(br.readBits(mappingMuxBits))
			}
		}

		m.submapFloor = make([]int, m.submaps)
		m.submapResidue = make([]int, m.submaps)
		for j := range m.submaps {
			br.readBits(mappingTimeBits)
			m.submapFloor[j] = int(br.readBits(mappingFloorBits))
			m.submapResidue[j] = int(br.readBits(mappingResBits))
		}
	}
	return nil
}

func expandLookupType1(cb *vorbisCodebook, lookupCount int) {
	if lookupCount == 0 {
		return
	}
	for j := range cb.entries {
		last := float32(0.0)
		indexDivider := 1
		for k := range cb.dimensions {
			multiIndex := (j / indexDivider) % lookupCount
			val := cb.minVal + float32(cb.multi[multiIndex])*cb.deltaVal
			if cb.seqP == 1 {
				val += last
				last = val
			}
			cb.lookupVals[j*cb.dimensions+k] = val
			indexDivider *= lookupCount
			if indexDivider <= 0 {
				break
			}
		}
	}
}

func expandLookupType2(cb *vorbisCodebook) {
	for j := range cb.entries {
		last := float32(0.0)
		multiIdx := j * cb.dimensions
		for k := range cb.dimensions {
			val := cb.minVal + float32(cb.multi[multiIdx+k])*cb.deltaVal
			if cb.seqP == 1 {
				val += last
				last = val
			}
			cb.lookupVals[j*cb.dimensions+k] = val
		}
	}
}

func float32frombits(bits uint32) float32 {
	mantissa := bits & vorbisFloatMantissa
	sign := bits & vorbisFloatSign
	exp := (bits & vorbisFloatExpMask) >> vorbisFloatExpShift

	res := float64(mantissa) * math.Exp2(float64(exp)-vorbisFloatExpBias)
	if sign != 0 {
		res = -res
	}
	return float32(res)
}

func lookup1Values(entries, dimensions int) int {
	if dimensions == 0 || entries == 0 {
		return 0
	}
	r := 0
	for r < entries {
		pow := 1
		overflow := false
		for range dimensions {
			pow *= (r + 1)
			if pow > entries || pow <= 0 {
				overflow = true
				break
			}
		}
		if overflow || pow > entries {
			break
		}
		r++
	}
	return r
}

// parseCodebook parses a single vorbisCodebook from the bitstream.
func parseCodebook(br *bitReader, cb *vorbisCodebook) error {
	sync := br.readBits(codebookSyncBits)
	if sync != codebookSync {
		return decutil.DecodeErr(ir.FormatOGG, "bad codebook sync", nil)
	}

	cb.dimensions = int(br.readBits(codebookDimBits))
	cb.entries = int(br.readBits(codebookEntryBits))

	if cb.entries > maxCodebookEntries || cb.dimensions > maxCodebookDimensions {
		return decutil.DecodeErr(ir.FormatOGG, "codebook too large", nil)
	}

	ordered := br.readBits(1)
	cb.lengths = make([]uint8, cb.entries)

	if ordered == 1 {
		currentLength := br.readBits(codebookLenBits) + 1
		for j := 0; j < cb.entries; {
			num := int(br.readBits(ilog(cb.entries - j)))
			for k := 0; k < num && j < cb.entries; k++ {
				cb.lengths[j] = uint8(currentLength) //nolint:gosec
				j++
			}
			currentLength++
			if currentLength > 32 { //nolint:mnd // Vorbis maximum allowed length
				return decutil.DecodeErr(ir.FormatOGG, "max length", errCodebookMax)
			}
		}
	} else {
		sparse := br.readBits(1)
		for j := range cb.entries {
			if sparse == 1 {
				if br.readBits(1) == 1 {
					cb.lengths[j] = uint8(br.readBits(codebookLenBits) + 1) //nolint:gosec
				}
			} else {
				cb.lengths[j] = uint8(br.readBits(codebookLenBits) + 1) //nolint:gosec
			}
		}
	}

	if tree, err := buildHuffmanTree(cb.lengths); err == nil {
		cb.huffTree = tree
	}

	cb.lookupType = int(br.readBits(codebookLookupBits))

	switch cb.lookupType {
	case 0:
		// No lookup table
	case 1, 2: //nolint:mnd
		cb.minVal = float32frombits(br.readBits(codebookMinValBits))
		cb.deltaVal = float32frombits(br.readBits(codebookMinValBits))
		cb.valBits = int(br.readBits(codebookValBitsBit)) + 1
		cb.seqP = int(br.readBits(1))

		lookupCount := cb.entries * cb.dimensions
		if cb.lookupType == 1 {
			lookupCount = lookup1Values(cb.entries, cb.dimensions)
		}

		cb.multi = make([]uint32, lookupCount)
		for j := range lookupCount {
			cb.multi[j] = br.readBits(cb.valBits)
		}

		cb.lookupVals = make([]float32, cb.entries*cb.dimensions)

		switch cb.lookupType {
		case 1:
			expandLookupType1(cb, lookupCount)
		default:
			expandLookupType2(cb)
		}
	default:
		return decutil.DecodeErr(ir.FormatOGG, "invalid codebook lookup type", nil)
	}

	return nil
}
