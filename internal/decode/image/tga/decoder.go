package tga

import (
	"bytes"
	"errors"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
)

const (
	tgaFormatName = "TGA"
	tgaName       = "tga"
	extTGA        = ".tga"
	rgbaChannels  = 4

	tgaTypeColorMappedRGB   = 1
	tgaTypeUncompressedRGB  = 2
	tgaTypeUncompressedGray = 3
	tgaTypeRLEColorMapped   = 9
	tgaTypeRLERGB           = 10
	tgaTypeRLEGray          = 11

	tgaHeaderSize   = 18
	tgaFooterSize   = 18
	tgaBPP8         = 8
	tgaBPP16        = 16
	tgaBPP24        = 24
	tgaBPP32        = 32
	tgaBitsPerByte  = 8
	tgaOriginTop    = 0x20
	tgaOriginRight  = 0x10
	tgaAttrBitsMask = 0x0F
	tgaRLERunBit    = 0x80
	tgaRLECountMax  = 0x7F

	tgaDimWidthOff  = 12
	tgaDimHeightOff = 14
	tgaDimEndOff    = 16

	shift10 = 10
	shift5  = 5
	shift3  = 3
	mask5   = 0x1F
	mask1   = 0x8000

	cmEntrySize2 = 2
	cmEntrySize3 = 3
	cmEntrySize4 = 4
)

var errTGAUnsupported = errors.New("unsupported TGA format")

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatTGA, Decoder: &Decoder{}}}
}

const tgaFooterString = "TRUEVISION-XFILE.\x00"

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	defer func() { _, _ = r.Seek(pos, io.SeekStart) }() //nolint:errcheck // reset position

	size, err := r.Seek(0, io.SeekEnd)
	if err != nil || size < 18 {
		return false
	}
	if _, err = r.Seek(-18, io.SeekEnd); err != nil {
		return false
	}
	buf := pool.GetBuffer(tgaFooterSize)
	defer pool.PutBuffer(buf)
	if _, err := io.ReadFull(r, buf[:18]); err != nil {
		return false
	}
	tgaMagic := []byte(tgaFooterString)
	return bytes.Equal(buf[:17], tgaMagic[:17])
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(tgaName, err)
	}

	if len(raw) < tgaHeaderSize {
		return nil, imgutil.DecodeErrStr(tgaName, errors.New("file too short or malformed header"))
	}

	var w, h int
	if len(raw) >= tgaHeaderSize {
		w = int(binread.ReadU16LE(raw[tgaDimWidthOff:tgaDimHeightOff]))
		h = int(binread.ReadU16LE(raw[tgaDimHeightOff:tgaDimEndOff]))
	}

	if err := imgutil.CheckPixelLimit(w, h, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(tgaName, err)
	}

	decoded := &ir.ImageAsset{
		Name:       tgaName,
		Format:     ir.ImageTGA,
		Width:      w,
		Height:     h,
		Channels:   ir.ChannelRGBA,
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
		PixelDecode: func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
			pixels, _, _, err := decodeTGA(d.Compressed)
			if err != nil {
				return nil, err
			}
			return &ir.PixelBuffer{Data: pixels, DataType: ir.DataTypeUint8, BitDepth: ir.BitDepth8}, nil
		},
	}

	return imgutil.BuildAsset(decoded, ir.FormatTGA), nil
}

func (d *Decoder) Extensions() []string { return []string{extTGA} }
func (d *Decoder) FormatName() string   { return tgaFormatName }

type tgaHeader struct {
	idLength     uint8
	colorMapType uint8
	imageType    uint8
	cmFirstEntry uint16
	cmLength     uint16
	cmEntrySize  uint8
	width        uint16
	height       uint16
	bpp          uint8
	descriptor   uint8
}

func readTGAHeader(data []byte) (tgaHeader, error) {
	if len(data) < tgaHeaderSize {
		return tgaHeader{}, errTGAUnsupported
	}
	return tgaHeader{
		idLength:     data[0],
		colorMapType: data[1],
		imageType:    data[2],
		cmFirstEntry: binread.ReadU16LE(data[3:5]),
		cmLength:     binread.ReadU16LE(data[5:7]),
		cmEntrySize:  data[7],
		width:        binread.ReadU16LE(data[12:14]),
		height:       binread.ReadU16LE(data[14:16]),
		bpp:          data[16],
		descriptor:   data[17],
	}, nil
}

func decodeTGA(data []byte) (rgba []byte, w, h int, err error) {
	hdr, err := readTGAHeader(data)
	if err != nil {
		return nil, 0, 0, err
	}

	bpp, err := validateTGAHeader(hdr)
	if err != nil {
		return nil, 0, 0, errTGAUnsupported
	}

	pos := tgaHeaderSize + int(hdr.idLength)

	var colorMap []byte
	if isColorMappedTGA(hdr.imageType) {
		cmBytes := int(hdr.cmLength) * int(hdr.cmEntrySize) / tgaBitsPerByte
		if pos+cmBytes > len(data) {
			return nil, 0, 0, errTGAUnsupported
		}
		colorMap = data[pos : pos+cmBytes]
		pos += cmBytes
	}

	w, h = int(hdr.width), int(hdr.height)

	raw, newPos, readErr := readTGAPixels(data, pos, hdr.imageType, w*h, bpp)
	if readErr != nil {
		return nil, 0, 0, readErr
	}
	_ = newPos

	switch hdr.imageType {
	case tgaTypeColorMappedRGB, tgaTypeRLEColorMapped:
		rgba = mappedToRGBA(raw, colorMap, int(hdr.cmFirstEntry), int(hdr.cmEntrySize)/tgaBitsPerByte, w*h)
	case tgaTypeUncompressedGray, tgaTypeRLEGray:
		rgba = grayToRGBA(raw, bpp, w*h)
	default:
		rgba = truecolorToRGBA(raw, bpp, int(hdr.descriptor&tgaAttrBitsMask), w*h)
	}

	if hdr.imageType == tgaTypeRLEColorMapped || hdr.imageType == tgaTypeRLERGB || hdr.imageType == tgaTypeRLEGray {
		pool.PutBuffer(raw)
	}

	if hdr.descriptor&tgaOriginTop == 0 {
		flipVertical(rgba, w, h)
	}
	if hdr.descriptor&tgaOriginRight != 0 {
		flipHorizontal(rgba, w, h)
	}
	return rgba, w, h, nil
}

func validateTGAHeader(hdr tgaHeader) (int, error) {
	if hdr.width == 0 || hdr.height == 0 {
		return 0, errTGAUnsupported
	}

	switch hdr.imageType {
	case tgaTypeColorMappedRGB, tgaTypeRLEColorMapped:
		if hdr.colorMapType != 1 || hdr.cmLength == 0 || hdr.bpp != tgaBPP8 {
			return 0, errTGAUnsupported
		}
		switch hdr.cmEntrySize {
		case tgaBPP16, tgaBPP24, tgaBPP32:
			return int(hdr.bpp) / tgaBitsPerByte, nil
		default:
			return 0, errTGAUnsupported
		}
	case tgaTypeUncompressedRGB, tgaTypeRLERGB:
		switch hdr.bpp {
		case tgaBPP16, tgaBPP24, tgaBPP32:
			return int(hdr.bpp) / tgaBitsPerByte, nil
		default:
			return 0, errTGAUnsupported
		}
	case tgaTypeUncompressedGray, tgaTypeRLEGray:
		switch hdr.bpp {
		case tgaBPP8, tgaBPP16:
			return int(hdr.bpp) / tgaBitsPerByte, nil
		default:
			return 0, errTGAUnsupported
		}
	default:
		return 0, errTGAUnsupported
	}
}

func isColorMappedTGA(imageType uint8) bool {
	return imageType == tgaTypeColorMappedRGB || imageType == tgaTypeRLEColorMapped
}

func readTGAPixels(data []byte, pos int, imageType uint8, pixelCount, bpp int) (pixels []byte, end int, err error) {
	if imageType == tgaTypeRLERGB || imageType == tgaTypeRLEColorMapped || imageType == tgaTypeRLEGray {
		return decodeTGARLE(data, pos, pixelCount, bpp)
	}
	need := pixelCount * bpp
	if pos+need > len(data) {
		return nil, pos, errTGAUnsupported
	}
	return data[pos : pos+need], pos + need, nil
}

func decodeTGARLE(data []byte, pos, pixelCount, bpp int) (pixels []byte, end int, err error) {
	sz := pixelCount * bpp
	out := pool.GetBuffer(sz)
	outPos := 0

	for outPos < sz {
		if pos >= len(data) {
			return nil, pos, errTGAUnsupported
		}
		header := data[pos]
		pos++

		count := int(header&tgaRLECountMax) + 1

		if header&tgaRLERunBit == 0 {
			need := count * bpp
			if pos+need > len(data) || outPos+need > len(out) {
				return nil, pos, errTGAUnsupported
			}
			copy(out[outPos:], data[pos:pos+need])
			outPos += need
			pos += need
			continue
		}

		if pos+bpp > len(data) {
			return nil, pos, errTGAUnsupported
		}
		pixel := data[pos : pos+bpp]
		pos += bpp
		for range count {
			if outPos+bpp > len(out) {
				return nil, pos, errTGAUnsupported
			}
			copy(out[outPos:], pixel)
			outPos += bpp
		}
	}
	return out, pos, nil
}

func truecolorToRGBA(src []byte, bpp, alphaBits, pixelCount int) []byte {
	rgba := make([]byte, pixelCount*rgbaChannels)
	for i := range pixelCount {
		srcOff := i * bpp
		dstOff := i * rgbaChannels
		if bpp == cmEntrySize2 {
			rgba[dstOff], rgba[dstOff+1], rgba[dstOff+2], rgba[dstOff+3] = expandTGA555(binread.ReadU16LE(src[srcOff:]), alphaBits)
			continue
		}
		rgba[dstOff] = src[srcOff+2]
		rgba[dstOff+1] = src[srcOff+1]
		rgba[dstOff+2] = src[srcOff]
		if bpp == rgbaChannels {
			rgba[dstOff+3] = src[srcOff+3]
		} else {
			rgba[dstOff+3] = 0xFF
		}
	}
	return rgba
}

func expandTGA555(v uint16, alphaBits int) (r, g, b, a byte) {
	r = byte((v>>shift10)&mask5) << shift3
	g = byte((v>>shift5)&mask5) << shift3
	b = byte(v&mask5) << shift3
	a = 0xFF
	if alphaBits > 0 && v&mask1 == 0 {
		a = 0
	}
	return r, g, b, a
}

func grayToRGBA(src []byte, bpp, pixelCount int) []byte {
	rgba := make([]byte, pixelCount*rgbaChannels)
	for i := range pixelCount {
		srcOff := i * bpp
		dstOff := i * rgbaChannels
		value := src[srcOff]
		rgba[dstOff] = value
		rgba[dstOff+1] = value
		rgba[dstOff+2] = value
		if bpp == cmEntrySize2 {
			rgba[dstOff+3] = src[srcOff+1]
		} else {
			rgba[dstOff+3] = 0xFF
		}
	}
	return rgba
}

func mappedToRGBA(indices, colorMap []byte, firstEntry, cmBytesPerEntry, pixelCount int) []byte {
	rgba := make([]byte, pixelCount*rgbaChannels)
	for i := range pixelCount {
		idx := int(indices[i]) - firstEntry
		dstOff := i * rgbaChannels

		if idx < 0 || idx*cmBytesPerEntry+cmBytesPerEntry > len(colorMap) {
			rgba[dstOff+3] = 0xFF
			continue
		}

		cmOff := idx * cmBytesPerEntry
		if cmBytesPerEntry >= cmEntrySize3 {
			rgba[dstOff] = colorMap[cmOff+2]
			rgba[dstOff+1] = colorMap[cmOff+1]
			rgba[dstOff+2] = colorMap[cmOff]
			if cmBytesPerEntry == cmEntrySize4 {
				rgba[dstOff+3] = colorMap[cmOff+3]
			} else {
				rgba[dstOff+3] = 0xFF
			}
		} else if cmBytesPerEntry == cmEntrySize2 {
			rgba[dstOff], rgba[dstOff+1], rgba[dstOff+2], rgba[dstOff+3] = expandTGA555(binread.ReadU16LE(colorMap[cmOff:]), 1)
		}
	}
	return rgba
}

func flipVertical(pixels []byte, w, h int) {
	rowBytes := w * rgbaChannels
	for y := range h / 2 {
		top := pixels[y*rowBytes : y*rowBytes+rowBytes]
		bot := pixels[(h-1-y)*rowBytes : (h-1-y)*rowBytes+rowBytes]
		for i := range rowBytes {
			top[i], bot[i] = bot[i], top[i]
		}
	}
}

func flipHorizontal(pixels []byte, w, h int) {
	for y := range h {
		row := pixels[y*w*rgbaChannels : (y+1)*w*rgbaChannels]
		for x := 0; x < w/2; x++ {
			left := x * rgbaChannels
			right := (w - 1 - x) * rgbaChannels
			for i := range rgbaChannels {
				row[left+i], row[right+i] = row[right+i], row[left+i]
			}
		}
	}
}
