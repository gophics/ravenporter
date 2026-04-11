// Package decutil provides shared utility functions for all decoders.
package decutil

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/rperr"
)

const maxProbeLen = 32

// ReadSeekerAt is the unified stream interface for decoders.
type ReadSeekerAt interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

// ReadAll reads the entire content from a seeker, resetting position to start.
func ReadAll(r ReadSeekerAt) ([]byte, error) {
	if br, ok := r.(*bytes.Reader); ok {
		if _, err := br.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		n := br.Len()
		buf := make([]byte, n)
		_, err := br.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		return buf, nil
	}

	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return io.ReadAll(r)
	}

	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		_, _ = r.Seek(pos, io.SeekStart) //nolint:errcheck // reset
		return io.ReadAll(r)
	}

	remaining := size - pos
	if remaining <= 0 {
		_, _ = r.Seek(pos, io.SeekStart) //nolint:errcheck // reset
		return nil, nil
	}

	if _, err = r.Seek(pos, io.SeekStart); err != nil {
		return nil, err
	}

	buf := make([]byte, remaining)
	if _, err = io.ReadFull(r, buf); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return buf, nil
}

func ReadAllLimit(r io.Reader, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		return io.ReadAll(r)
	}

	lr := &io.LimitedReader{R: r, N: maxSize + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, errFileTooLarge
	}
	return data, nil
}

func HasASCIIPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := range len(prefix) {
		if foldASCIIByte(s[i]) != prefix[i] {
			return false
		}
	}
	return true
}

func HasASCIISuffixFold(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	start := len(s) - len(suffix)
	for i := range len(suffix) {
		if foldASCIIByte(s[start+i]) != suffix[i] {
			return false
		}
	}
	return true
}

func BytesContainsASCIIFold(buf []byte, needle string) bool {
	if needle == "" || len(buf) < len(needle) {
		return false
	}
	for i := 0; i <= len(buf)-len(needle); i++ {
		if bytesMatchASCIIFold(buf[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func bytesMatchASCIIFold(buf []byte, needle string) bool {
	for i := range len(needle) {
		if foldASCIIByte(buf[i]) != needle[i] {
			return false
		}
	}
	return true
}

func foldASCIIByte[T ~byte](b T) byte {
	if b >= 'A' && b <= 'Z' {
		return byte(b + ('a' - 'A'))
	}
	return byte(b)
}

var errFileTooLarge = errors.New("file exceeds MaxFileSize limit")

// CheckMaxFileSize returns an error if the data exceeds the configured limit.
func CheckMaxFileSize(data []byte, maxSize int64) error {
	if maxSize > 0 && int64(len(data)) > maxSize {
		return errFileTooLarge
	}
	return nil
}

// CheckStreamSize checks file size via seek without reading the full file.
func CheckStreamSize(r io.Seeker, maxSize int64) error {
	if maxSize <= 0 {
		return nil
	}
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if _, err := r.Seek(pos, io.SeekStart); err != nil {
		return err
	}
	if size-pos > maxSize {
		return errFileTooLarge
	}
	return nil
}

// ProbeBytes checks if the reader starts with the given magic bytes.
func ProbeBytes(r io.ReadSeeker, magic []byte) bool {
	if len(magic) > maxProbeLen {
		return false
	}
	buf := pool.GetBuffer(maxProbeLen)
	defer pool.PutBuffer(buf)

	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	n, _ := io.ReadFull(r, buf[:len(magic)]) //nolint:errcheck // length-checked below
	_, seekErr := r.Seek(pos, io.SeekStart)
	if seekErr != nil {
		return false
	}
	return n == len(magic) && bytes.Equal(buf[:n], magic)
}

func ProbeContains(r io.ReadSeeker, magic []byte) bool {
	buf := pool.GetBuffer(maxProbeLen)
	defer pool.PutBuffer(buf)

	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	n, _ := r.Read(buf) //nolint:errcheck // length-checked below
	_, seekErr := r.Seek(pos, io.SeekStart)
	if seekErr != nil {
		return false
	}
	return bytes.Contains(buf[:n], magic)
}

// ProbeRead reads up to size bytes from the current position, passes them to
// match, and restores the seek position. Handles pool allocation internally.
func ProbeRead(r io.ReadSeeker, size int, match func([]byte) bool) bool {
	buf := pool.GetBuffer(size)
	defer pool.PutBuffer(buf)

	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	n, _ := r.Read(buf[:size]) //nolint:errcheck // length passed to match
	_, seekErr := r.Seek(pos, io.SeekStart)
	if seekErr != nil || n == 0 {
		return false
	}
	return match(buf[:n])
}

// DecodeErr creates a standard DecodeError for any format.
func DecodeErr(format ir.FormatID, msg string, cause error) error {
	e := &rperr.DecodeError{Format: format, Offset: -1, Message: msg}
	if cause != nil {
		e.Cause = cause
	}
	return e
}

// AudioDuration computes a time.Duration from sample count and sample rate.
func AudioDuration(samples, sampleRate int) time.Duration {
	if sampleRate <= 0 {
		return 0
	}
	return time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
}

// LayoutFromChannels returns the ChannelLayout for a given channel count.
func LayoutFromChannels(ch int) ir.ChannelLayout {
	switch {
	case ch <= 1:
		return ir.LayoutMono
	case ch <= 2: //nolint:mnd // stereo
		return ir.LayoutStereo
	case ch <= 6: //nolint:mnd // 5.1
		return ir.Layout5_1
	default:
		return ir.Layout7_1
	}
}

// BitDepthFromBits converts a bits-per-sample value to an ir.BitDepth.
func BitDepthFromBits(bits int) ir.BitDepth {
	switch bits {
	case 8: //nolint:mnd // bit-depth value
		return ir.BitDepth8
	case 16: //nolint:mnd // bit-depth value
		return ir.BitDepth16
	case 24: //nolint:mnd // bit-depth value
		return ir.BitDepth24
	case 32: //nolint:mnd // bit-depth value
		return ir.BitDepth32
	case 64: //nolint:mnd // bit-depth value
		return ir.BitDepth64
	default:
		return ir.BitDepth16
	}
}

// ColorToFactor converts 8-bit RGB to an [4]float32 RGBA factor (alpha = 1.0).
func ColorToFactor(r, g, b byte) [4]float32 {
	return [4]float32{float32(r) / 255.0, float32(g) / 255.0, float32(b) / 255.0, 1.0}
}

// ParseF32 parses a string into a float32 value, returning 0.0 on failure.
func ParseF32(s string) float32 {
	v, _ := strconv.ParseFloat(s, 32) //nolint:errcheck // 0.0 on failure
	return float32(v)
}

// SplitFields splits s on spaces into buf, reusing the backing array.
func SplitFields(s string, buf []string) []string {
	buf = buf[:0]
	for s != "" {
		i := strings.IndexByte(s, ' ')
		if i < 0 {
			buf = append(buf, s)
			break
		}
		if i > 0 {
			buf = append(buf, s[:i])
		}
		s = s[i+1:]
	}
	return buf
}

// SplitByteFields splits b on spaces into buf, reusing the backing array.
func SplitByteFields(b []byte, buf [][]byte) [][]byte {
	buf = buf[:0]
	for len(b) > 0 {
		i := bytes.IndexByte(b, ' ')
		if i < 0 {
			buf = append(buf, b)
			break
		}
		if i > 0 {
			buf = append(buf, b[:i])
		}
		b = b[i+1:]
	}
	return buf
}

// LineScanner iterates over lines in a byte slice without allocating strings.
type LineScanner struct {
	Data []byte
	Pos  int
}

// Next returns the next non-empty trimmed line, or nil at EOF.
func (s *LineScanner) Next() []byte {
	for s.Pos < len(s.Data) {
		start := s.Pos
		end := bytes.IndexByte(s.Data[start:], '\n')
		if end < 0 {
			s.Pos = len(s.Data)
			if line := bytes.TrimSpace(s.Data[start:]); len(line) > 0 {
				return line
			}
			return nil
		}
		s.Pos = start + end + 1
		if line := bytes.TrimSpace(s.Data[start : start+end]); len(line) > 0 {
			return line
		}
	}
	return nil
}

// Bstr converts a byte slice to a string without copying.
func Bstr(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b)) //nolint:gosec // zero-copy for strconv
}

func ParseFloat32Bytes(b []byte) (float32, error) {
	f, err := strconv.ParseFloat(Bstr(b), 32)
	return float32(f), err
}

func ParseIntBytes(b []byte) (int, error) {
	return strconv.Atoi(Bstr(b))
}
