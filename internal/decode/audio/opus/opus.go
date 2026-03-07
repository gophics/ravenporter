package opus

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
	pion "github.com/pion/opus"
)

const (
	oggMagic    = "OggS"
	headTag     = "OpusHead"
	tagsTag     = "OpusTags"
	probeLen    = 64
	opusSR      = 48000
	headTagLen  = 8
	granuleOff  = 6
	minPageSize = 14

	channelOff = 9
	preSkipOff = 10
	srOff      = 12
	gainOff    = 16
	minPayload = 19

	maxFrameSize        = 5760
	maxChannels         = 2
	gainDivisor         = 20.0 * 256.0
	minTagLen           = 4
	pictureMin          = 32
	mimeSkip            = 8
	descBase            = 12
	dimSkip             = 16
	searchMax           = 65536
	maxEstimatedSamples = 100_000_000

	tagR128Track = "R128_TRACK_GAIN"
	tagR128Album = "R128_ALBUM_GAIN"

	stereoBit  = 0x04
	configMask = 0x1F
	configShft = 3

	parseBits = 16
	parseBase = 10
	powBase   = 10

	tocSamples2500us = 120
	tocSamples5ms    = 240
	tocSamples10ms   = 480
	tocSamples20ms   = 960
	tocSamples40ms   = 1920
	tocSamples60ms   = 2880
)

var (
	errBadHead = errors.New("invalid OpusHead")
)

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatOpus, Decoder: &Decoder{}}}
}

type Decoder struct{}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return decutil.ProbeRead(r, probeLen, func(buf []byte) bool {
		if len(buf) < probeLen || string(buf[:4]) != oggMagic {
			return false
		}
		for i := range len(buf) - headTagLen {
			if string(buf[i:i+headTagLen]) == headTag {
				return true
			}
		}
		return false
	})
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatOpus, "file too large", err)
	}

	rawBytes, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOpus, "failed to read file", err)
	}
	byteReader := bytes.NewReader(rawBytes)

	demuxer := acquireOggDemuxer(byteReader)
	defer releaseOggDemuxer(demuxer)

	err = demuxer.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOpus, "failed to read id header", err)
	}
	info, err := parseHead(demuxer.packet.data)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOpus, err.Error(), err)
	}

	err = demuxer.readNextPacket()
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatOpus, "failed to read tags header", err)
	}
	metadata := parseTags(demuxer.packet.data)
	gainScalar := computeGainScalar(info.outputGain, demuxer.packet.data)

	lastGranule := scanLastGranule(bytes.NewReader(rawBytes))
	estSamples := estimateSamples(lastGranule, info.channelCount)

	clip := &ir.AudioClip{
		Name:        "Opus Audio",
		Format:      ir.AudioOpus,
		SampleRate:  opusSR,
		Layout:      decutil.LayoutFromChannels(info.channelCount),
		BitDepth:    ir.BitDepth16,
		Duration:    calcDuration(lastGranule, info.preSkip),
		LoopStart:   ir.NoIndex,
		LoopEnd:     ir.NoIndex,
		Metadata:    metadata,
		Compressed:  rawBytes,
		SourceCodec: ir.AudioOpus,
	}

	capturedInfo := info
	capturedOpts := opts
	clip.SampleDecode = func(c *ir.AudioClip) ([]float32, error) {
		decodeDemuxer := acquireOggDemuxer(bytes.NewReader(c.Compressed))
		defer releaseOggDemuxer(decodeDemuxer)

		// Advance past the two headers we parsed
		if err := decodeDemuxer.readNextPacket(); err != nil {
			return nil, err
		}
		if err := decodeDemuxer.readNextPacket(); err != nil {
			return nil, err
		}

		sysCtx := capturedOpts.Context
		if sysCtx == nil {
			sysCtx = context.Background()
		}

		audioData, finalInfo, err := decodeAudioPackets(sysCtx, decodeDemuxer, capturedInfo, gainScalar, estSamples)
		if err != nil {
			return nil, err
		}

		if capturedOpts.MaxAudioSamples > 0 && len(audioData) > capturedOpts.MaxAudioSamples {
			return nil, decutil.DecodeErr(ir.FormatOpus, "audio sample limit exceeded", nil)
		}

		preSkipSamples := finalInfo.preSkip * finalInfo.channelCount
		if len(audioData) > preSkipSamples {
			audioData = audioData[preSkipSamples:]
		}
		return audioData, nil
	}

	return &ir.Asset{
		AudioClips: []*ir.AudioClip{clip},
		Metadata:   ir.AssetMetadata{SourceFormat: ir.FormatOpus},
	}, nil
}

func decodeAudioPackets(
	sysCtx context.Context, demuxer *oggDemuxer, info *opusInfo, gainScalar float32, capHint int,
) ([]float32, *opusInfo, error) {
	decoder := pion.NewDecoder()
	audioData := make([]float32, 0, capHint)
	pcmBuf := make([]float32, maxFrameSize*maxChannels)

	for {
		if err := sysCtx.Err(); err != nil {
			return audioData, info, nil
		}
		err := demuxer.readNextPacket()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, info, decutil.DecodeErr(ir.FormatOpus, "packet read failed", err)
		}

		if demuxer.packet.bos {
			info, gainScalar, decoder = handleChainLink(demuxer, info, gainScalar, decoder)
			continue
		}

		if len(demuxer.packet.data) == 0 {
			continue
		}

		if !safeDecodeFloat32(decoder, demuxer.packet.data, pcmBuf) {
			continue
		}

		produced := tocFrameSamples(demuxer.packet.data[0])
		validBuf := pcmBuf[:produced]
		if gainScalar != 1.0 {
			for i := range validBuf {
				validBuf[i] *= gainScalar
			}
		}

		audioData = append(audioData, validBuf...)
	}

	return audioData, info, nil
}

func safeDecodeFloat32(dec pion.Decoder, data []byte, pcmBuf []float32) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	_, _, err := dec.DecodeFloat32(data, pcmBuf)
	return err == nil
}

func handleChainLink(
	demuxer *oggDemuxer, info *opusInfo, gainScalar float32, dec pion.Decoder,
) (*opusInfo, float32, pion.Decoder) {
	newInfo, headErr := parseHead(demuxer.packet.data)
	if headErr != nil {
		return info, gainScalar, dec
	}

	err := demuxer.readNextPacket()
	if err != nil {
		return newInfo, 1.0, pion.NewDecoder()
	}

	scalar := computeGainScalar(newInfo.outputGain, demuxer.packet.data)
	return newInfo, scalar, pion.NewDecoder()
}

func computeGainScalar(headerGain int16, tagsData []byte) float32 {
	r128 := parseR128TagGain(tagsData)
	total := int32(headerGain) + int32(r128)
	if total == 0 {
		return 1.0
	}
	return float32(math.Pow(powBase, float64(total)/gainDivisor))
}

func (d *Decoder) Extensions() []string { return []string{".opus"} }
func (d *Decoder) FormatName() string   { return "Opus" }

type opusInfo struct {
	channelCount int
	preSkip      int
	sampleRate   int
	outputGain   int16
}

func parseHead(data []byte) (*opusInfo, error) {
	if len(data) < headTagLen || !bytes.HasPrefix(data, []byte(headTag)) {
		return nil, errBadHead
	}
	if len(data) < minPayload {
		return nil, errBadHead
	}

	payload := data[headTagLen:]
	return &opusInfo{
		channelCount: int(payload[channelOff-headTagLen]),
		preSkip:      int(binread.ReadU16LE(payload[preSkipOff-headTagLen:])),
		sampleRate:   int(binread.ReadU32LE(payload[srOff-headTagLen:])),
		outputGain:   int16(binread.ReadU16LE(payload[gainOff-headTagLen:])), //nolint:gosec // Q7.8 intentional
	}, nil
}

// tocFrameSamples maps the Opus TOC config to output sample count at 48kHz.
// Config numbers are defined by RFC 6716 Section 3.1.
//
//nolint:mnd
func tocFrameSamples(toc byte) int {
	config := (toc >> configShft) & configMask
	switch config {
	case 16, 20, 24, 28:
		return tocSamples2500us
	case 17, 21, 25, 29:
		return tocSamples5ms
	case 0, 4, 8, 12, 14, 18, 22, 26, 30:
		return tocSamples10ms
	case 1, 5, 9, 13, 15, 19, 23, 27, 31:
		return tocSamples20ms
	case 2, 6, 10:
		return tocSamples40ms
	case 3, 7, 11:
		return tocSamples60ms
	default:
		return tocSamples20ms
	}
}

func estimateSamples(granule int64, channels int) int {
	if granule <= 0 || channels == 0 {
		return 0
	}
	est := int(granule) * channels
	if est < 0 || est > maxEstimatedSamples {
		return 0
	}
	return est
}

func scanLastGranule(r detect.ReadSeekerAt) int64 {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0
	}
	searchDist := int64(searchMax)
	if size < searchDist {
		searchDist = size
	}

	buf := pool.GetBuffer(int(searchDist))
	defer pool.PutBuffer(buf)

	_, err = r.ReadAt(buf[:searchDist], size-searchDist)
	if err != nil && err != io.EOF {
		return 0
	}

	for i := int(searchDist) - minPageSize; i >= 0; i-- {
		if string(buf[i:i+4]) == oggMagic && i+minPageSize <= int(searchDist) {
			return int64(binread.ReadU64LE(buf[i+granuleOff:])) //nolint:gosec
		}
	}
	return 0
}

func calcDuration(lastGranule int64, preSkip int) time.Duration {
	if lastGranule > 0 {
		return decutil.AudioDuration(max(0, int(lastGranule)-preSkip), opusSR)
	}
	return 0
}

func parseTags(data []byte) ir.AudioMetadata {
	tag := []byte(tagsTag)
	if bytes.HasPrefix(data, tag) {
		md := decutil.ParseVorbisComment(data[len(tag):])
		parseAlbumArt(data[len(tag):], &md)
		return md
	}
	return ir.AudioMetadata{}
}

func parseR128TagGain(data []byte) int16 {
	tag := []byte(tagsTag)
	if !bytes.HasPrefix(data, tag) {
		return 0
	}
	payload := data[len(tag):]
	if len(payload) < minTagLen {
		return 0
	}
	vendorLen := int(binread.ReadU32LE(payload[:minTagLen]))
	if len(payload) < minTagLen+vendorLen+minTagLen {
		return 0
	}
	payload = payload[minTagLen+vendorLen:]
	comments := int(binread.ReadU32LE(payload[:minTagLen]))
	payload = payload[minTagLen:]

	for i := 0; i < comments && len(payload) >= minTagLen; i++ {
		length := int(binread.ReadU32LE(payload[:minTagLen]))
		if len(payload) < minTagLen+length {
			break
		}
		comment := string(payload[minTagLen : minTagLen+length])
		payload = payload[minTagLen+length:]

		key, value, ok := strings.Cut(comment, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		if upper == tagR128Track || upper == tagR128Album {
			v, parseErr := strconv.ParseInt(value, parseBase, parseBits)
			if parseErr == nil {
				return int16(v)
			}
		}
	}
	return 0
}

func parseAlbumArt(data []byte, md *ir.AudioMetadata) {
	if len(data) < minTagLen {
		return
	}
	vendorLen := int(binread.ReadU32LE(data[:minTagLen]))
	if len(data) < minTagLen+vendorLen+minTagLen {
		return
	}
	data = data[minTagLen+vendorLen:]
	comments := int(binread.ReadU32LE(data[:minTagLen]))
	data = data[minTagLen:]

	prefix := []byte("METADATA_BLOCK_PICTURE=")
	for i := 0; i < comments && len(data) >= minTagLen; i++ {
		length := int(binread.ReadU32LE(data[:minTagLen]))
		if len(data) < minTagLen+length {
			break
		}
		comment := data[minTagLen : minTagLen+length]
		if bytes.HasPrefix(comment, prefix) {
			extractPicture(comment[len(prefix):], md)
		}
		data = data[minTagLen+length:]
	}
}

func extractPicture(b64 []byte, md *ir.AudioMetadata) {
	raw, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil || len(raw) < pictureMin {
		return
	}

	mimeLen := int(binread.ReadU32BE(raw[minTagLen:mimeSkip]))
	if len(raw) < mimeSkip+mimeLen+minTagLen {
		return
	}
	descLen := int(binread.ReadU32BE(raw[mimeSkip+mimeLen : mimeSkip+mimeLen+minTagLen]))
	if len(raw) < descBase+mimeLen+descLen+dimSkip+minTagLen {
		return
	}

	picOff := descBase + mimeLen + descLen + dimSkip
	picLen := int(binread.ReadU32BE(raw[picOff : picOff+minTagLen]))
	if len(raw) < picOff+minTagLen+picLen {
		return
	}

	md.Artwork = append([]byte(nil), raw[picOff+minTagLen:picOff+minTagLen+picLen]...)
}
