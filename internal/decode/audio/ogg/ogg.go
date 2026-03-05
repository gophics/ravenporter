package ogg

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	pageMagic     = "OggS"
	vorbisIDMagic = "\x01vorbis"
	probeSize     = 64
	maxPCMCap     = 256 << 20 // 256M samples max pre-alloc (~1 GB per channel)
)

var errCodebookMax = errors.New("codebook length exceeded allowed Vorbis range")

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatOGG, Decoder: &Decoder{}}}
}

type Decoder struct{}

type frameDecoder struct {
	setup         *vorbisSetup
	synth         *vorbisSynth
	channelData   [][]float32
	imdctBuf      []float32
	pcmOut        [][]float32
	prevBlock     [][]float32
	prevSize      int
	currentSerial uint32
	noResidue     []bool
	chToDecode    []int
	overlapBuf    []float32
	vqClasses     []int
}

func newFrameDecoder(setup *vorbisSetup, synth *vorbisSynth, serial uint32, pcmCap int) *frameDecoder {
	maxSize := setup.blocksize1
	if pcmCap < maxSize {
		pcmCap = maxSize
	}
	fd := &frameDecoder{
		setup:         setup,
		synth:         synth,
		currentSerial: serial,
		channelData:   make([][]float32, setup.channels),
		imdctBuf:      make([]float32, maxSize),
		pcmOut:        make([][]float32, setup.channels),
		prevBlock:     make([][]float32, setup.channels),
		noResidue:     make([]bool, setup.channels),
		chToDecode:    make([]int, 0, setup.channels),
		overlapBuf:    make([]float32, maxSize),
		vqClasses:     make([]int, 0, 64), //nolint:mnd // reasonable pre-alloc for VQ classes
	}

	for i := range setup.channels {
		fd.channelData[i] = make([]float32, maxSize)
		fd.pcmOut[i] = make([]float32, 0, pcmCap)
		fd.prevBlock[i] = make([]float32, maxSize)
	}
	return fd
}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeRead(r, probeSize, func(buf []byte) bool {
		if len(buf) < 32 { //nolint:mnd // minimum OGG page size
			return false
		}
		if !bytes.Equal(buf[:4], []byte(pageMagic)) {
			return false
		}
		return bytes.Contains(buf, []byte(vorbisIDMagic))
	})
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "file too large", err)
	}

	rawBytes, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "failed to read file", err)
	}

	byteReader := bytes.NewReader(rawBytes)
	pcmCap := int(readLastGranulePos(byteReader))
	if pcmCap <= 0 || pcmCap > maxPCMCap {
		pcmCap = 0
	}

	if _, err := byteReader.Seek(0, io.SeekStart); err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "seek failed", err)
	}
	demux := newDemuxer(byteReader)
	var setup vorbisSetup

	pkt, err := demux.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "failed to read id header", err)
	}
	if err := readIdentificationHeader(pkt.data, &setup); err != nil {
		return nil, err
	}

	pkt, err = demux.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "failed to read comment header", err)
	}
	if err := readCommentHeader(pkt.data, &setup); err != nil {
		return nil, err
	}

	pkt, err = demux.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "failed to read setup header", err)
	}
	if err := readSetupHeader(pkt.data, &setup); err != nil {
		return nil, err
	}

	clip := &ir.AudioClip{
		Name:        "OGG Audio",
		Format:      ir.AudioOGG,
		SampleRate:  setup.sampleRate,
		Layout:      decutil.LayoutFromChannels(setup.channels),
		BitDepth:    ir.BitDepth16,
		Duration:    decutil.AudioDuration(pcmCap, setup.sampleRate),
		LoopStart:   ir.NoIndex,
		LoopEnd:     ir.NoIndex,
		Metadata:    setup.audioMetadata,
		Compressed:  rawBytes,
		SourceCodec: ir.AudioOGG,
	}

	capturedOpts := opts
	clip.SampleDecode = func(c *ir.AudioClip) ([]float32, error) {
		return decodeOGGSamples(c.Compressed, pcmCap, capturedOpts)
	}

	return &ir.Asset{
		AudioClips: []*ir.AudioClip{clip},
		Metadata:   ir.AssetMetadata{SourceFormat: ir.FormatOGG},
	}, nil
}

func decodeOGGSamples(raw []byte, pcmCap int, opts detect.DecodeOptions) ([]float32, error) {
	byteReader := bytes.NewReader(raw)
	demux := newDemuxer(byteReader)
	var setup vorbisSetup

	pkt, err := demux.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "failed to read id header", err)
	}
	if err := readIdentificationHeader(pkt.data, &setup); err != nil {
		return nil, err
	}

	pkt, err = demux.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "failed to read comment header", err)
	}
	if err := readCommentHeader(pkt.data, &setup); err != nil {
		return nil, err
	}

	pkt, err = demux.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOGG, "failed to read setup header", err)
	}
	if err := readSetupHeader(pkt.data, &setup); err != nil {
		return nil, err
	}

	synth := newSynth(setup.blocksize0, setup.blocksize1)
	fd := newFrameDecoder(&setup, synth, pkt.serial, pcmCap)

	sysCtx := opts.Context
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	for {
		if err := sysCtx.Err(); err != nil {
			break
		}
		pkt, err := demux.readNextPacket()
		if err != nil {
			break
		}

		if pkt.serial != fd.currentSerial {
			continue
		}

		if err := decodePacket(fd, pkt.data); err != nil {
			continue
		}
	}

	var samples []float32
	if len(fd.pcmOut) > 0 {
		totalSamples := len(fd.pcmOut[0])
		samples = make([]float32, totalSamples*setup.channels)
		for i := range totalSamples {
			base := i * setup.channels
			for ch := range setup.channels {
				samples[base+ch] = fd.pcmOut[ch][i]
			}
		}
	}

	if opts.MaxAudioSamples > 0 && len(samples) > opts.MaxAudioSamples {
		return nil, decutil.DecodeErr(ir.FormatOGG, "audio sample limit exceeded", nil)
	}

	return samples, nil
}

func (d *Decoder) Extensions() []string { return []string{".ogg", ".oga"} }
func (d *Decoder) FormatName() string   { return "OGG" }
