package psd

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strconv"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	psdFormatName = "PSD"
	psdName       = "psd"
	extPSD        = ".psd"
	extPSB        = ".psb"
)

var magicPSD = []byte("8BPS")

const (
	psdHeaderSize = 26
	psdChannelOff = 12
	psdHeightOff  = 14
	psdWidthOff   = 18
	psdDepthOff   = 22
	psdModeOff    = 24

	psdBlockLenSize  = 4
	psdLayerCountMin = 2

	psdCompRaw     = 0
	psdCompRLE     = 1
	psdCompZIP     = 2
	psdCompZIPPred = 3

	psdRowLenSize = 2

	psdModeGrayscale = 1
	psdModeRGB       = 3
	psdModeCMYK      = 4

	psdOutChannels = 4

	psdDepth8  = 8
	psdDepth16 = 16
	psdDepth32 = 32

	psdBitsPerByte   = 8
	psdMaxByte       = 255
	psdMaxUint16     = 65535.0
	psdRoundingBias  = 0.5
	psdMinRGBSamples = 3
	psdCMYKChannels  = 4

	metaBitDepth  = "BitDepth"
	metaColorMode = "ColorMode"
	metaLayers    = "LayerCount"
)

var (
	errPSDImageDataTruncated = errors.New("psd: image data section truncated")
	errPSDUnsupportedComp    = errors.New("psd: unsupported compression type")
)

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatPSD, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return imgutil.ProbeBytes(r, magicPSD) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(psdName, err)
	}

	w, h := psdDimensions(raw)
	depth, mode := psdDepthAndMode(raw)

	if err := imgutil.CheckPixelLimit(w, h, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(psdName, err)
	}

	decoded := &ir.ImageAsset{
		Name:       psdName,
		Format:     ir.ImagePSD,
		Width:      w,
		Height:     h,
		Channels:   ir.ChannelRGBA,
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
		Metadata:   make(map[string]string, psdCMYKChannels),
	}

	decoded.Metadata[metaBitDepth] = strconv.Itoa(depth)
	decoded.Metadata[metaColorMode] = strconv.Itoa(mode)

	parsePSDBlocks(raw, decoded)

	decoded.PixelDecode = func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
		return readPSDCompositePixels(d.Compressed, d.Width, d.Height)
	}

	return imgutil.BuildAsset(decoded, ir.FormatPSD), nil
}

func parsePSDBlocks(data []byte, decoded *ir.ImageAsset) {
	depth, _ := psdDepthAndMode(data)

	pos := psdHeaderSize
	if pos+psdBlockLenSize > len(data) {
		return
	}
	cmLen := int(binread.ReadU32BE(data[pos:]))
	pos += psdBlockLenSize + cmLen

	if pos+psdBlockLenSize > len(data) {
		return
	}
	irLen := int(binread.ReadU32BE(data[pos:]))
	pos += psdBlockLenSize + irLen

	if pos+psdBlockLenSize > len(data) {
		return
	}
	lmLen := int(binread.ReadU32BE(data[pos:]))
	if lmLen > 0 && pos+psdBlockLenSize+lmLen <= len(data) {
		parseLayerMaskInfo(data[pos+psdBlockLenSize:pos+psdBlockLenSize+lmLen], decoded, depth)
	}
}

func parseLayerMaskInfo(data []byte, decoded *ir.ImageAsset, depth int) {
	if len(data) < psdBlockLenSize {
		return
	}
	layerInfoLen := int(binread.ReadU32BE(data[0:]))
	if layerInfoLen == 0 || layerInfoLen+psdBlockLenSize > len(data) {
		return
	}

	layerData := data[psdBlockLenSize : psdBlockLenSize+layerInfoLen]
	if len(layerData) < psdLayerCountMin {
		return
	}

	rawCount := binread.ReadU16BE(layerData[0:])
	layerCount := int(rawCount)
	if int16(rawCount) < 0 { //nolint:gosec
		layerCount = -int(int16(rawCount)) //nolint:gosec
	}

	if layerCount > 0 {
		decoded.Metadata[metaLayers] = strconv.Itoa(layerCount)
	}

	if depth == 16 || depth == 32 {
		decoded.Metadata["PSDFloatLDR"] = "true"
	}
}

func psdDimensions(data []byte) (w, h int) {
	if len(data) < psdHeaderSize {
		return 0, 0
	}
	h = int(binread.ReadU32BE(data[psdHeightOff:]))
	w = int(binread.ReadU32BE(data[psdWidthOff:]))
	return w, h
}

func psdDepthAndMode(data []byte) (depth, mode int) {
	if len(data) < psdHeaderSize {
		return 0, 0
	}
	depth = int(binread.ReadU16BE(data[psdDepthOff:]))
	mode = int(binread.ReadU16BE(data[psdModeOff:]))
	return depth, mode
}

func (d *Decoder) Extensions() []string { return []string{extPSD, extPSB} }
func (d *Decoder) FormatName() string   { return psdFormatName }

func psdChannelCount(data []byte) int {
	if len(data) < psdHeaderSize {
		return 0
	}
	return int(binread.ReadU16BE(data[psdChannelOff:]))
}

func psdImageDataOffset(data []byte) int {
	if len(data) < psdHeaderSize {
		return -1
	}
	pos := psdHeaderSize

	if pos+psdBlockLenSize > len(data) {
		return -1
	}
	pos += psdBlockLenSize + int(binread.ReadU32BE(data[pos:]))

	if pos+psdBlockLenSize > len(data) {
		return -1
	}
	pos += psdBlockLenSize + int(binread.ReadU32BE(data[pos:]))

	if pos+psdBlockLenSize > len(data) {
		return -1
	}
	pos += psdBlockLenSize + int(binread.ReadU32BE(data[pos:]))

	return pos
}

func readPSDCompositePixels(data []byte, w, h int) (*ir.PixelBuffer, error) {
	channels := psdChannelCount(data)
	depth, mode := psdDepthAndMode(data)
	if channels < 1 || w < 1 || h < 1 || depth < psdBitsPerByte {
		return nil, errPSDImageDataTruncated
	}

	bytesPerSample := depth / psdBitsPerByte
	rowBytes := w * bytesPerSample

	pos := psdImageDataOffset(data)
	if pos < 0 || pos+2 > len(data) {
		return nil, errPSDImageDataTruncated
	}

	comp := int(binread.ReadU16BE(data[pos:]))
	pos += 2

	totalScanlines := h * channels

	planar, err := decompressPlanar(data[pos:], comp, totalScanlines, rowBytes)
	if err != nil {
		return nil, err
	}

	rgba := planarToRGBA(planar, w, h, channels, depth, mode, bytesPerSample)

	return &ir.PixelBuffer{
		Data:     rgba,
		DataType: ir.DataTypeUint8,
		BitDepth: ir.BitDepth8,
	}, nil
}

func decompressPlanar(data []byte, comp, totalScanlines, rowBytes int) ([]byte, error) {
	totalSize := totalScanlines * rowBytes
	planar := make([]byte, totalSize)

	switch comp {
	case psdCompRaw:
		if len(data) < totalSize {
			return nil, errPSDImageDataTruncated
		}
		copy(planar, data[:totalSize])

	case psdCompRLE:
		rowTableSize := totalScanlines * psdRowLenSize
		if len(data) < rowTableSize {
			return nil, errPSDImageDataTruncated
		}
		rowLengths := make([]int, totalScanlines)
		for i := range totalScanlines {
			rowLengths[i] = int(binread.ReadU16BE(data[i*psdRowLenSize:]))
		}
		pos := rowTableSize
		dst := 0
		for i := range totalScanlines {
			rl := rowLengths[i]
			if pos+rl > len(data) || dst+rowBytes > totalSize {
				return nil, errPSDImageDataTruncated
			}
			packbitsDecodeScanline(data[pos:pos+rl], planar[dst:dst+rowBytes])
			pos += rl
			dst += rowBytes
		}

	case psdCompZIP, psdCompZIPPred:
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, errPSDImageDataTruncated
		}
		n, err := io.ReadFull(r, planar)
		_ = r.Close() //nolint:errcheck // best-effort cleanup of decompressor
		if err != nil || n < totalSize {
			return nil, errPSDImageDataTruncated
		}
		if comp == psdCompZIPPred {
			undoPrediction(planar, rowBytes)
		}

	default:
		return nil, errPSDUnsupportedComp
	}

	return planar, nil
}

func undoPrediction(planar []byte, rowBytes int) {
	for off := 0; off+rowBytes <= len(planar); off += rowBytes {
		row := planar[off : off+rowBytes]
		for i := 1; i < len(row); i++ {
			row[i] += row[i-1]
		}
	}
}

func planarToRGBA(planar []byte, w, h, channels, depth, mode, bytesPerSample int) []byte {
	planeSize := w * h * bytesPerSample
	rgba := make([]byte, w*h*psdOutChannels)
	samples := make([]float64, channels)

	for y := range h {
		for x := range w {
			idx := y*w + x
			out := idx * psdOutChannels
			readSamples(samples, planar, channels, planeSize, idx, bytesPerSample, depth)
			r, g, b, a := convertToRGBA(samples, mode, channels)
			rgba[out] = r
			rgba[out+1] = g
			rgba[out+2] = b
			rgba[out+3] = a
		}
	}

	return rgba
}

func readSamples(vals []float64, planar []byte, channels, planeSize, pixelIdx, bytesPerSample, depth int) {
	for c := range channels {
		off := c*planeSize + pixelIdx*bytesPerSample
		if off+bytesPerSample > len(planar) {
			vals[c] = 0
			continue
		}
		switch depth {
		case psdDepth8:
			vals[c] = float64(planar[off]) / psdMaxByte
		case psdDepth16:
			vals[c] = float64(binary.BigEndian.Uint16(planar[off:])) / psdMaxUint16
		case psdDepth32:
			vals[c] = float64(math.Float32frombits(binary.BigEndian.Uint32(planar[off:])))
		default:
			vals[c] = 0
		}
	}
}

func convertToRGBA(samples []float64, mode, channels int) (r, g, b, a byte) {
	a = 0xFF

	switch mode {
	case psdModeGrayscale:
		v := clampByte(samples[0])
		r, g, b = v, v, v
		if channels > 1 {
			a = clampByte(samples[1])
		}

	case psdModeCMYK:
		c := samples[0]
		m := samples[1]
		y := samples[2]
		k := samples[3]
		r = clampByte((1 - c) * (1 - k))
		g = clampByte((1 - m) * (1 - k))
		b = clampByte((1 - y) * (1 - k))
		if channels > psdCMYKChannels {
			a = clampByte(samples[psdCMYKChannels])
		}

	default:
		r = clampByte(samples[0])
		if len(samples) > 1 {
			g = clampByte(samples[1])
		}
		if len(samples) > psdMinRGBSamples-1 {
			b = clampByte(samples[2])
		}
		if channels >= psdOutChannels {
			a = clampByte(samples[3])
		}
	}

	return r, g, b, a
}

func clampByte(v float64) byte {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return psdMaxByte
	}
	return byte(v*psdMaxByte + psdRoundingBias)
}

func packbitsDecodeScanline(src, dst []byte) {
	si, di := 0, 0
	for si < len(src) && di < len(dst) {
		n := int(int8(src[si]))
		si++

		switch {
		case n >= 0:
			count := n + 1
			if si+count > len(src) || di+count > len(dst) {
				return
			}
			copy(dst[di:], src[si:si+count])
			si += count
			di += count

		case n > -128:
			count := 1 - n
			if si >= len(src) {
				return
			}
			v := src[si]
			si++
			end := min(di+count, len(dst))
			for di < end {
				dst[di] = v
				di++
			}

		default:
			// n == -128: no-op
		}
	}
}
