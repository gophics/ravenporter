package aiff

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "AIFF"
	extAIFF    = ".aiff"
	extAIF     = ".aif"

	formTag      = 0x464F524D
	aiffTag      = 0x41494646
	aifcTag      = 0x41494643
	commChunkTag = 0x434F4D4D
	ssndChunkTag = 0x53534E44
	markChunkTag = 0x4D41524B
	instChunkTag = 0x494E5354
	nameChunkTag = 0x4E414D45
	authChunkTag = 0x41555448
	annoChunkTag = 0x414E4E4F

	commDataSize    = 18
	commAIFCMinSize = 22
	probeLen        = 12
	ssndHeaderSize  = 8
	chunkHeaderLen  = 8
	instChunkSize   = 20
	loopNoPlay      = 0
	sustainLoopOff  = 8
	releaseLoopOff  = 14
	loopEntryLen    = 6

	markerEntrySize = 6
	maxMarkers      = 64
	minMarkSize     = 2
	ext80Mask       = 0x7FFF
	extBias         = 16383
	mantTop         = 63

	compNoneTag = 0x4E4F4E45
	compRawTag  = 0x72617720
	compSowtTag = 0x736F7774
	compTwosTag = 0x74776F73
	compTWOSTag = 0x54574F53
	compFl32Tag = 0x666C3332
	compFL32Tag = 0x464C3332
	compFl64Tag = 0x666C3634
	compFL64Tag = 0x464C3634
	compIn24Tag = 0x696E3234
	compIn32Tag = 0x696E3332
	compAlawTag = 0x616C6177
	compALAWTag = 0x414C4157
	compUlawTag = 0x756C6177
	compULAWTag = 0x554C4157
	compIma4Tag = 0x696D6134

	aiffBits24 = 24
	aiffBits32 = 32
	aiffBits64 = 64
)

var (
	errNotFORM = errors.New("not a FORM file")
	errNotAIFF = errors.New("not an AIFF/AIFC file")
	errNoCOMM  = errors.New("missing COMM chunk")
	errBadCOMM = errors.New("invalid COMM chunk")
	errNoSSND  = errors.New("missing SSND chunk")
	errBadComp = errors.New("unsupported AIFC compression")
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatAIFF, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	defer func() { _, _ = r.Seek(pos, io.SeekStart) }() //nolint:errcheck // reset pos

	var buf [probeLen]byte
	n, err := r.Read(buf[:])
	return err == nil && n >= probeLen &&
		binary.BigEndian.Uint32(buf[:4]) == formTag &&
		(binary.BigEndian.Uint32(buf[8:12]) == aiffTag || binary.BigEndian.Uint32(buf[8:12]) == aifcTag)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatAIFF, err.Error(), err)
	}
	sysCtx := opts.Context
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	info, err := parse(sysCtx, r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatAIFF, err.Error(), err)
	}
	data, readErr := decutil.ReadAll(r)
	if readErr != nil {
		return nil, decutil.DecodeErr(ir.FormatAIFF, readErr.Error(), readErr)
	}

	clip := &ir.AudioClip{
		Name:       "AIFF Audio",
		Format:     ir.AudioAIFF,
		SampleRate: info.sampleRate,
		Layout:     decutil.LayoutFromChannels(info.numChannels),
		BitDepth:   decutil.BitDepthFromBits(info.bitsPerSample),
		Duration:   decutil.AudioDuration(info.numFrames, info.sampleRate),
		LoopStart:  info.loopStart,
		LoopEnd:    info.loopEnd,
		Metadata:   info.metadata,
		Compressed: data,
	}

	clip.SampleDecode = func(c *ir.AudioClip) ([]float32, error) {
		samples, decErr := decodeSamples(sysCtx, c.Compressed, info)
		if decErr != nil {
			return nil, decutil.DecodeErr(ir.FormatAIFF, decErr.Error(), decErr)
		}
		if opts.MaxAudioSamples > 0 && len(samples) > opts.MaxAudioSamples {
			return nil, decutil.DecodeErr(ir.FormatAIFF, "audio sample limit exceeded", nil)
		}
		return samples, nil
	}
	return &ir.Asset{
		AudioClips: []*ir.AudioClip{clip},
		Metadata:   ir.AssetMetadata{SourceFormat: ir.FormatAIFF},
	}, nil
}

func (d *Decoder) Extensions() []string { return []string{extAIFF, extAIF} }
func (d *Decoder) FormatName() string   { return formatName }

type aiffInfo struct {
	numChannels   int
	numFrames     int
	bitsPerSample int
	sampleRate    int
	dataOffset    int64
	dataSize      uint32
	markers       [maxMarkers]markerEntry
	markerCount   int
	loopStart     int
	loopEnd       int
	isAIFC        bool
	isFloat       bool
	isRaw         bool
	isByteSwapped bool
	isAlaw        bool
	isUlaw        bool
	isIMA4        bool
	metadata      ir.AudioMetadata
}

type markerEntry struct {
	id  int16
	pos uint32
}

func parse(sysCtx context.Context, r io.ReadSeeker) (*aiffInfo, error) { //nolint:cyclop
	var header [probeLen]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, errNotFORM
	}
	if binary.BigEndian.Uint32(header[:4]) != formTag {
		return nil, errNotFORM
	}
	formType := binary.BigEndian.Uint32(header[8:12])
	if formType != aiffTag && formType != aifcTag {
		return nil, errNotAIFF
	}

	info := aiffInfo{
		loopStart: ir.NoIndex,
		loopEnd:   ir.NoIndex,
		isAIFC:    formType == aifcTag,
	}
	foundCOMM, foundSSND := false, false

	var chunkHdr [chunkHeaderLen]byte
	for {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, chunkHdr[:]); err != nil {
			break
		}
		chunkID := binary.BigEndian.Uint32(chunkHdr[:4])
		chunkSize := binary.BigEndian.Uint32(chunkHdr[4:])

		switch chunkID {
		case commChunkTag:
			if err := parseCOMM(r, chunkSize, &info); err != nil {
				return nil, err
			}
			foundCOMM = true

		case ssndChunkTag:
			if err := parseSSND(r, chunkSize, &info); err != nil {
				return nil, err
			}
			foundSSND = true

		case markChunkTag:
			parseMarkers(r, chunkSize, &info)
		case instChunkTag:
			parseInstrument(r, chunkSize, &info)
		case nameChunkTag:
			info.metadata.Title = readTextChunk(r, chunkSize)
		case authChunkTag:
			info.metadata.Artist = readTextChunk(r, chunkSize)
		case annoChunkTag:
			info.metadata.Comment = readTextChunk(r, chunkSize)

		default:
			skipChunk(r, chunkSize)
		}
	}

	if !foundCOMM {
		return nil, errNoCOMM
	}
	if !foundSSND {
		return nil, errNoSSND
	}
	return &info, nil
}

func parseSSND(r io.ReadSeeker, chunkSize uint32, info *aiffInfo) error {
	if chunkSize < ssndHeaderSize {
		return errNoSSND
	}
	var ssndHdr [ssndHeaderSize]byte
	if _, err := io.ReadFull(r, ssndHdr[:]); err != nil {
		return errNoSSND
	}

	offset, seekErr := r.Seek(0, io.SeekCurrent)
	if seekErr != nil {
		return errNoSSND
	}
	info.dataOffset = offset
	info.dataSize = chunkSize - ssndHeaderSize

	if _, err := r.Seek(int64(info.dataSize), io.SeekCurrent); err != nil {
		return errNoSSND
	}
	return nil
}

func parseCOMM(r io.ReadSeeker, chunkSize uint32, info *aiffInfo) error {
	if chunkSize < commDataSize {
		return errBadCOMM
	}
	var buf [commDataSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return errBadCOMM
	}
	info.numChannels = int(int16(buf[0])<<8 | int16(buf[1]))
	info.numFrames = int(binary.BigEndian.Uint32(buf[2:6]))
	info.bitsPerSample = int(int16(buf[6])<<8 | int16(buf[7]))
	info.sampleRate = extended80ToInt([10]byte(buf[8:18]))

	if !info.isAIFC || chunkSize < commAIFCMinSize {
		skipRemaining(r, chunkSize, commDataSize)
		return nil
	}

	var compType [4]byte
	if _, err := io.ReadFull(r, compType[:]); err != nil {
		return errBadCOMM
	}
	switch binary.BigEndian.Uint32(compType[:]) {
	case compNoneTag:
	case compRawTag:
		if info.bitsPerSample != decutil.BitsPerByte {
			return errBadComp
		}
		info.isRaw = true
	case compTwosTag, compTWOSTag:
	case compSowtTag:
		info.isByteSwapped = true
	case compFl32Tag, compFL32Tag:
		if info.bitsPerSample != aiffBits32 {
			return errBadComp
		}
		info.isFloat = true
	case compFl64Tag, compFL64Tag:
		if info.bitsPerSample != aiffBits64 {
			return errBadComp
		}
		info.isFloat = true
	case compIn24Tag:
		if info.bitsPerSample != aiffBits24 {
			return errBadComp
		}
	case compIn32Tag:
		if info.bitsPerSample != aiffBits32 {
			return errBadComp
		}
	case compAlawTag, compALAWTag:
		info.isAlaw = true
	case compUlawTag, compULAWTag:
		info.isUlaw = true
	case compIma4Tag:
		info.isIMA4 = true
	default:
		return errBadComp
	}
	skipRemaining(r, chunkSize, commAIFCMinSize)
	return nil
}

func decodeSamples(sysCtx context.Context, data []byte, info *aiffInfo) ([]float32, error) {
	var r bytes.Reader
	r.Reset(data)

	if _, err := r.Seek(info.dataOffset, io.SeekStart); err != nil {
		return nil, err
	}

	bytesPerSample := info.bitsPerSample / decutil.BitsPerByte
	if bytesPerSample == 0 && !info.isIMA4 {
		return nil, errBadCOMM
	}
	if bytesPerSample == 0 && !info.isAlaw && !info.isUlaw && !info.isIMA4 {
		bytesPerSample = 1
	}

	if info.isIMA4 {
		return decodeIMA4(sysCtx, &r, info.dataSize, info.numChannels), nil
	}

	if bytesPerSample <= 0 {
		return nil, errors.New("invalid bits per sample")
	}

	sampleCount := int(info.dataSize) / bytesPerSample
	if sampleCount < 0 || sampleCount > 100*1024*1024 {
		return nil, errors.New("invalid data size or length")
	}
	samples := make([]float32, sampleCount)

	var rawBuf [8192]byte
	chunkSize := (len(rawBuf) / bytesPerSample) * bytesPerSample
	bytesRead := 0
	sampleOffset := 0

	for bytesRead < int(info.dataSize) {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		toRead := min(chunkSize, int(info.dataSize)-bytesRead)
		n, err := io.ReadFull(&r, rawBuf[:toRead])
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		endSample := min(sampleOffset+(n/bytesPerSample), len(samples))
		dst := samples[sampleOffset:endSample]
		src := rawBuf[:n]

		switch {
		case info.isRaw:
			decutil.Decode8Bit(src, dst)
		case info.isAlaw:
			decutil.DecodeAlaw(src, dst)
		case info.isUlaw:
			decutil.DecodeUlaw(src, dst)
		case info.isFloat:
			decutil.DecodeIEEEFloatBE(src, dst, bytesPerSample)
		case info.isByteSwapped:
			decodeSowtSamples(src, dst, bytesPerSample)
		default:
			decodeBESamples(src, dst, bytesPerSample)
		}

		sampleOffset += n / bytesPerSample
		bytesRead += n
	}

	return samples, nil
}

func decodeSowtSamples(raw []byte, dst []float32, bytesPerSample int) {
	switch bytesPerSample {
	case decutil.Bytes8:
		decutil.Decode8Bit(raw, dst)
	case decutil.Bytes16:
		decutil.Decode16LE(raw, dst)
	case decutil.Bytes24:
		decutil.Decode24LE(raw, dst)
	case decutil.Bytes32:
		decutil.Decode32LE(raw, dst)
	}
}

func decodeBESamples(raw []byte, dst []float32, bytesPerSample int) {
	switch bytesPerSample {
	case decutil.Bytes8:
		decutil.Decode8Bit(raw, dst)
	case decutil.Bytes16:
		decode16BE(raw, dst)
	case decutil.Bytes24:
		decode24BE(raw, dst)
	case decutil.Bytes32:
		decode32BE(raw, dst)
	}
}

func decode16BE(raw []byte, dst []float32) {
	for i := range len(dst) {
		off := i * decutil.Bytes16
		dst[i] = float32(binread.ReadI16BE(raw[off:])) / decutil.MaxInt16
	}
}

func decode24BE(raw []byte, dst []float32) {
	for i := range len(dst) {
		off := i * decutil.Bytes24
		v := int32(raw[off])<<decutil.Shift16 | int32(raw[off+1])<<decutil.Shift8 | int32(raw[off+2])
		if v&decutil.SignBit24 != 0 {
			v |= decutil.SignMask24
		}
		dst[i] = float32(v) / decutil.MaxInt24
	}
}

func decode32BE(raw []byte, dst []float32) {
	for i := range len(dst) {
		off := i * decutil.Bytes32
		v := int32(binread.ReadU32BE(raw[off:])) //nolint:gosec
		dst[i] = float32(v) / decutil.MaxInt32
	}
}

func extended80ToInt(b [10]byte) int {
	exponent := int(binary.BigEndian.Uint16(b[:2])) & ext80Mask
	mantissa := binary.BigEndian.Uint64(b[2:10])
	exp := exponent - extBias - mantTop
	if exp >= 0 {
		return int(mantissa << uint(exp)) //nolint:gosec
	}
	return int(mantissa >> uint(-exp)) //nolint:gosec
}

func readTextChunk(r io.ReadSeeker, size uint32) string {
	var stackBuf [256]byte
	var buf []byte
	if size <= uint32(len(stackBuf)) {
		buf = stackBuf[:size]
	} else {
		buf = make([]byte, size)
	}
	if _, err := io.ReadFull(r, buf); err != nil {
		return ""
	}
	if size%2 != 0 {
		_, _ = r.Seek(1, io.SeekCurrent) //nolint:errcheck
	}
	return strings.TrimRight(string(buf), "\x00 ")
}

func parseMarkers(r io.ReadSeeker, size uint32, info *aiffInfo) {
	if size < minMarkSize {
		_, _ = r.Seek(int64(size), io.SeekCurrent) //nolint:errcheck
		return
	}
	var stackBuf [512]byte
	var buf []byte
	if size <= uint32(len(stackBuf)) {
		buf = stackBuf[:size]
	} else {
		buf = make([]byte, size)
	}
	if _, err := io.ReadFull(r, buf); err != nil {
		return
	}
	numMarkers := int(binary.BigEndian.Uint16(buf[:2]))
	off := 2
	for range numMarkers {
		if off+markerEntrySize > len(buf) {
			break
		}
		if info.markerCount >= maxMarkers {
			break
		}
		id := int16(binary.BigEndian.Uint16(buf[off : off+2])) //nolint:gosec
		pos := binary.BigEndian.Uint32(buf[off+2 : off+markerEntrySize])
		info.markers[info.markerCount] = markerEntry{id: id, pos: pos}
		info.markerCount++
		off += markerEntrySize
		if off >= len(buf) {
			break
		}
		strLen := int(buf[off])
		off++
		off += strLen
		if strLen%2 == 0 {
			off++
		}
	}
}

func parseInstrument(r io.ReadSeeker, size uint32, info *aiffInfo) {
	if size < instChunkSize {
		_, _ = r.Seek(int64(size), io.SeekCurrent) //nolint:errcheck
		return
	}
	var buf [instChunkSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return
	}
	if remaining := int64(size) - instChunkSize; remaining > 0 {
		_, _ = r.Seek(remaining, io.SeekCurrent) //nolint:errcheck
	}

	if resolveLoop(buf[sustainLoopOff:sustainLoopOff+loopEntryLen], info) {
		return
	}
	resolveLoop(buf[releaseLoopOff:releaseLoopOff+loopEntryLen], info)
}

func resolveLoop(entry []byte, info *aiffInfo) bool {
	playMode := binary.BigEndian.Uint16(entry[:2])
	if playMode == loopNoPlay {
		return false
	}
	beginID := int16(binary.BigEndian.Uint16(entry[2:4])) //nolint:gosec
	endID := int16(binary.BigEndian.Uint16(entry[4:6]))   //nolint:gosec
	for i := range info.markerCount {
		m := &info.markers[i]
		if m.id == beginID {
			info.loopStart = int(m.pos)
		}
		if m.id == endID {
			info.loopEnd = int(m.pos)
		}
	}
	return info.loopStart != ir.NoIndex
}

func skipChunk(r io.ReadSeeker, size uint32) {
	skip := int64(size)
	if skip%2 != 0 {
		skip++
	}
	_, _ = r.Seek(skip, io.SeekCurrent) //nolint:errcheck
}

func skipRemaining(r io.ReadSeeker, chunkSize, consumed uint32) {
	if remaining := int64(chunkSize) - int64(consumed); remaining > 0 {
		_, _ = r.Seek(remaining, io.SeekCurrent) //nolint:errcheck
	}
}
