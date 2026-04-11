package psd

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strconv"
	"unsafe"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pool"
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
	psdVersionOff = 4
	psdChannelOff = 12
	psdHeightOff  = 14
	psdWidthOff   = 18
	psdDepthOff   = 22
	psdModeOff    = 24

	psdBlockLenSize  = 4
	psbBlockLenSize  = 8
	psdLayerCountMin = 2

	psdCompRaw     = 0
	psdCompRLE     = 1
	psdCompZIP     = 2
	psdCompZIPPred = 3

	psdRowLenSize = 2
	psbRowLenSize = 4

	psdVersionPSD = 1
	psdVersionPSB = 2

	psdModeBitmap       = 0
	psdModeGrayscale    = 1
	psdModeIndexed      = 2
	psdModeRGB          = 3
	psdModeCMYK         = 4
	psdModeMultichannel = 7
	psdModeDuotone      = 8
	psdModeLab          = 9

	psdOutChannels = 4

	psdDepth1  = 1
	psdDepth8  = 8
	psdDepth16 = 16
	psdDepth32 = 32

	psdBitsPerByte      = 8
	psdBitmapHighBit    = 7
	psdMaxByte          = 255
	psdMaxUint16        = 65535.0
	psdUint16Bytes      = 2
	psdFloat32Bytes     = 4
	psdByteToUint16     = 0x101
	psdRoundingBias     = 0.5
	psdMinRGBSamples    = 3
	psdCMYKChannels     = 4
	psdThirdSample      = 2
	psdIndexedColors    = 256
	psdIndexedTableSize = psdIndexedColors * 3

	psdLabScaleL     = 100.0
	psdLabOffsetL    = 16.0
	psdLabDivisor    = 116.0
	psdLabScaleA     = 500.0
	psdLabScaleB     = 200.0
	psdLabByteScale  = 255.0
	psdLabByteCenter = 128.0
	psdLabRefX       = 0.96422
	psdLabRefZ       = 0.82521
	psdLabEpsilon    = 216.0 / 24389.0
	psdLabKappa      = 24389.0 / 27.0

	psdLabRGBM00 = 3.1338561
	psdLabRGBM01 = -1.6168667
	psdLabRGBM02 = -0.4906146
	psdLabRGBM10 = -0.9787684
	psdLabRGBM11 = 1.9161415
	psdLabRGBM12 = 0.0334540
	psdLabRGBM20 = 0.0719453
	psdLabRGBM21 = -0.2289914
	psdLabRGBM22 = 1.4052427

	psdSRGBThreshold   = 0.0031308
	psdSRGBLinearScale = 12.92
	psdSRGBGammaScale  = 1.055
	psdSRGBGammaBias   = 0.055
	psdSRGBGammaPower  = 2.4

	metaBitDepth  = "BitDepth"
	metaColorMode = "ColorMode"
	metaLayers    = "LayerCount"
)

var (
	errPSDImageDataTruncated = errors.New("psd: image data section truncated")
	errPSDUnsupportedComp    = errors.New("psd: unsupported compression type")
	errPSDUnsupportedMode    = errors.New("psd: unsupported color mode")
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
	version := psdVersion(data)

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
	layerLenSize := psdLayerBlockLenSize(version)
	if pos+layerLenSize > len(data) {
		return
	}
	lmLen := readPSDSectionLength(data[pos:], layerLenSize)
	if lmLen > 0 && pos+layerLenSize+lmLen <= len(data) {
		parseLayerMaskInfo(data[pos+layerLenSize:pos+layerLenSize+lmLen], decoded, depth, version)
	}
}

func parseLayerMaskInfo(data []byte, decoded *ir.ImageAsset, depth, version int) {
	layerLenSize := psdLayerBlockLenSize(version)
	if len(data) < layerLenSize {
		return
	}
	layerInfoLen := readPSDSectionLength(data, layerLenSize)
	if layerInfoLen == 0 || layerInfoLen+layerLenSize > len(data) {
		return
	}

	layerData := data[layerLenSize : layerLenSize+layerInfoLen]
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
	version := psdVersion(data)
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
	layerLenSize := psdLayerBlockLenSize(version)
	if pos+layerLenSize > len(data) {
		return -1
	}
	pos += layerLenSize + readPSDSectionLength(data[pos:], layerLenSize)

	return pos
}

func readPSDCompositePixels(data []byte, w, h int) (*ir.PixelBuffer, error) {
	channels := psdChannelCount(data)
	depth, mode := psdDepthAndMode(data)
	version := psdVersion(data)
	if channels < 1 || w < 1 || h < 1 || depth < psdDepth1 {
		return nil, errPSDImageDataTruncated
	}
	if err := validatePSDComposite(depth, mode, channels, psdColorModeData(data)); err != nil {
		return nil, err
	}
	colorModeData := psdColorModeData(data)

	rowBytes := psdRowBytes(w, depth)

	pos := psdImageDataOffset(data)
	if pos < 0 || pos+2 > len(data) {
		return nil, errPSDImageDataTruncated
	}

	comp := int(binread.ReadU16BE(data[pos:]))
	pos += 2

	totalScanlines := h * channels

	planar, err := decompressPlanar(data[pos:], comp, totalScanlines, rowBytes, psdRowLenFieldSize(version))
	if err != nil {
		return nil, err
	}
	defer pool.PutBuffer(planar)

	if depth == psdDepth16 {
		return &ir.PixelBuffer{
			Data:     planarToRGBA16(planar, colorModeData, w, h, channels, mode, rowBytes),
			DataType: ir.DataTypeUint16,
			BitDepth: ir.BitDepth16,
		}, nil
	}
	if depth == psdDepth32 {
		return &ir.PixelBuffer{
			Data:     planarToRGBA32(planar, colorModeData, w, h, channels, mode, rowBytes),
			DataType: ir.DataTypeFloat32,
			BitDepth: ir.BitDepth32,
		}, nil
	}

	return &ir.PixelBuffer{
		Data:     planarToRGBA8(planar, colorModeData, w, h, channels, depth, mode, rowBytes),
		DataType: ir.DataTypeUint8,
		BitDepth: ir.BitDepth8,
	}, nil
}

func psdColorModeData(data []byte) []byte {
	pos := psdHeaderSize
	if pos+psdBlockLenSize > len(data) {
		return nil
	}
	length := int(binread.ReadU32BE(data[pos:]))
	pos += psdBlockLenSize
	if pos+length > len(data) {
		return nil
	}
	return data[pos : pos+length]
}

func psdRowBytes(width, depth int) int {
	if depth == psdDepth1 {
		return (width + psdBitsPerByte - 1) / psdBitsPerByte
	}
	return width * (depth / psdBitsPerByte)
}

func validatePSDComposite(depth, mode, channels int, colorModeData []byte) error {
	switch mode {
	case psdModeBitmap:
		if depth != psdDepth1 {
			return errPSDUnsupportedMode
		}
	case psdModeIndexed:
		if depth != psdDepth8 || len(colorModeData) < psdIndexedTableSize {
			return errPSDUnsupportedMode
		}
	case psdModeLab:
		if channels < psdMinRGBSamples || depth == psdDepth1 {
			return errPSDUnsupportedMode
		}
	default:
		if depth == psdDepth1 {
			return errPSDUnsupportedMode
		}
	}
	return nil
}

func decompressPlanar(data []byte, comp, totalScanlines, rowBytes, rowLenSize int) ([]byte, error) {
	totalSize := totalScanlines * rowBytes
	planar := pool.GetBuffer(totalSize)

	switch comp {
	case psdCompRaw:
		if len(data) < totalSize {
			pool.PutBuffer(planar)
			return nil, errPSDImageDataTruncated
		}
		copy(planar, data[:totalSize])

	case psdCompRLE:
		rowTableSize := totalScanlines * rowLenSize
		if len(data) < rowTableSize {
			pool.PutBuffer(planar)
			return nil, errPSDImageDataTruncated
		}
		pos := rowTableSize
		dst := 0
		for i := range totalScanlines {
			off := i * rowLenSize
			rl := int(binread.ReadU16BE(data[off:]))
			if rowLenSize != psdRowLenSize {
				rl = int(binread.ReadU32BE(data[off:]))
			}
			if pos+rl > len(data) || dst+rowBytes > totalSize {
				pool.PutBuffer(planar)
				return nil, errPSDImageDataTruncated
			}
			packbitsDecodeScanline(data[pos:pos+rl], planar[dst:dst+rowBytes])
			pos += rl
			dst += rowBytes
		}

	case psdCompZIP, psdCompZIPPred:
		if err := psdZlibReader.DecompressInto(planar, data); err != nil {
			pool.PutBuffer(planar)
			return nil, errPSDImageDataTruncated
		}
		if comp == psdCompZIPPred {
			undoPrediction(planar, rowBytes)
		}

	default:
		pool.PutBuffer(planar)
		return nil, errPSDUnsupportedComp
	}

	return planar, nil
}

var psdZlibReader pool.ZlibReader

func psdVersion(data []byte) int {
	if len(data) < psdVersionOff+2 {
		return 0
	}
	return int(binread.ReadU16BE(data[psdVersionOff:]))
}

func psdLayerBlockLenSize(version int) int {
	if version == psdVersionPSB {
		return psbBlockLenSize
	}
	return psdBlockLenSize
}

func psdRowLenFieldSize(version int) int {
	if version == psdVersionPSB {
		return psbRowLenSize
	}
	return psdRowLenSize
}

func readPSDSectionLength(data []byte, size int) int {
	switch size {
	case psbBlockLenSize:
		length := binary.BigEndian.Uint64(data)
		if length > uint64(^uint(0)>>1) {
			return 0
		}
		return int(length)
	default:
		return int(binread.ReadU32BE(data))
	}
}

func undoPrediction(planar []byte, rowBytes int) {
	for off := 0; off+rowBytes <= len(planar); off += rowBytes {
		row := planar[off : off+rowBytes]
		for i := 1; i < len(row); i++ {
			row[i] += row[i-1]
		}
	}
}

func planarToRGBA8(planar, colorModeData []byte, w, h, channels, depth, mode, rowBytes int) []byte {
	planeSize := rowBytes * h
	bytesPerSample := 0
	if depth >= psdBitsPerByte {
		bytesPerSample = depth / psdBitsPerByte
	}

	rgba := make([]byte, w*h*psdOutChannels)
	var local [psdOutChannels]float64
	samples := local[:]
	if channels > len(local) {
		samples = make([]float64, channels)
	} else {
		samples = samples[:channels]
	}

	for y := range h {
		for x := range w {
			idx := y*w + x
			out := idx * psdOutChannels

			switch mode {
			case psdModeBitmap:
				v := bitmapSample(planar, rowBytes, x, y)
				rgba[out] = v
				rgba[out+1] = v
				rgba[out+2] = v
				rgba[out+3] = 0xFF
			case psdModeIndexed:
				r, g, b := indexedSample(planar, colorModeData, planeSize, idx)
				rgba[out] = r
				rgba[out+1] = g
				rgba[out+2] = b
				rgba[out+3] = 0xFF
				if channels > 1 {
					rgba[out+3] = clampByte(readSample(planar, planeSize, idx, bytesPerSample, depth, 1))
				}
			default:
				readSamples(samples, planar, channels, planeSize, idx, bytesPerSample, depth)
				r, g, b, a := convertToRGBA(samples, mode, channels)
				rgba[out] = r
				rgba[out+1] = g
				rgba[out+2] = b
				rgba[out+3] = a
			}
		}
	}

	return rgba
}

func planarToRGBA16(planar, colorModeData []byte, w, h, channels, mode, rowBytes int) []byte {
	planeSize := rowBytes * h
	rgba := make([]byte, w*h*psdOutChannels*psdUint16Bytes)
	var local [psdOutChannels]float64
	samples := local[:]
	if channels > len(local) {
		samples = make([]float64, channels)
	} else {
		samples = samples[:channels]
	}

	for y := range h {
		for x := range w {
			idx := y*w + x
			out := idx * psdOutChannels * psdUint16Bytes

			switch mode {
			case psdModeIndexed:
				r8, g8, b8 := indexedSample(planar, colorModeData, planeSize, idx)
				binary.LittleEndian.PutUint16(rgba[out:], uint16(r8)*psdByteToUint16)
				binary.LittleEndian.PutUint16(rgba[out+2:], uint16(g8)*psdByteToUint16)
				binary.LittleEndian.PutUint16(rgba[out+4:], uint16(b8)*psdByteToUint16)
				alpha := uint16(psdMaxUint16)
				if channels > 1 {
					alpha = clampUint16(readSample(planar, planeSize, idx, psdUint16Bytes, psdDepth16, 1))
				}
				binary.LittleEndian.PutUint16(rgba[out+6:], alpha)
			default:
				readSamples(samples, planar, channels, planeSize, idx, psdUint16Bytes, psdDepth16)
				r, g, b, a := convertToRGBA16(samples, mode, channels)
				binary.LittleEndian.PutUint16(rgba[out:], r)
				binary.LittleEndian.PutUint16(rgba[out+2:], g)
				binary.LittleEndian.PutUint16(rgba[out+4:], b)
				binary.LittleEndian.PutUint16(rgba[out+6:], a)
			}
		}
	}

	return rgba
}

func planarToRGBA32(planar, colorModeData []byte, w, h, channels, mode, rowBytes int) []byte {
	planeSize := rowBytes * h
	rgba := make([]float32, w*h*psdOutChannels)
	var local [psdOutChannels]float64
	samples := local[:]
	if channels > len(local) {
		samples = make([]float64, channels)
	} else {
		samples = samples[:channels]
	}

	for y := range h {
		for x := range w {
			idx := y*w + x
			out := idx * psdOutChannels

			switch mode {
			case psdModeBitmap:
				v := bitmapSample(planar, rowBytes, x, y)
				value := float32(v) / psdMaxByte
				rgba[out] = value
				rgba[out+1] = value
				rgba[out+2] = value
				rgba[out+3] = 1
			case psdModeIndexed:
				r8, g8, b8 := indexedSample(planar, colorModeData, planeSize, idx)
				rgba[out] = float32(r8) / psdMaxByte
				rgba[out+1] = float32(g8) / psdMaxByte
				rgba[out+2] = float32(b8) / psdMaxByte
				rgba[out+3] = 1
				if channels > 1 {
					rgba[out+3] = clampUnit32(readSample(planar, planeSize, idx, psdFloat32Bytes, psdDepth32, 1))
				}
			default:
				readSamples(samples, planar, channels, planeSize, idx, psdFloat32Bytes, psdDepth32)
				r, g, b, a := convertToRGBA32(samples, mode, channels)
				rgba[out] = r
				rgba[out+1] = g
				rgba[out+2] = b
				rgba[out+3] = a
			}
		}
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(&rgba[0])), len(rgba)*4) //nolint:gosec,mnd
}

func bitmapSample(planar []byte, rowBytes, x, y int) byte {
	off := y*rowBytes + x/psdBitsPerByte
	if off >= len(planar) {
		return 0
	}
	shift := psdBitmapHighBit - (x % psdBitsPerByte)
	if (planar[off]>>shift)&1 == 0 {
		return 0
	}
	return psdMaxByte
}

func indexedSample(planar, colorModeData []byte, planeSize, pixelIdx int) (r, g, b byte) {
	if pixelIdx >= planeSize || len(colorModeData) < psdIndexedTableSize {
		return 0, 0, 0
	}
	index := int(planar[pixelIdx])
	return colorModeData[index], colorModeData[psdIndexedColors+index], colorModeData[psdIndexedColors*2+index]
}

func readSamples(vals []float64, planar []byte, channels, planeSize, pixelIdx, bytesPerSample, depth int) {
	for c := range channels {
		vals[c] = readSample(planar, planeSize, pixelIdx, bytesPerSample, depth, c)
	}
}

func readSample(planar []byte, planeSize, pixelIdx, bytesPerSample, depth, channel int) float64 {
	off := channel*planeSize + pixelIdx*bytesPerSample
	if off+bytesPerSample > len(planar) {
		return 0
	}
	switch depth {
	case psdDepth8:
		return float64(planar[off]) / psdMaxByte
	case psdDepth16:
		return float64(binary.BigEndian.Uint16(planar[off:])) / psdMaxUint16
	case psdDepth32:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(planar[off:])))
	default:
		return 0
	}
}

func convertToRGBA(samples []float64, mode, channels int) (r, g, b, a byte) {
	rf, gf, bf, af := convertToNormalizedRGBA(samples, mode, channels)
	return clampByte(rf), clampByte(gf), clampByte(bf), clampByte(af)
}

func convertToRGBA16(samples []float64, mode, channels int) (r, g, b, a uint16) {
	rf, gf, bf, af := convertToNormalizedRGBA(samples, mode, channels)
	return clampUint16(rf), clampUint16(gf), clampUint16(bf), clampUint16(af)
}

func convertToRGBA32(samples []float64, mode, channels int) (r, g, b, a float32) {
	rf, gf, bf, af := convertToNormalizedRGBA(samples, mode, channels)
	return float32(rf), float32(gf), float32(bf), float32(af)
}

func convertToNormalizedRGBA(samples []float64, mode, channels int) (r, g, b, a float64) {
	a = 1

	switch mode {
	case psdModeGrayscale, psdModeDuotone:
		v := clampUnit(samples[0])
		r, g, b = v, v, v
		if channels > 1 {
			a = clampUnit(samples[1])
		}
	case psdModeCMYK:
		r, g, b = cmykToRGBUnit(samples[0], samples[1], samples[2], samples[3])
		if channels > psdCMYKChannels {
			a = clampUnit(samples[psdCMYKChannels])
		}
	case psdModeLab:
		r, g, b = labToRGBUnit(samples[0], samples[1], samples[2])
		if channels > psdMinRGBSamples {
			a = clampUnit(samples[psdMinRGBSamples])
		}
	case psdModeMultichannel:
		r, g, b = multichannelToRGBUnit(samples, channels)
	default:
		r = clampUnit(samples[0])
		if channels > 1 {
			g = clampUnit(samples[1])
		}
		if channels > psdMinRGBSamples-1 {
			b = clampUnit(samples[2])
		}
		if channels >= psdOutChannels {
			a = clampUnit(samples[3])
		}
	}

	return r, g, b, a
}

func cmykToRGBUnit(c, m, y, k float64) (r, g, b float64) {
	return clampUnit((1 - c) * (1 - k)),
		clampUnit((1 - m) * (1 - k)),
		clampUnit((1 - y) * (1 - k))
}

func multichannelToRGBUnit(samples []float64, channels int) (r, g, b float64) {
	switch {
	case channels <= 1:
		v := clampUnit(1 - samples[0])
		return v, v, v
	case channels >= psdCMYKChannels:
		return cmykToRGBUnit(samples[0], samples[1], samples[2], samples[3])
	default:
		c := samples[0]
		m := samples[1]
		y := 0.0
		if channels > psdThirdSample {
			y = samples[psdThirdSample]
		}
		return clampUnit(1 - c), clampUnit(1 - m), clampUnit(1 - y)
	}
}

func labToRGBUnit(l, a, b float64) (r, g, bl float64) {
	lf := l*psdLabScaleL + psdLabOffsetL
	fy := lf / psdLabDivisor
	fx := fy + ((a*psdLabByteScale)-psdLabByteCenter)/psdLabScaleA
	fz := fy - ((b*psdLabByteScale)-psdLabByteCenter)/psdLabScaleB

	x := labPivotInverse(fx) * psdLabRefX
	y := labPivotInverse(fy)
	z := labPivotInverse(fz) * psdLabRefZ

	rr := psdLabRGBM00*x + psdLabRGBM01*y + psdLabRGBM02*z
	gg := psdLabRGBM10*x + psdLabRGBM11*y + psdLabRGBM12*z
	bb := psdLabRGBM20*x + psdLabRGBM21*y + psdLabRGBM22*z

	return gammaEncodeUnit(rr), gammaEncodeUnit(gg), gammaEncodeUnit(bb)
}

func labPivotInverse(v float64) float64 {
	cube := v * v * v
	if cube > psdLabEpsilon {
		return cube
	}
	return (psdLabDivisor*v - psdLabOffsetL) / psdLabKappa
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

func clampUint16(v float64) uint16 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return uint16(psdMaxUint16)
	}
	return uint16(v*psdMaxUint16 + psdRoundingBias)
}

func gammaEncodeUnit(v float64) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	if v <= psdSRGBThreshold {
		return clampUnit(psdSRGBLinearScale * v)
	}
	return clampUnit(psdSRGBGammaScale*math.Pow(v, 1.0/psdSRGBGammaPower) - psdSRGBGammaBias)
}

func clampUnit32(v float64) float32 {
	return float32(clampUnit(v))
}

func clampUnit(v float64) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	return v
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
