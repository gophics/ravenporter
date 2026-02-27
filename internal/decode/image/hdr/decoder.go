package hdr

import (
	"bytes"
	"errors"
	"io"
	"unsafe"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pixel"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

const (
	hdrFormatName    = "HDR"
	hdrName          = "hdr"
	extHDR           = ".hdr"
	rgbeChannels     = 3
	rgbeStride       = 4
	rgbeRLEMarker    = 0x02
	rgbeRLEMin       = 8
	rgbeMaxRLEWidth  = 32768
	rgbeRunThreshold = 128
	rleWidthShift    = 8
	dimPrefixLen     = 2
	decimalBase      = 10
)

var (
	magicRadiance  = []byte("#?RADIANCE")
	magicRGBE      = []byte("#?RGBE")
	errHDRBadMagic = errors.New("not a Radiance HDR file")
	errHDRNoSize   = errors.New("missing image dimensions")
	errHDRTrunc    = errors.New("truncated HDR data")
)

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatHDR, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return imgutil.ProbeBytes(r, magicRadiance)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(hdrName, err)
	}

	w, h, dataOff, decErr := parseHDR(raw)
	if decErr != nil {
		return nil, imgutil.DecodeErrStr(hdrName, decErr)
	}

	if err := imgutil.CheckPixelLimit(w, h, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(hdrName, err)
	}

	decoded := &ir.ImageAsset{
		Name:        hdrName,
		Format:      ir.ImageHDR,
		Width:       w,
		Height:      h,
		Channels:    ir.ChannelRGB,
		ColorSpace:  ir.ColorLinear,
		MipLevels:   1,
		Compressed:  raw,
		PixelDecode: hdrPixelDecode(dataOff),
	}

	return imgutil.BuildAsset(decoded, ir.FormatHDR), nil
}

func (d *Decoder) Extensions() []string { return []string{extHDR} }
func (d *Decoder) FormatName() string   { return hdrFormatName }

func hdrPixelDecode(dataOff int) ir.PixelDecodeFunc {
	return func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
		if d.Width == 0 || d.Height == 0 {
			return &ir.PixelBuffer{DataType: ir.DataTypeFloat32, BitDepth: ir.BitDepth32}, nil
		}
		pixels, err := readAllScanlines(d.Compressed, dataOff, d.Width, d.Height)
		if err != nil {
			return nil, err
		}
		return &ir.PixelBuffer{
			DataType: ir.DataTypeFloat32,
			Data:     unsafe.Slice((*byte)(unsafe.Pointer(&pixels[0])), len(pixels)*4), //nolint:gosec,mnd // fast slice cast
			BitDepth: ir.BitDepth32,
		}, nil
	}
}

func parseHDR(data []byte) (w, h, dataOff int, err error) {
	pos := skipHeader(data)
	if pos < 0 {
		return 0, 0, 0, errHDRBadMagic
	}

	w, h, pos = parseDimensions(data, pos)
	if w <= 0 || h <= 0 {
		return 0, 0, 0, errHDRNoSize
	}
	return w, h, pos, nil
}

func skipHeader(data []byte) int {
	lineEnd := bytes.IndexByte(data, '\n')
	if lineEnd < 0 {
		return -1
	}

	if !bytes.HasPrefix(data, magicRadiance) && !bytes.HasPrefix(data, magicRGBE) {
		return -1
	}

	pos := lineEnd + 1
	for pos < len(data) {
		idx := bytes.IndexByte(data[pos:], '\n')
		if idx < 0 {
			return -1
		}
		lineEnd = pos + idx
		if isBlankLine(data, pos, lineEnd) {
			return lineEnd + 1
		}
		pos = lineEnd + 1
	}
	return -1
}

func parseDimensions(data []byte, pos int) (w, h, end int) {
	idx := bytes.IndexByte(data[pos:], '\n')
	lineEnd := -1
	if idx >= 0 {
		lineEnd = pos + idx
	}
	if lineEnd < 0 {
		return 0, 0, pos
	}

	line := data[pos:lineEnd]
	end = lineEnd + 1

	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}

		key := byte(0)
		if (line[i] == '-' || line[i] == '+') && i+1 < len(line) {
			key = line[i+1]
			i += dimPrefixLen
		} else {
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				i++
			}
			continue
		}

		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		val := 0
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			val = val*decimalBase + int(line[i]-'0')
			i++
		}

		switch key {
		case 'X':
			w = val
		case 'Y':
			h = val
		}
	}
	return w, h, end
}

func readAllScanlines(data []byte, pos, w, h int) ([]float32, error) {
	pixels := make([]float32, w*h*rgbeChannels)
	scanline := pool.GetBuffer(w * rgbeStride)
	defer pool.PutBuffer(scanline)

	for y := range h {
		var err error
		pos, err = readScanline(data, pos, scanline[:w*rgbeStride], w)
		if err != nil {
			return nil, err
		}
		convertScanline(pixels, scanline[:w*rgbeStride], y, w)
	}
	return pixels, nil
}

func convertScanline(dst []float32, scanline []byte, y, w int) {
	for x := range w {
		dstOff := (y*w + x) * rgbeChannels
		srcOff := x * rgbeStride
		dst[dstOff], dst[dstOff+1], dst[dstOff+2] = pixel.RGBEToFloat(
			scanline[srcOff], scanline[srcOff+1], scanline[srcOff+2], scanline[srcOff+3],
		)
	}
}

func readScanline(data []byte, pos int, scanline []byte, w int) (int, error) {
	if pos+rgbeStride > len(data) {
		return pos, errHDRTrunc
	}

	if data[pos] != rgbeRLEMarker || data[pos+1] != rgbeRLEMarker || w < rgbeRLEMin || w >= rgbeMaxRLEWidth {
		return readOldStyleScanline(data, pos, scanline)
	}
	rleW := int(data[pos+2])<<rleWidthShift | int(data[pos+3])
	if rleW != w {
		return readOldStyleScanline(data, pos, scanline)
	}
	return readNewStyleRLE(data, pos+rgbeStride, scanline, w)
}

func readOldStyleScanline(data []byte, pos int, scanline []byte) (int, error) {
	need := len(scanline)
	if pos+need > len(data) {
		return pos, errHDRTrunc
	}
	copy(scanline, data[pos:pos+need])
	return pos + need, nil
}

func readNewStyleRLE(data []byte, pos int, scanline []byte, w int) (int, error) {
	for ch := range rgbeStride {
		var err error
		pos, err = decodeRLEChannel(data, pos, scanline, ch, w)
		if err != nil {
			return pos, err
		}
	}
	return pos, nil
}

func decodeRLEChannel(data []byte, pos int, scanline []byte, ch, w int) (int, error) {
	p := 0
	for p < w {
		if pos >= len(data) {
			return pos, errHDRTrunc
		}
		b := data[pos]
		pos++

		if b <= rgbeRunThreshold {
			count := int(b)
			for range count {
				if pos >= len(data) {
					return pos, errHDRTrunc
				}
				if p < w {
					scanline[p*rgbeStride+ch] = data[pos]
				}
				pos++
				p++
			}
			continue
		}

		count := int(b) - rgbeRunThreshold
		if pos >= len(data) {
			return pos, errHDRTrunc
		}
		val := data[pos]
		pos++
		for range count {
			if p < w {
				scanline[p*rgbeStride+ch] = val
			}
			p++
		}
	}
	return pos, nil
}

func isBlankLine(data []byte, start, end int) bool {
	for i := start; i < end; i++ {
		if data[i] != ' ' && data[i] != '\t' && data[i] != '\r' {
			return false
		}
	}
	return true
}
