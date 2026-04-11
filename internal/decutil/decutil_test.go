package decutil

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/rperr"
)

func TestReadAll(t *testing.T) {
	data := []byte("hello")

	buf, err := ReadAll(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, data, buf)

	reader := &readOnlyReader{Buffer: bytes.NewBuffer(data)}
	buf, err = ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, data, buf)
}

func TestCheckSizes(t *testing.T) {
	assert.NoError(t, CheckMaxFileSize([]byte("abc"), 4))
	assert.ErrorIs(t, CheckMaxFileSize([]byte("abcd"), 3), errFileTooLarge)

	assert.NoError(t, CheckStreamSize(bytes.NewReader([]byte("abcd")), 4))
	assert.ErrorIs(t, CheckStreamSize(bytes.NewReader([]byte("abcd")), 3), errFileTooLarge)
}

func TestProbeHelpers(t *testing.T) {
	reader := bytes.NewReader([]byte("hello world"))

	assert.True(t, ProbeBytes(reader, []byte("hello")))
	pos, _ := reader.Seek(0, io.SeekCurrent)
	assert.EqualValues(t, 0, pos)

	assert.True(t, ProbeContains(reader, []byte("world")))
	assert.True(t, ProbeRead(reader, 5, func(data []byte) bool {
		return bytes.Equal(data, []byte("hello"))
	}))
	assert.False(t, ProbeBytes(reader, bytes.Repeat([]byte("a"), maxProbeLen+1)))
}

func TestDecodeErrAndValueHelpers(t *testing.T) {
	baseErr := errors.New("boom")
	err := DecodeErr(ir.FormatOBJ, "bad data", baseErr)
	var decodeErr *rperr.DecodeError
	require.ErrorAs(t, err, &decodeErr)
	assert.Equal(t, ir.FormatOBJ, decodeErr.Format)
	assert.Equal(t, "bad data", decodeErr.Message)
	assert.ErrorIs(t, decodeErr, baseErr)

	assert.Equal(t, time.Second, AudioDuration(48000, 48000))
	assert.Equal(t, ir.LayoutMono, LayoutFromChannels(1))
	assert.Equal(t, ir.LayoutStereo, LayoutFromChannels(2))
	assert.Equal(t, ir.Layout5_1, LayoutFromChannels(6))
	assert.Equal(t, ir.Layout7_1, LayoutFromChannels(8))
	assert.Equal(t, ir.BitDepth24, BitDepthFromBits(24))
	assert.Equal(t, ir.BitDepth64, BitDepthFromBits(64))
	assert.Equal(t, ir.BitDepth16, BitDepthFromBits(12))
	assert.Equal(t, [4]float32{1, 128.0 / 255.0, 0, 1}, ColorToFactor(255, 128, 0))
	assert.InDelta(t, 1.25, ParseF32("1.25"), 0.0001)
	assert.Zero(t, ParseF32("bad"))
}

func TestSplitHelpersAndScanner(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, SplitFields(" a  b c ", nil))

	fields := SplitByteFields([]byte(" a  b c "), nil)
	require.Len(t, fields, 3)
	assert.Equal(t, "a", string(fields[0]))
	assert.Equal(t, "b", string(fields[1]))
	assert.Equal(t, "c", string(fields[2]))

	scanner := &LineScanner{Data: []byte("\n one \n\n two\n")}
	assert.Equal(t, "one", string(scanner.Next()))
	assert.Equal(t, "two", string(scanner.Next()))
	assert.Nil(t, scanner.Next())

	assert.Equal(t, "abc", Bstr([]byte("abc")))
	v, err := ParseFloat32Bytes([]byte("1.5"))
	require.NoError(t, err)
	assert.InDelta(t, 1.5, v, 0.0001)
	i, err := ParseIntBytes([]byte("-12"))
	require.NoError(t, err)
	assert.Equal(t, -12, i)
}

func TestPCMAndCodecHelpers(t *testing.T) {
	dst := make([]float32, 2)
	Decode8Bit([]byte{128, 255}, dst)
	assert.InDelta(t, 0, dst[0], 0.0001)
	assert.InDelta(t, 127.0/128.0, dst[1], 0.0001)

	dst = make([]float32, 1)
	Decode16LE([]byte{0xFF, 0x7F}, dst)
	assert.InDelta(t, 32767.0/32768.0, dst[0], 0.0001)

	Decode24LE([]byte{0xFF, 0xFF, 0x7F}, dst)
	assert.InDelta(t, 8388607.0/8388608.0, dst[0], 0.0001)

	Decode32LE([]byte{0xFF, 0xFF, 0xFF, 0x7F}, dst)
	assert.InDelta(t, 2147483647.0/2147483648.0, dst[0], 0.0001)

	DecodeIEEEFloat([]byte{0x00, 0x00, 0x00, 0x3F}, dst, Bytes32)
	assert.InDelta(t, 0.5, dst[0], 0.0001)
	DecodeIEEEFloat([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xE0, 0x3F}, dst, Bytes64)
	assert.InDelta(t, 0.5, dst[0], 0.0001)
	DecodeIEEEFloatBE([]byte{0x3F, 0x00, 0x00, 0x00}, dst, Bytes32)
	assert.InDelta(t, 0.5, dst[0], 0.0001)
	DecodeIEEEFloatBE([]byte{0x3F, 0xE0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, dst, Bytes64)
	assert.InDelta(t, 0.5, dst[0], 0.0001)

	DecodeAlaw([]byte{0xD5}, dst)
	assert.NotZero(t, dst[0])
	DecodeUlaw([]byte{0xFF}, dst)
	assert.NotZero(t, dst[0])
}

func TestExtractPictureComment(t *testing.T) {
	meta := &ir.AudioMetadata{}
	ExtractPictureComment([]byte{1, 2, 3}, meta)
	assert.Empty(t, meta.Comment)

	block := make([]byte, picMinSize)
	off := 4
	block[off+3] = 3
	off += 4 + 3
	block[off+3] = 5
	off += 4
	copy(block[off:off+5], "cover")

	ExtractPictureComment(block, meta)
	assert.Equal(t, "cover", meta.Comment)
}

type readOnlyReader struct {
	*bytes.Buffer
}

func (r *readOnlyReader) ReadAt(p []byte, off int64) (int, error) {
	return copy(p, r.Bytes()[off:]), nil
}

func (r *readOnlyReader) Seek(_ int64, _ int) (int64, error) {
	return 0, errors.New("seek unsupported")
}
