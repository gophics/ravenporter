package mp3

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "MP3"
	extMP3     = ".mp3"

	syncByte0       = 0xFF
	syncMask1       = 0xE0
	minProbe        = 2
	probeLen        = 3
	frameHeaderLen  = 4
	id3v2HeaderSize = 10
	id3v2SizeMask   = 0x7F
	bitrateMultiple = 1000
	spfLayer3MPEG1  = 1152
	spfLayer3MPEG2  = 576

	versionShift = 3
	versionMask  = 0x3
	bitrateShift = 4
	srShift      = 2
	srMask       = 0x3
	paddingShift = 1
	paddingMask  = 1
	channelShift = 6
	channelMask  = 0x3
	channelMono  = 3

	id3v2Shift0 = 21
	id3v2Shift1 = 14
	id3v2Shift2 = 7

	vbrFramesFlag = 0x0001
	vbrBytesFlag  = 0x0002

	ctxCheckInterval = 1024
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatMP3, Decoder: &Decoder{}}}
}

var magicID3 = []byte("ID3")

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeRead(r, probeLen, func(buf []byte) bool {
		if len(buf) < minProbe {
			return false
		}
		if len(buf) >= probeLen && bytes.Equal(buf[:probeLen], magicID3) {
			return true
		}
		return buf[0] == syncByte0 && buf[1]&syncMask1 == syncMask1
	})
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatMP3, "file too large", err)
	}
	raw, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatMP3, "failed to read", err)
	}

	sysCtx := opts.Context
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	audioData := skipID3v2(raw)
	info := parseFrameHeader(audioData)

	samples := decodeAllFrames(sysCtx, audioData, info)

	if opts.MaxAudioSamples > 0 && len(samples) > opts.MaxAudioSamples {
		return nil, decutil.DecodeErr(ir.FormatMP3, "audio sample limit exceeded", nil)
	}

	totalSamples := len(samples) / max(1, info.channels)

	duration := decutil.AudioDuration(totalSamples, info.sampleRate)
	if info.vbrFrames > 0 && info.sampleRate > 0 {
		duration = decutil.AudioDuration(int(info.vbrFrames)*samplesPerFrame[info.version], info.sampleRate)
	} else if len(samples) == 0 {
		duration = calcDuration(audioData, info.sampleRate)
	}

	clip := &ir.AudioClip{
		Name:        "MP3 Audio",
		Format:      ir.AudioMP3,
		SampleRate:  info.sampleRate,
		Layout:      decutil.LayoutFromChannels(info.channels),
		BitDepth:    ir.BitDepth16,
		Duration:    duration,
		LoopStart:   ir.NoIndex,
		LoopEnd:     ir.NoIndex,
		Metadata:    decutil.ParseID3v2Tags(raw),
		Compressed:  raw,
		SourceCodec: ir.AudioMP3,
	}

	capturedInfo := info
	capturedAudioData := audioData // Sub-slice of raw
	clip.SampleDecode = func(_ *ir.AudioClip) ([]float32, error) {
		samples := decodeAllFrames(context.Background(), capturedAudioData, capturedInfo)
		if opts.MaxAudioSamples > 0 && len(samples) > opts.MaxAudioSamples {
			return nil, decutil.DecodeErr(ir.FormatMP3, "audio sample limit exceeded", nil)
		}
		return samples, nil
	}

	return &ir.Asset{
		AudioClips: []*ir.AudioClip{clip},
		Metadata:   ir.AssetMetadata{SourceFormat: ir.FormatMP3},
	}, nil
}

func decodeAllFrames(sysCtx context.Context, data []byte, info mp3Info) []float32 {
	if info.sampleRate == 0 {
		return nil
	}

	fd := newFrameDecoder()
	spf := spfLayer3MPEG1 * info.channels
	estimatedFrames := len(data) / max(1, computeFrameLen(info.sampleRate, info.bitrate))
	out := make([]float32, 0, max(spf, estimatedFrames*spf))

	buf := make([]float32, spf)
	offset := 0

	for offset < len(data)-frameHeaderLen {
		if offset%ctxCheckInterval == 0 {
			if err := sysCtx.Err(); err != nil {
				return out
			}
		}
		if !isSync(data[offset], data[offset+1]) {
			offset++
			continue
		}

		_, ok := readFrameInfo(data[offset+1], data[offset+2], data[offset+3])
		if !ok {
			offset++
			continue
		}

		consumed, written := fd.decodeFrame(data[offset:], &info, buf)
		if consumed == 0 {
			offset++
			continue
		}

		if written > 0 {
			out = append(out, buf[:written]...)
		}
		offset += consumed
	}

	return out
}

func (d *Decoder) Extensions() []string { return []string{extMP3} }
func (d *Decoder) FormatName() string   { return formatName }

type mp3Info struct {
	version    int
	mode       int
	modeExt    int
	sampleRate int
	channels   int
	bitrate    int
	hasCRC     bool
	vbrFrames  int64
	vbrBytes   int64
}

var samplesPerFrame = [4]int{spfLayer3MPEG2, 0, spfLayer3MPEG2, spfLayer3MPEG1}

var sampleRates = [4][4]int{
	{11025, 12000, 8000, 0},
	{0, 0, 0, 0},
	{22050, 24000, 16000, 0},
	{44100, 48000, 32000, 0},
}

var bitrates = [16]int{
	0, 32, 40, 48, 56, 64, 80, 96,
	112, 128, 160, 192, 224, 256, 320, 0,
}

func parseVBRHeaders(data []byte, info *mp3Info, xingOffset, frameStart int) bool {
	// Check for Xing or Info header.
	if xingOffset+8 <= len(data) {
		hasXing := bytes.HasPrefix(data[xingOffset:], []byte("Xing"))
		hasInfo := bytes.HasPrefix(data[xingOffset:], []byte("Info"))
		if hasXing || hasInfo {
			return parseXingHeader(data, info, xingOffset)
		}
	}

	// Check for VBRI header.
	vbriOffset := frameStart + frameHeaderLen + sideInfoStereo
	if vbriOffset+26 <= len(data) && bytes.HasPrefix(data[vbriOffset:], []byte("VBRI")) {
		return parseVBRIHeader(data, info, vbriOffset)
	}
	return false
}

func parseXingHeader(data []byte, info *mp3Info, offset int) bool {
	flags := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
	offset += 8
	if flags&vbrFramesFlag != 0 && offset+4 <= len(data) {
		info.vbrFrames = int64(data[offset])<<24 | int64(data[offset+1])<<16 | int64(data[offset+2])<<8 | int64(data[offset+3])
		offset += 4
	}
	if flags&vbrBytesFlag != 0 && offset+4 <= len(data) {
		info.vbrBytes = int64(data[offset])<<24 | int64(data[offset+1])<<16 | int64(data[offset+2])<<8 | int64(data[offset+3])
	}
	return true
}

func parseVBRIHeader(data []byte, info *mp3Info, offset int) bool {
	info.vbrBytes = int64(data[offset+10])<<24 | int64(data[offset+11])<<16 | int64(data[offset+12])<<8 | int64(data[offset+13])
	info.vbrFrames = int64(data[offset+14])<<24 | int64(data[offset+15])<<16 | int64(data[offset+16])<<8 | int64(data[offset+17])
	return true
}

func parseFrameHeader(data []byte) mp3Info {
	for i := 0; i < len(data)-frameHeaderLen; i++ {
		if !isSync(data[i], data[i+1]) {
			continue
		}
		info, ok := readFrameInfo(data[i+1], data[i+2], data[i+3])
		if !ok {
			continue
		}

		// Calculate Xing/VBRI offset.
		offset := i + frameHeaderLen
		if info.hasCRC {
			offset += 2
		}

		if info.version == versionMPEG1 {
			if info.channels == 1 {
				offset += 17 // Mono
			} else {
				offset += 32 // Stereo
			}
		} else { // MPEG-2 / 2.5
			if info.channels == 1 {
				offset += 9 // Mono
			} else {
				offset += 17 // Stereo
			}
		}

		parseVBRHeaders(data, &info, offset, i)
		return info
	}
	return mp3Info{}
}

func calcDuration(data []byte, sampleRate int) time.Duration {
	if sampleRate <= 0 {
		return 0
	}

	totalSamples := 0
	for i := 0; i < len(data)-frameHeaderLen; i++ {
		if !isSync(data[i], data[i+1]) {
			continue
		}
		version := (data[i+1] >> versionShift) & versionMask
		spf := samplesPerFrame[version]
		if spf == 0 {
			continue
		}

		b2 := data[i+2]
		brIdx := b2 >> bitrateShift
		srIdx := (b2 >> srShift) & srMask
		padding := int((b2 >> paddingShift) & paddingMask)

		if int(version) >= len(sampleRates) || int(srIdx) >= len(sampleRates[version]) {
			continue
		}
		sr := sampleRates[version][srIdx]
		if sr == 0 {
			continue
		}

		br := 0
		if int(brIdx) < len(bitrates) {
			br = bitrates[brIdx]
		}
		// In a full scan, we might skip free-format frames (br == 0) unless we correctly parse their length,
		// but since they don't give a fast length, we just guess based on standard formulas.
		if br == 0 {
			offset, ok := parseFreeFormatLength(data, i, version)
			if ok {
				totalSamples += spf
				i += offset - 1
			}
			continue
		}

		frameSize := (spf/8*br*bitrateMultiple)/sr + padding
		totalSamples += spf
		i += max(1, frameSize-1)
	}

	return decutil.AudioDuration(totalSamples, sampleRate)
}

func isSync(b0, b1 byte) bool {
	return b0 == syncByte0 && b1&syncMask1 == syncMask1
}

func readFrameInfo(b1, b2, b3 byte) (mp3Info, bool) {
	version := (b1 >> versionShift) & versionMask
	srIdx := (b2 >> srShift) & srMask
	brIdx := b2 >> bitrateShift
	channelMode := (b3 >> channelShift) & channelMask

	if int(version) >= len(sampleRates) || int(srIdx) >= len(sampleRates[version]) {
		return mp3Info{}, false
	}
	sr := sampleRates[version][srIdx]
	if sr == 0 {
		return mp3Info{}, false
	}

	channels := 2
	if channelMode == channelMono {
		channels = 1
	}

	br := 0
	if int(brIdx) < len(bitrates) {
		br = bitrates[brIdx]
	}

	return mp3Info{
		version:    int(version),
		mode:       int(channelMode),
		modeExt:    int((b3 >> modeExtShift) & modeExtMask),
		sampleRate: sr,
		channels:   channels,
		bitrate:    br,
		hasCRC:     (b1 & crcProtMask) == 0,
	}, true
}

func parseFreeFormatLength(data []byte, start int, version byte) (int, bool) {
	for i := start + 1; i < len(data)-3; i++ {
		if isSync(data[i], data[i+1]) {
			info, ok := readFrameInfo(data[i+1], data[i+2], data[i+3])
			if ok && info.version == int(version) {
				return i - start, true
			}
		}
	}
	return 0, false
}

func skipID3v2(data []byte) []byte {
	if len(data) < id3v2HeaderSize || !bytes.Equal(data[:3], magicID3) {
		return data
	}
	size := int(data[6]&id3v2SizeMask)<<id3v2Shift0 |
		int(data[7]&id3v2SizeMask)<<id3v2Shift1 |
		int(data[8]&id3v2SizeMask)<<id3v2Shift2 |
		int(data[9]&id3v2SizeMask)
	end := size + id3v2HeaderSize
	if end > len(data) {
		return data
	}
	return data[end:]
}
