package flac

import (
	"bytes"
	"context"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "FLAC"
	extFLAC    = ".flac"

	magicLen       = 4
	metaHeaderSize = 4
	streamInfoSize = 34
	streamInfoType = 0
	vorbisComment  = 4
	pictureBlock   = 6
	metaLastMask   = 0x80
	metaTypeMask   = 0x7F

	srShift         = 12
	srMask          = 0xFFFFF
	bitsShift       = 4
	bitsMask        = 0x1F
	channelShift    = 9
	channelMask     = 0x7
	totalSampleMask = 0xF
	totalShift      = 32
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatFLAC, Decoder: &Decoder{}}}
}

var magic = []byte("fLaC")

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return decutil.ProbeBytes(r, magic) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatFLAC, "file too large", err)
	}
	raw, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatFLAC, "failed to read", err)
	}

	if len(raw) < magicLen+metaHeaderSize+streamInfoSize || !bytes.Equal(raw[:4], magic) {
		return nil, decutil.DecodeErr(ir.FormatFLAC, "file too short or invalid magic bytes", nil)
	}

	info, meta, audioStart := parseMetadata(raw)
	clip := &ir.AudioClip{
		Name:        "FLAC Audio",
		Format:      ir.AudioFLAC,
		SampleRate:  info.sampleRate,
		Layout:      decutil.LayoutFromChannels(info.channels),
		BitDepth:    decutil.BitDepthFromBits(info.bitsPerSample),
		Duration:    decutil.AudioDuration(info.totalSamples, info.sampleRate),
		LoopStart:   ir.NoIndex,
		LoopEnd:     ir.NoIndex,
		Metadata:    meta,
		Compressed:  raw,
		SourceCodec: ir.AudioFLAC,
	}

	capturedInfo := info
	capturedAudioData := raw[audioStart:]
	clip.SampleDecode = func(_ *ir.AudioClip) ([]float32, error) {
		samples := decodeFrames(context.Background(), capturedAudioData, capturedInfo)
		if opts.MaxAudioSamples > 0 && len(samples) > opts.MaxAudioSamples {
			return nil, decutil.DecodeErr(ir.FormatFLAC, "audio sample limit exceeded", nil)
		}
		return samples, nil
	}

	return &ir.Asset{
		AudioClips: []*ir.AudioClip{clip},
		Metadata:   ir.AssetMetadata{SourceFormat: ir.FormatFLAC},
	}, nil
}

func (d *Decoder) Extensions() []string { return []string{extFLAC} }
func (d *Decoder) FormatName() string   { return formatName }

type flacInfo struct {
	sampleRate    int
	channels      int
	bitsPerSample int
	totalSamples  int
}

func parseMetadata(data []byte) (flacInfo, ir.AudioMetadata, int) {
	var info flacInfo
	var meta ir.AudioMetadata
	if len(data) < magicLen+metaHeaderSize+streamInfoSize {
		return info, meta, len(data)
	}

	pos := magicLen
	for pos+metaHeaderSize <= len(data) {
		header := data[pos]
		isLast := header&metaLastMask != 0
		blockType := header & metaTypeMask
		blockSize := int(data[pos+1])<<16 | int(data[pos+2])<<8 | int(data[pos+3])
		pos += metaHeaderSize

		if pos+blockSize > len(data) {
			break
		}

		block := data[pos : pos+blockSize]
		switch blockType {
		case streamInfoType:
			if blockSize >= streamInfoSize {
				parseStreamInfo(block, &info)
			}
		case vorbisComment:
			meta = decutil.ParseVorbisComment(block)
		case pictureBlock:
			decutil.ExtractPictureComment(block, &meta)
		}

		pos += blockSize
		if isLast {
			break
		}
	}
	return info, meta, pos
}

func parseStreamInfo(block []byte, info *flacInfo) {
	packed := binread.ReadU32BE(block[10:])
	info.sampleRate = int(packed>>srShift) & srMask
	info.channels = int((packed>>channelShift)&channelMask) + 1
	info.bitsPerSample = int((packed>>bitsShift)&bitsMask) + 1
	totalHigh := int64(block[13]&totalSampleMask) << totalShift
	totalLow := int64(binread.ReadU32BE(block[14:]))
	info.totalSamples = int(totalHigh | totalLow)
}
