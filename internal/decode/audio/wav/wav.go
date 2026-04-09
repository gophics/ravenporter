package wav

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decode/audio/mp3"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/rperr"
)

const (
	formatName  = "WAV"
	wavName     = "wav"
	extWAV      = ".wav"
	riffID      = "RIFF"
	rf64ID      = "RF64"
	bw64ID      = "BW64"
	waveID      = "WAVE"
	ds64ChunkID = "ds64"
	fmtChunkID  = "fmt "
	dataChunkID = "data"
	listChunkID = "LIST"
	smplChunkID = "smpl"
	cueChunkID  = "cue "
	infoID      = "INFO"
	adtlID      = "adtl"
	lablID      = "labl"

	listTitleID   = "INAM"
	listArtistID  = "IART"
	listAlbumID   = "IALB"
	listCommentID = "ICMT"
	listGenreID   = "IGNR"

	fmtPCM        = 1
	fmtADPCM      = 2
	fmtIEEEFloat  = 3
	fmtAlaw       = 6
	fmtUlaw       = 7
	fmtIMAADPCM   = 0x0011
	fmtMP3        = 0x0055
	fmtExtensible = 0xFFFE

	riffHeaderSize  = 12
	fmtMinSize      = 16
	fmtExtMinSize   = 40
	chunkHeaderLen  = 8
	subFormatOffset = 24

	smplMinSize       = 36
	smplLoopBase      = 36
	smplLoopRecordLen = 24
	smplNumLoopsOff   = 28
	smplLoopStartOff  = 8
	smplLoopEndOff    = 12
	cueCountOff       = 0
	cueDataOff        = 4
	cueRecordLen      = 24
	cueSampleOff      = 20
	cueLabelIDOff     = 0
	cueLabelTextOff   = 4

	ds64DataSizeOff = 8
	ds64MinSize     = 16
	listTypeLen     = 4

	maxMetadataSize = 1024 * 1024
	rawBufSize      = 8192

	fmtBlockAlignOff      = 12
	fmtSamplesPerBlockOff = 18
	fmtNumCoeffsOff       = 20
	fmtCoeffDataOff       = 22
	fmtADPCMMinSize       = 22
	coeffPairSize         = 4
)

var (
	errNotRIFF     = errors.New("not a RIFF file")
	errNotWAVE     = errors.New("not a WAVE file")
	errNoFmt       = errors.New("missing fmt chunk")
	errNoData      = errors.New("missing data chunk")
	errUnsupported = errors.New("unsupported WAV format")
	errBadBitDepth = errors.New("unsupported bit depth")
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatWAV, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	defer func() { _, _ = r.Seek(pos, io.SeekStart) }() //nolint:errcheck // resets pos

	var buf [riffHeaderSize]byte
	n, err := r.Read(buf[:])
	if err != nil || n < riffHeaderSize {
		return false
	}
	containerID := string(buf[:4])
	return (containerID == riffID || containerID == rf64ID || containerID == bw64ID) && string(buf[8:12]) == waveID
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, wavErr(err)
	}
	sysCtx := opts.Context
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	hdr, err := parseWAV(sysCtx, r)
	if err != nil {
		return nil, err
	}
	// Do not decode samples immediately; return a closure for lazy decoding.
	// Since WAV could be quite large, we'll assume the ReadSeeker remains accessible,
	// or we extract the raw bytes into clip.Compressed.
	data, readErr := decutil.ReadAll(r)
	if readErr != nil {
		return nil, readErr
	}

	clip := &ir.AudioClip{
		Name:        "WAV Audio",
		Format:      ir.AudioWAV,
		SampleRate:  int(hdr.sampleRate),
		Layout:      decutil.LayoutFromChannels(int(hdr.numChannels)),
		ChannelMask: hdr.channelMask,
		BitDepth:    decutil.BitDepthFromBits(int(hdr.bitsPerSample)),
		Duration:    hdr.duration(),
		LoopStart:   hdr.loopStart,
		LoopEnd:     hdr.loopEnd,
		Metadata:    hdr.metadata,
		Compressed:  data, // Store raw file bytes for lazy decode or passthrough
	}

	clip.SampleDecode = func(c *ir.AudioClip) ([]float32, error) {
		// Replace r with a memory reader on the cached data
		hdr.r = bytes.NewReader(c.Compressed)
		samples, decErr := decodeSamples(sysCtx, hdr)
		if decErr != nil {
			return nil, decErr
		}
		if opts.MaxAudioSamples > 0 && len(samples) > opts.MaxAudioSamples {
			return nil, wavErr(errors.New("audio sample limit exceeded"))
		}
		c.Duration = decutil.AudioDuration(len(samples)/int(hdr.numChannels), int(hdr.sampleRate))
		return samples, nil
	}
	return &ir.Asset{
		AudioClips: []*ir.AudioClip{clip},
		Metadata:   ir.AssetMetadata{SourceFormat: ir.FormatWAV},
	}, nil
}

func (d *Decoder) Extensions() []string { return []string{extWAV} }
func (d *Decoder) FormatName() string   { return formatName }

type wavHeader struct {
	audioFormat     uint16
	numChannels     uint16
	sampleRate      uint32
	bitsPerSample   uint16
	blockAlign      uint16
	channelMask     uint32
	samplesPerBlock uint16
	adpcmCoeffs     [][2]int
	dataOffset      int64
	dataSize        int64
	r               detect.ReadSeekerAt
	metadata        ir.AudioMetadata
	loopStart       int
	loopEnd         int
	cueIndex        map[uint32]int
	cueLabels       map[uint32]string
}

func (h *wavHeader) duration() time.Duration {
	if h == nil || h.sampleRate == 0 {
		return 0
	}

	switch h.audioFormat {
	case fmtADPCM, fmtIMAADPCM:
		if h.blockAlign == 0 || h.samplesPerBlock == 0 {
			return 0
		}
		blocks := int(h.dataSize / int64(h.blockAlign))
		return decutil.AudioDuration(blocks*int(h.samplesPerBlock), int(h.sampleRate))
	default:
		if h.blockAlign == 0 {
			return 0
		}
		frames := int(h.dataSize / int64(h.blockAlign))
		return decutil.AudioDuration(frames, int(h.sampleRate))
	}
}

func parseWAV(sysCtx context.Context, r detect.ReadSeekerAt) (*wavHeader, error) {
	var riffBuf [riffHeaderSize]byte
	if _, err := io.ReadFull(r, riffBuf[:]); err != nil {
		return nil, wavErr(errNotRIFF)
	}
	containerID := string(riffBuf[:4])
	if containerID != riffID && containerID != rf64ID && containerID != bw64ID {
		return nil, wavErr(errNotRIFF)
	}
	if string(riffBuf[8:12]) != waveID {
		return nil, wavErr(errNotWAVE)
	}

	hdr := wavHeader{
		loopStart: ir.NoIndex,
		loopEnd:   ir.NoIndex,
		cueIndex:  make(map[uint32]int),
		cueLabels: make(map[uint32]string),
	}
	return parseChunks(sysCtx, r, &hdr)
}

func parseChunks(
	sysCtx context.Context, r detect.ReadSeekerAt, hdr *wavHeader,
) (*wavHeader, error) {
	var foundFmt, foundData bool
	var metaBuf [256]byte
	var ds64DataSize int64

chunkLoop:
	for {
		if err := sysCtx.Err(); err != nil {
			return nil, wavErr(err)
		}
		var chunkBuf [chunkHeaderLen]byte
		if _, err := io.ReadFull(r, chunkBuf[:]); err != nil {
			break
		}
		chunkID := string(chunkBuf[:4])
		chunkSize := int64(binread.ReadU32LE(chunkBuf[4:]))

		switch chunkID {
		case ds64ChunkID:
			data, ok := readMetadataChunk(r, chunkSize, padded(chunkSize), metaBuf[:])
			if !ok {
				break chunkLoop
			}
			ds64DataSize = parseDS64Chunk(data)
		case fmtChunkID:
			if err := parseFmtChunk(r, chunkSize, hdr); err != nil {
				return nil, err
			}
			foundFmt = true
		case dataChunkID:
			if chunkSize < 0 {
				return nil, wavErr(errNoData)
			}
			actualSize := chunkSize
			if chunkSize == int64(^uint32(0)) && ds64DataSize > 0 {
				actualSize = ds64DataSize
			}
			offset, seekErr := r.Seek(0, io.SeekCurrent)
			if seekErr != nil {
				return nil, wavErr(errNoData)
			}
			hdr.dataOffset = offset
			hdr.dataSize = actualSize
			hdr.r = r
			if _, err := r.Seek(padded(actualSize), io.SeekCurrent); err != nil {
				return nil, wavErr(errNoData)
			}
			foundData = true
		case listChunkID, smplChunkID, cueChunkID:
			data, ok := readMetadataChunk(r, chunkSize, padded(chunkSize), metaBuf[:])
			if !ok {
				break chunkLoop
			}
			switch chunkID {
			case listChunkID:
				parseLISTChunk(data, hdr)
			case smplChunkID:
				parseSmplChunk(data, hdr)
			case cueChunkID:
				parseCueChunk(data, hdr)
			}
		default:
			if _, err := r.Seek(padded(chunkSize), io.SeekCurrent); err != nil {
				break chunkLoop
			}
		}
	}

	if !foundFmt {
		return nil, wavErr(errNoFmt)
	}
	if !foundData {
		return nil, wavErr(errNoData)
	}
	return hdr, nil
}

func parseFmtChunk(r io.Reader, size int64, hdr *wavHeader) error {
	if size < fmtMinSize {
		return wavErr(errNoFmt)
	}
	if size > maxMetadataSize {
		return wavErr(errNoFmt)
	}
	bufSize := int(size)

	var stackBuf [256]byte
	var buf []byte
	if bufSize <= len(stackBuf) {
		buf = stackBuf[:bufSize]
	} else {
		buf = make([]byte, bufSize)
	}

	if _, err := io.ReadFull(r, buf); err != nil {
		return wavErr(errNoFmt)
	}
	hdr.audioFormat = binread.ReadU16LE(buf[0:2])
	hdr.numChannels = binread.ReadU16LE(buf[2:4])
	hdr.sampleRate = binread.ReadU32LE(buf[4:8])
	hdr.blockAlign = binread.ReadU16LE(buf[fmtBlockAlignOff:])
	hdr.bitsPerSample = binread.ReadU16LE(buf[14:16])

	if hdr.audioFormat == fmtExtensible && size >= fmtExtMinSize {
		hdr.channelMask = binread.ReadU32LE(buf[20:24])
		subFormat := binread.ReadU16LE(buf[subFormatOffset:])
		hdr.audioFormat = subFormat
	}

	switch hdr.audioFormat {
	case fmtPCM, fmtIEEEFloat, fmtAlaw, fmtUlaw:
		return nil
	case fmtADPCM:
		return parseADPCMFmt(buf, hdr)
	case fmtIMAADPCM:
		return parseIMAADPCMFmt(buf, hdr)
	case fmtMP3:
		return nil
	default:
		return wavErr(errUnsupported)
	}
}

func parseADPCMFmt(buf []byte, hdr *wavHeader) error {
	if len(buf) < fmtADPCMMinSize {
		return wavErr(errUnsupported)
	}
	hdr.samplesPerBlock = binread.ReadU16LE(buf[fmtSamplesPerBlockOff:])
	numCoeffs := int(binread.ReadU16LE(buf[fmtNumCoeffsOff:]))
	numCoeffs = min(numCoeffs, adpcmMaxCoeffCount)

	coeffOff := fmtCoeffDataOff
	hdr.adpcmCoeffs = make([][2]int, numCoeffs)
	for i := range numCoeffs {
		if coeffOff+coeffPairSize > len(buf) {
			break
		}
		hdr.adpcmCoeffs[i][0] = int(int16(binread.ReadU16LE(buf[coeffOff:])))   //nolint:gosec
		hdr.adpcmCoeffs[i][1] = int(int16(binread.ReadU16LE(buf[coeffOff+2:]))) //nolint:gosec
		coeffOff += coeffPairSize
	}
	return nil
}

func decodeSamples(sysCtx context.Context, hdr *wavHeader) ([]float32, error) {
	switch hdr.audioFormat {
	case fmtADPCM:
		return decodeADPCMSamples(hdr)
	case fmtIMAADPCM:
		return decodeIMAADPCMSamples(hdr)
	case fmtMP3:
		return decodeMP3Samples(hdr)
	}

	bytesPerSample := int(hdr.bitsPerSample) / decutil.BitsPerByte
	if bytesPerSample == 0 {
		return nil, wavErr(errBadBitDepth)
	}
	sampleCount := int(hdr.dataSize) / bytesPerSample
	samples := make([]float32, sampleCount)

	_, err := hdr.r.Seek(hdr.dataOffset, io.SeekStart)
	if err != nil {
		return nil, wavErr(errNoData)
	}

	var rawBuf [rawBufSize]byte
	chunkSize := (len(rawBuf) / bytesPerSample) * bytesPerSample

	bytesRead := int64(0)
	sampleOffset := 0

	for bytesRead < hdr.dataSize {
		if err := sysCtx.Err(); err != nil {
			return nil, wavErr(err)
		}
		toRead := min(chunkSize, int(hdr.dataSize-bytesRead))

		n, err := io.ReadFull(hdr.r, rawBuf[:toRead])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, wavErr(errNoData)
		}
		if n == 0 {
			break
		}

		endSample := min(sampleOffset+(n/bytesPerSample), len(samples))

		dst := samples[sampleOffset:endSample]
		src := rawBuf[:n]

		switch hdr.audioFormat {
		case fmtIEEEFloat:
			decutil.DecodeIEEEFloat(src, dst, bytesPerSample)
		case fmtAlaw:
			decutil.DecodeAlaw(src, dst)
		case fmtUlaw:
			decutil.DecodeUlaw(src, dst)
		case fmtPCM:
			if err := decodePCM(src, dst, bytesPerSample); err != nil {
				return nil, err
			}
		default:
			return nil, wavErr(errUnsupported)
		}

		sampleOffset += n / bytesPerSample
		bytesRead += int64(n)
	}

	return samples, nil
}

func decodePCM(raw []byte, samples []float32, bytesPerSample int) error {
	switch bytesPerSample {
	case decutil.Bytes8:
		decutil.Decode8Bit(raw, samples)
	case decutil.Bytes16:
		decutil.Decode16LE(raw, samples)
	case decutil.Bytes24:
		decutil.Decode24LE(raw, samples)
	case decutil.Bytes32:
		decutil.Decode32LE(raw, samples)
	default:
		return wavErr(errBadBitDepth)
	}
	return nil
}

func parseDS64Chunk(data []byte) int64 {
	if len(data) < ds64MinSize {
		return 0
	}
	size := binread.ReadU64LE(data[ds64DataSizeOff:])
	if size > uint64(^uint(0)>>1) {
		return 0
	}
	return int64(size)
}

func padded(size int64) int64 {
	n := size
	if n%2 != 0 {
		n++
	}
	return n
}

func readMetadataChunk(r io.ReadSeeker, chunkSize, padSize int64, scratch []byte) ([]byte, bool) {
	if chunkSize <= 0 || chunkSize >= maxMetadataSize {
		_, err := r.Seek(padSize, io.SeekCurrent)
		return nil, err == nil
	}

	var buf []byte
	if padSize <= int64(len(scratch)) {
		buf = scratch[:padSize]
	} else {
		buf = make([]byte, padSize)
	}

	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, true
	}

	return buf[:int(chunkSize)], true
}

func parseLISTChunk(data []byte, hdr *wavHeader) {
	if len(data) < listTypeLen {
		return
	}

	switch string(data[:listTypeLen]) {
	case infoID:
		parseINFOList(data[listTypeLen:], hdr)
	case adtlID:
		parseADTLList(data[listTypeLen:], hdr)
	}
}

func parseINFOList(data []byte, hdr *wavHeader) {
	for len(data) >= 8 {
		id := string(data[:4])
		size := int(binread.ReadU32LE(data[4:8]))
		pad := 0
		if size%2 != 0 {
			pad = 1
		}
		if len(data) < 8+size+pad {
			break
		}

		val := string(bytes.TrimRight(data[8:8+size], "\x00"))
		switch id {
		case listTitleID:
			hdr.metadata.Title = val
		case listArtistID:
			hdr.metadata.Artist = val
		case listAlbumID:
			hdr.metadata.Album = val
		case listCommentID:
			hdr.metadata.Comment = val
		case listGenreID:
			hdr.metadata.Genre = val
		}

		data = data[8+size+pad:]
	}
}

func parseADTLList(data []byte, hdr *wavHeader) {
	for len(data) >= 8 {
		id := string(data[:4])
		size := int(binread.ReadU32LE(data[4:8]))
		pad := 0
		if size%2 != 0 {
			pad = 1
		}
		if len(data) < 8+size+pad {
			break
		}

		if id == lablID && size >= cueLabelTextOff {
			payload := data[8 : 8+size]
			cueID := binread.ReadU32LE(payload[cueLabelIDOff:])
			label := string(bytes.TrimRight(payload[cueLabelTextOff:], "\x00"))
			setCueLabel(hdr, cueID, label)
		}

		data = data[8+size+pad:]
	}
}

func parseSmplChunk(data []byte, hdr *wavHeader) {
	if len(data) < smplMinSize {
		return
	}
	numLoops := int(binread.ReadU32LE(data[smplNumLoopsOff : smplNumLoopsOff+4]))
	if numLoops == 0 || len(data) < smplLoopBase+smplLoopRecordLen {
		return
	}

	start := binread.ReadU32LE(data[smplLoopBase+smplLoopStartOff : smplLoopBase+smplLoopStartOff+4])
	end := binread.ReadU32LE(data[smplLoopBase+smplLoopEndOff : smplLoopBase+smplLoopEndOff+4])

	hdr.loopStart = int(start)
	hdr.loopEnd = int(end)
}

func parseCueChunk(data []byte, hdr *wavHeader) {
	if len(data) < cueDataOff {
		return
	}

	count := min(int(binread.ReadU32LE(data[cueCountOff:])), (len(data)-cueDataOff)/cueRecordLen)
	if count == 0 {
		return
	}
	if hdr.cueIndex == nil {
		hdr.cueIndex = make(map[uint32]int, count)
	}

	cues := hdr.metadata.CuePoints
	if cap(cues)-len(cues) < count {
		next := make([]ir.CuePoint, len(cues), len(cues)+count)
		copy(next, cues)
		cues = next
	}

	for off := cueDataOff; count > 0; count, off = count-1, off+cueRecordLen {
		cueID := binread.ReadU32LE(data[off:])
		name := strconv.FormatUint(uint64(cueID), 10)
		if label, ok := hdr.cueLabels[cueID]; ok {
			name = label
		}
		cues = append(cues, ir.CuePoint{
			Name:   name,
			Sample: int(binread.ReadU32LE(data[off+cueSampleOff:])),
		})
		hdr.cueIndex[cueID] = len(cues) - 1
	}

	hdr.metadata.CuePoints = cues
}

func setCueLabel(hdr *wavHeader, cueID uint32, label string) {
	if label == "" {
		return
	}
	if hdr.cueLabels == nil {
		hdr.cueLabels = make(map[uint32]string, 1)
	}
	hdr.cueLabels[cueID] = label
	if idx, ok := hdr.cueIndex[cueID]; ok && idx >= 0 && idx < len(hdr.metadata.CuePoints) {
		hdr.metadata.CuePoints[idx].Name = label
	}
}

func wavErr(cause error) error {
	return &rperr.DecodeError{Format: ir.FormatID(wavName), Offset: -1, Message: cause.Error()}
}

func decodeMP3Samples(hdr *wavHeader) ([]float32, error) {
	if _, err := hdr.r.Seek(hdr.dataOffset, io.SeekStart); err != nil {
		return nil, wavErr(errNoData)
	}

	raw := make([]byte, int(hdr.dataSize))
	n, err := io.ReadFull(hdr.r, raw)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, wavErr(err)
	}
	raw = raw[:n]

	mp3Dec := &mp3.Decoder{}
	opts := detect.DecodeOptions{MaxFileSize: hdr.dataSize}
	scene, err := mp3Dec.Decode(bytes.NewReader(raw), opts)
	if err != nil {
		return nil, wavErr(err)
	}
	if len(scene.AudioClips) == 0 {
		return nil, wavErr(errNoData)
	}
	return scene.AudioClips[0].DecodeSamples()
}
