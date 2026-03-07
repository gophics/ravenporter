package opus

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	pion "github.com/pion/opus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPicture(t *testing.T) {
	picData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	mime := "image/jpeg"

	var raw []byte
	putU32BE := func(v uint32) {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, v)
		raw = append(raw, b...)
	}

	putU32BE(3)
	putU32BE(uint32(len(mime)))
	raw = append(raw, []byte(mime)...)
	putU32BE(0)   // descLen
	putU32BE(100) // width
	putU32BE(100) // height
	putU32BE(24)  // depth
	putU32BE(0)   // colors
	putU32BE(uint32(len(picData)))
	raw = append(raw, picData...)

	b64 := []byte(base64.StdEncoding.EncodeToString(raw))

	md := &ir.AudioMetadata{}
	extractPicture(b64, md)
	require.NotEmpty(t, md.Artwork)
	assert.Equal(t, picData, md.Artwork)
}

func TestExtractPicture_Invalid(t *testing.T) {
	md := &ir.AudioMetadata{}
	extractPicture([]byte("not-valid-base64!!!"), md)
	assert.Empty(t, md.Artwork)

	extractPicture([]byte(base64.StdEncoding.EncodeToString([]byte{0, 1, 2})), md)
	assert.Empty(t, md.Artwork)
}

func TestParseAlbumArtWithPicture(t *testing.T) {
	picData := []byte{0x89, 0x50, 0x4E, 0x47}
	mime := "image/png"
	var picBin []byte
	putU32BE := func(v uint32) {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, v)
		picBin = append(picBin, b...)
	}
	putU32BE(3)
	putU32BE(uint32(len(mime)))
	picBin = append(picBin, []byte(mime)...)
	putU32BE(0) // descLen
	putU32BE(0)
	putU32BE(0)
	putU32BE(0)
	putU32BE(0)
	putU32BE(uint32(len(picData)))
	picBin = append(picBin, picData...)

	b64Str := base64.StdEncoding.EncodeToString(picBin)
	comment := "METADATA_BLOCK_PICTURE=" + b64Str

	vendor := "test"
	var data []byte
	putU32LE := func(v uint32) {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, v)
		data = append(data, b...)
	}
	putU32LE(uint32(len(vendor)))
	data = append(data, []byte(vendor)...)
	putU32LE(1)
	putU32LE(uint32(len(comment)))
	data = append(data, []byte(comment)...)

	md := &ir.AudioMetadata{}
	parseAlbumArt(data, md)
	require.NotEmpty(t, md.Artwork)
	assert.Equal(t, picData, md.Artwork)
}

func TestParseAlbumArt_NoComments(t *testing.T) {
	md := &ir.AudioMetadata{}
	parseAlbumArt([]byte{}, md)
	assert.Empty(t, md.Artwork)
}

func TestComputeGainScalar(t *testing.T) {
	assert.Equal(t, float32(1.0), computeGainScalar(0, nil))
	assert.NotEqual(t, float32(1.0), computeGainScalar(256, nil))
}

func TestTocFrameSamples(t *testing.T) {
	for _, toc := range []byte{0x00, 0x08, 0x10} {
		got := tocFrameSamples(toc)
		assert.True(t, got > 0, "toc=0x%02x should produce >0 samples", toc)
	}
}

func TestResync(t *testing.T) {
	var buf []byte
	buf = append(buf, make([]byte, 100)...)
	buf = append(buf, 'O', 'g', 'g', 'S')
	buf = append(buf, make([]byte, 100)...)

	r := bytes.NewReader(buf)
	d := acquireOggDemuxer(r)
	_, _ = r.Seek(0, io.SeekStart)
	assert.True(t, d.resync())
}

func TestResync_NotFound(t *testing.T) {
	r := bytes.NewReader(make([]byte, 200))
	d := acquireOggDemuxer(r)
	assert.False(t, d.resync())
}

func TestHandleChainLink(t *testing.T) {
	head := buildOpusHead(2, 312, 48000)
	tagsPayload := buildOpusTagsOrdered([][2]string{{"TITLE", "Test"}})

	var stream []byte
	stream = append(stream, buildOggPage(0, flagBOS, 0, 1, head)...)
	stream = append(stream, buildOggPage(1, 0, 0, 1, tagsPayload)...)

	r := bytes.NewReader(stream)
	d := acquireOggDemuxer(r)

	err := d.readNextPacket()
	require.NoError(t, err)
	assert.True(t, d.packet.bos)

	info := &opusInfo{channelCount: 1, sampleRate: 48000}
	newInfo, scalar, _ := handleChainLink(d, info, 1.0, pion.NewDecoder())
	assert.Equal(t, 2, newInfo.channelCount)
	assert.True(t, scalar > 0)
}

func TestReadPageAndRelease(t *testing.T) {
	page := buildOggPage(0, flagBOS|flagEOS, 960, 7, []byte{1, 2, 3})
	d := acquireOggDemuxer(bytes.NewReader(page))
	require.NoError(t, d.readPage())
	assert.Equal(t, uint8(flagBOS|flagEOS), d.page.headerType)
	assert.Equal(t, uint32(7), d.page.serial)
	assert.Equal(t, []byte{1, 2, 3}, d.page.body)

	releaseOggDemuxer(d)
	assert.Nil(t, d.buf)
	assert.Nil(t, d.bodyBuf)
	assert.Nil(t, d.packetBuf)
}

func TestReadPageBadMagic(t *testing.T) {
	d := acquireOggDemuxer(bytes.NewReader(append([]byte("BAD!"), make([]byte, pageHeaderSize-4)...)))
	err := d.readPage()
	require.ErrorIs(t, err, errBadMagic)
}

func TestReadNextPacketSkipsForeignSerialWithoutBOS(t *testing.T) {
	stream := append(
		buildOggPage(0, flagBOS, 0, 1, []byte("OpusHead................")),
		buildOggPage(1, 0, 0, 2, []byte("foreign"))...,
	)

	d := acquireOggDemuxer(bytes.NewReader(stream))
	require.NoError(t, d.readNextPacket())
	assert.Equal(t, uint32(1), d.packet.serial)
	err := d.readNextPacket()
	require.Error(t, err)
}

func TestDecoderDecodeRejectsOversizedInput(t *testing.T) {
	decoder := &Decoder{}
	_, err := decoder.Decode(bytes.NewReader([]byte("too-large")), detect.DecodeOptions{MaxFileSize: 4})
	require.Error(t, err)
}

func TestDecoderDecodeRejectsInvalidHeaders(t *testing.T) {
	t.Run("bad id header", func(t *testing.T) {
		stream := buildOggPage(0, flagBOS, 0, 1, []byte("invalid"))
		_, err := (&Decoder{}).Decode(bytes.NewReader(stream), detect.DecodeOptions{})
		require.Error(t, err)
	})

	t.Run("missing tags header", func(t *testing.T) {
		stream := buildOggPage(0, flagBOS, 0, 1, buildOpusHead(2, 312, 48000))
		_, err := (&Decoder{}).Decode(bytes.NewReader(stream), detect.DecodeOptions{})
		require.Error(t, err)
	})
}

func TestDecodeAudioPacketsHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := acquireOggDemuxer(bytes.NewReader(nil))
	defer releaseOggDemuxer(d)

	audioData, finalInfo, err := decodeAudioPackets(ctx, d, &opusInfo{channelCount: 1}, 1.0, 16)
	require.NoError(t, err)
	assert.Empty(t, audioData)
	assert.Equal(t, 1, finalInfo.channelCount)
}

func TestDecodeAudioPacketsHandlesReadError(t *testing.T) {
	d := acquireOggDemuxer(&failingReadSeekerAt{Reader: bytes.NewReader(nil), err: errors.New("boom")})
	defer releaseOggDemuxer(d)

	audioData, finalInfo, err := decodeAudioPackets(context.Background(), d, &opusInfo{channelCount: 1}, 1.0, 16)
	require.Error(t, err)
	assert.Nil(t, audioData)
	assert.Equal(t, 1, finalInfo.channelCount)
}

func TestDecodeAudioPacketsSkipsChainLinksAndInvalidPackets(t *testing.T) {
	stream := append(
		buildOggPage(0, flagBOS, 0, 1, []byte("bad-head")),
		buildOggPage(1, 0, 0, 1, nil)...,
	)
	stream = append(stream, buildOggPage(2, flagEOS, 960, 1, []byte{0xff})...)

	d := acquireOggDemuxer(bytes.NewReader(stream))
	defer releaseOggDemuxer(d)

	audioData, finalInfo, err := decodeAudioPackets(context.Background(), d, &opusInfo{channelCount: 1}, 1.0, 16)
	require.NoError(t, err)
	assert.Empty(t, audioData)
	assert.Equal(t, 1, finalInfo.channelCount)
}

func TestSafeDecodeFloat32(t *testing.T) {
	assert.False(t, safeDecodeFloat32(pion.NewDecoder(), []byte{0xff}, make([]float32, maxFrameSize*maxChannels)))

	var zeroDecoder pion.Decoder
	assert.True(t, safeDecodeFloat32(zeroDecoder, []byte{0x08}, make([]float32, maxFrameSize*maxChannels)))
}

func TestHandleChainLinkFallbacks(t *testing.T) {
	t.Run("invalid head keeps current decoder state", func(t *testing.T) {
		d := acquireOggDemuxer(bytes.NewReader(nil))
		defer releaseOggDemuxer(d)
		d.packet.data = []byte("bad-head")

		info := &opusInfo{channelCount: 1, sampleRate: opusSR}
		decoder := pion.NewDecoder()
		nextInfo, scalar, nextDecoder := handleChainLink(d, info, 0.5, decoder)
		assert.Same(t, info, nextInfo)
		assert.Equal(t, float32(0.5), scalar)
		assert.NotNil(t, nextDecoder)
	})

	t.Run("missing tags resets decoder", func(t *testing.T) {
		stream := buildOggPage(0, flagBOS, 0, 1, buildOpusHead(2, 312, 48000))
		d := acquireOggDemuxer(bytes.NewReader(stream))
		defer releaseOggDemuxer(d)

		require.NoError(t, d.readNextPacket())

		nextInfo, scalar, nextDecoder := handleChainLink(d, &opusInfo{channelCount: 1}, 0.5, pion.NewDecoder())
		assert.Equal(t, 2, nextInfo.channelCount)
		assert.Equal(t, float32(1.0), scalar)
		assert.NotNil(t, nextDecoder)
	})
}

func TestCalcDuration(t *testing.T) {
	assert.Equal(t, time.Second, calcDuration(opusSR+312, 312))
	assert.Zero(t, calcDuration(0, 312))
}

func TestExtractPictureRejectsTruncatedPayloads(t *testing.T) {
	putU32BE := func(dst *[]byte, v uint32) {
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, v)
		*dst = append(*dst, buf...)
	}

	raw := make([]byte, 0, pictureMin)
	putU32BE(&raw, 3)
	putU32BE(&raw, 64)
	raw = append(raw, bytes.Repeat([]byte{'a'}, 8)...)

	md := &ir.AudioMetadata{}
	extractPicture([]byte(base64.StdEncoding.EncodeToString(raw)), md)
	assert.Empty(t, md.Artwork)
}

func buildOpusHead(channels, preSkip int, sampleRate uint32) []byte {
	head := make([]byte, 19)
	copy(head, "OpusHead")
	head[8] = 1
	head[9] = byte(channels)
	binary.LittleEndian.PutUint16(head[10:], uint16(preSkip))
	binary.LittleEndian.PutUint32(head[12:], sampleRate)
	binary.LittleEndian.PutUint16(head[16:], 0)
	head[18] = 0
	return head
}

type failingReadSeekerAt struct {
	*bytes.Reader
	err error
}

func (r *failingReadSeekerAt) Read([]byte) (int, error) {
	return 0, r.err
}
