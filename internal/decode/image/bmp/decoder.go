package bmp

import (
	"errors"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/ir"
	_ "golang.org/x/image/bmp" // Register image/bmp.
)

const (
	bmpFormatName = "BMP"
	bmpName       = "bmp"
	extBMP        = ".bmp"

	bmpFileHeaderSize = 14
	bmpPixelOffOff    = 10

	bmpDIBHeaderCoreSize = 12
	bmpDIBHeaderMinSize  = 40

	bmpDIBWidth       = 4
	bmpDIBHeight      = 8
	bmpDIBBPP         = 14
	bmpDIBCompression = 16

	bmpCoreWidth  = 4
	bmpCoreHeight = 6
	bmpCoreBPP    = 10

	bmpBPP1  = 1
	bmpBPP4  = 4
	bmpBPP8  = 8
	bmpBPP16 = 16
	bmpBPP24 = 24
	bmpBPP32 = 32

	bmpCompressionRGB  = 0
	bmpCompressionRLE8 = 1
	bmpCompressionRLE4 = 2
	bmpCompressionBF   = 3

	bmpBitsPerByte = 8
	bmpBPPByte32   = 4

	rgbaChannels = 4

	bmpRLEEOF   = 0
	bmpRLEEOL   = 1
	bmpRLEDelta = 2

	bmpRowAlign     = 3
	bmpPalEntrySize = 4
	bmpNibbleShift  = 4
	bmpNibbleMask   = 0x0F
	bmpPalAlignBits = 31
	bmpPalAlignSize = 32
	bmpBPPHalf      = 2

	bmpRLEMinLen    = 2
	bmpRLEAlignMask = 1

	bmpDIBColorsOff = 32
	bmpMaxPalBPP    = 8
	bmpMaskSize     = 4
	bmpCorePalSize  = 3
	bmpBPP16Bytes   = bmpBPP16 / bmpBitsPerByte
	bmpOneBitMask   = 0x01
	bmpColorMax     = 255
	bmpHighBitShift = 7
)

var (
	magicBMP          = []byte{'B', 'M'}
	errBMPUnsupported = errors.New("unsupported BMP format")
	errBMPTruncated   = errors.New("truncated BMP data")
)

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatBMP, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return imgutil.ProbeBytes(r, magicBMP) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(bmpName, err)
	}

	info, headerErr := parseBMPHeaders(raw)
	if headerErr != nil {
		return nil, imgutil.DecodeErrStr(bmpName, headerErr)
	}

	if err := imgutil.CheckPixelLimit(info.width, info.height, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(bmpName, err)
	}

	decoded := &ir.ImageAsset{
		Name:       bmpName,
		Format:     ir.ImageBMP,
		Width:      info.width,
		Height:     info.height,
		Channels:   ir.ChannelRGBA,
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
		PixelDecode: func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
			hdr, err := parseBMPHeaders(d.Compressed)
			if err != nil {
				return nil, err
			}
			pixels, err := readBMPPixels(d.Compressed, hdr)
			if err != nil {
				return nil, err
			}
			return &ir.PixelBuffer{Data: pixels, DataType: ir.DataTypeUint8, BitDepth: ir.BitDepth8}, nil
		},
	}

	return imgutil.BuildAsset(decoded, ir.FormatBMP), nil
}

func (d *Decoder) Extensions() []string { return []string{extBMP} }
func (d *Decoder) FormatName() string   { return bmpFormatName }

type bmpInfo struct {
	width       int
	height      int
	topDown     bool
	bpp         int
	compression uint32
	pixelOff    int
	colors      int
	dibSize     int
	paletteSize int
	rMask       uint32
	gMask       uint32
	bMask       uint32
	aMask       uint32
}

type bmpLayout struct {
	width       int
	height      int
	bpp         int
	compression uint32
	topDown     bool
	dibSize     int
	paletteSize int
}

func parseBMPHeaders(data []byte) (bmpInfo, error) {
	if len(data) < bmpFileHeaderSize+bmpDIBHeaderCoreSize {
		return bmpInfo{}, errBMPTruncated
	}

	pixelOff := int(binread.ReadU32LE(data[bmpPixelOffOff:]))
	dib := data[bmpFileHeaderSize:]
	layout, err := parseBMPLayout(dib)
	if err != nil {
		return bmpInfo{}, err
	}

	info := bmpInfo{
		width:       layout.width,
		height:      layout.height,
		topDown:     layout.topDown,
		bpp:         layout.bpp,
		compression: layout.compression,
		pixelOff:    pixelOff,
		colors:      bmpPaletteColors(dib, layout.bpp, layout.dibSize),
		dibSize:     layout.dibSize,
		paletteSize: layout.paletteSize,
	}
	applyBMPMasks(data, &info)
	return info, nil
}

func parseBMPLayout(dib []byte) (bmpLayout, error) {
	layout := bmpLayout{
		dibSize:     int(binread.ReadU32LE(dib)),
		compression: bmpCompressionRGB,
		paletteSize: bmpPalEntrySize,
	}

	switch {
	case layout.dibSize == bmpDIBHeaderCoreSize:
		if len(dib) < bmpDIBHeaderCoreSize {
			return bmpLayout{}, errBMPTruncated
		}
		layout.width = int(binread.ReadU16LE(dib[bmpCoreWidth:]))
		layout.height = int(binread.ReadU16LE(dib[bmpCoreHeight:]))
		layout.bpp = int(binread.ReadU16LE(dib[bmpCoreBPP:]))
		layout.paletteSize = bmpCorePalSize
	case layout.dibSize >= bmpDIBHeaderMinSize:
		if len(dib) < bmpDIBHeaderMinSize {
			return bmpLayout{}, errBMPTruncated
		}
		layout.width = int(int32(binread.ReadU32LE(dib[bmpDIBWidth:])))   //nolint:gosec
		layout.height = int(int32(binread.ReadU32LE(dib[bmpDIBHeight:]))) //nolint:gosec
		layout.bpp = int(binread.ReadU16LE(dib[bmpDIBBPP:]))
		layout.compression = binread.ReadU32LE(dib[bmpDIBCompression:])
		layout.topDown = layout.height < 0
		if layout.topDown {
			layout.height = -layout.height
		}
	default:
		return bmpLayout{}, errBMPUnsupported
	}

	if layout.width <= 0 || layout.height <= 0 {
		return bmpLayout{}, errBMPUnsupported
	}
	if !isSupportedBMPBPP(layout.bpp) || !isSupportedBMPCompression(layout.compression) {
		return bmpLayout{}, errBMPUnsupported
	}
	return layout, nil
}

func isSupportedBMPBPP(bpp int) bool {
	switch bpp {
	case bmpBPP1, bmpBPP4, bmpBPP8, bmpBPP16, bmpBPP24, bmpBPP32:
		return true
	default:
		return false
	}
}

func isSupportedBMPCompression(compression uint32) bool {
	switch compression {
	case bmpCompressionRGB, bmpCompressionBF, bmpCompressionRLE4, bmpCompressionRLE8:
		return true
	default:
		return false
	}
}

func bmpPaletteColors(dib []byte, bpp, dibSize int) int {
	if dibSize > bmpDIBColorsOff {
		if colors := int(binread.ReadU32LE(dib[bmpDIBColorsOff:])); colors > 0 {
			return colors
		}
	}
	if bpp <= bmpMaxPalBPP {
		return 1 << bpp
	}
	return 0
}

func applyBMPMasks(data []byte, info *bmpInfo) {
	if info.compression == bmpCompressionBF && info.dibSize >= bmpDIBHeaderMinSize {
		maskOff := bmpFileHeaderSize + info.dibSize
		if maskOff+bmpMaskSize*3 > len(data) {
			return
		}
		info.rMask = binread.ReadU32LE(data[maskOff:])
		info.gMask = binread.ReadU32LE(data[maskOff+bmpMaskSize:])
		info.bMask = binread.ReadU32LE(data[maskOff+bmpMaskSize*2:])
		if maskOff+bmpMaskSize*4 <= len(data) {
			info.aMask = binread.ReadU32LE(data[maskOff+bmpMaskSize*3:])
		}
		return
	}
	if info.bpp == bmpBPP16 {
		info.rMask = 0x7C00
		info.gMask = 0x03E0
		info.bMask = 0x001F
	}
}

func readBMPPixels(data []byte, info bmpInfo) ([]byte, error) {
	if info.compression == bmpCompressionRLE4 || info.compression == bmpCompressionRLE8 {
		return readBMPRLE(data, info)
	}
	if info.bpp <= bmpMaxPalBPP {
		return readBMPPalette(data, info)
	}
	if info.bpp == bmpBPP16 {
		return readBMP16(data, info)
	}

	bytesPerPixel := info.bpp / bmpBitsPerByte

	rowSize := (info.width*bytesPerPixel + bmpRowAlign) & ^bmpRowAlign
	totalSize := rowSize * info.height

	if info.pixelOff+totalSize > len(data) {
		return nil, errBMPTruncated
	}

	rgba := make([]byte, info.width*info.height*rgbaChannels)

	for y := range info.height {
		srcY := y
		if !info.topDown {
			srcY = info.height - 1 - y
		}

		srcRow := data[info.pixelOff+srcY*rowSize:]
		for x := range info.width {
			srcOff := x * bytesPerPixel
			dstOff := (y*info.width + x) * rgbaChannels

			rgba[dstOff] = srcRow[srcOff+2]
			rgba[dstOff+1] = srcRow[srcOff+1]
			rgba[dstOff+2] = srcRow[srcOff]
			if bytesPerPixel == bmpBPPByte32 {
				rgba[dstOff+3] = srcRow[srcOff+3]
			} else {
				rgba[dstOff+3] = 0xFF
			}
		}
	}

	return rgba, nil
}

func readBMPPalette(data []byte, info bmpInfo) ([]byte, error) {
	palOff := bmpFileHeaderSize + info.dibSize
	if palOff+info.colors*info.paletteSize > len(data) {
		return nil, errBMPTruncated
	}
	palette := data[palOff : palOff+info.colors*info.paletteSize]

	pixelsPerRow := info.width
	rowSize := ((info.width*info.bpp + bmpPalAlignBits) / bmpPalAlignSize) * bmpPalEntrySize

	if info.pixelOff+rowSize*info.height > len(data) {
		return nil, errBMPTruncated
	}

	rgba := make([]byte, info.width*info.height*rgbaChannels)

	for y := range info.height {
		srcY := y
		if !info.topDown {
			srcY = info.height - 1 - y
		}

		srcRow := data[info.pixelOff+srcY*rowSize:]
		for x := range pixelsPerRow {
			var idx byte
			switch info.bpp {
			case bmpBPP1:
				byteOff := x / bmpBitsPerByte
				shift := bmpHighBitShift - (x % bmpBitsPerByte)
				idx = (srcRow[byteOff] >> shift) & bmpOneBitMask
			case bmpBPP8:
				idx = srcRow[x]
			case bmpBPP4:
				byteOff := x / bmpBPPHalf
				shift := bmpNibbleShift * ((x + 1) % bmpBPPHalf)
				idx = (srcRow[byteOff] >> shift) & bmpNibbleMask
			default:
				return nil, errBMPUnsupported
			}

			if int(idx) >= info.colors {
				idx = byte(info.colors - 1)
			}

			dstOff := (y*info.width + x) * rgbaChannels
			pBase := int(idx) * info.paletteSize

			rgba[dstOff] = palette[pBase+2]
			rgba[dstOff+1] = palette[pBase+1]
			rgba[dstOff+2] = palette[pBase]
			rgba[dstOff+3] = 0xFF
		}
	}

	return rgba, nil
}

func readBMP16(data []byte, info bmpInfo) ([]byte, error) {
	rowSize := (info.width*bmpBPP16/bmpBitsPerByte + bmpRowAlign) & ^bmpRowAlign
	totalSize := rowSize * info.height
	if info.pixelOff+totalSize > len(data) {
		return nil, errBMPTruncated
	}

	rgba := make([]byte, info.width*info.height*rgbaChannels)
	for y := range info.height {
		srcY := y
		if !info.topDown {
			srcY = info.height - 1 - y
		}
		srcRow := data[info.pixelOff+srcY*rowSize:]
		for x := range info.width {
			srcOff := x * bmpBPP16Bytes
			val := uint32(binread.ReadU16LE(srcRow[srcOff:]))
			dstOff := (y*info.width + x) * rgbaChannels
			rgba[dstOff] = applyBMPMask(val, info.rMask)
			rgba[dstOff+1] = applyBMPMask(val, info.gMask)
			rgba[dstOff+2] = applyBMPMask(val, info.bMask)
			if info.aMask != 0 {
				rgba[dstOff+3] = applyBMPMask(val, info.aMask)
			} else {
				rgba[dstOff+3] = 0xFF
			}
		}
	}

	return rgba, nil
}

func readBMPRLE(data []byte, info bmpInfo) ([]byte, error) {
	palOff := bmpFileHeaderSize + info.dibSize
	if palOff+info.colors*4 > len(data) {
		return nil, errBMPTruncated
	}
	palette := data[palOff : palOff+info.colors*4]

	src := data[info.pixelOff:]
	rgba := make([]byte, info.width*info.height*rgbaChannels)

	x, y := 0, 0
	if !info.topDown {
		y = info.height - 1
	}

	for len(src) >= bmpRLEMinLen {
		b1 := src[0]
		b2 := src[1]
		src = src[2:]

		if b1 == 0 {
			switch b2 {
			case bmpRLEEOF:
				return rgba, nil
			case bmpRLEEOL:
				x = 0
				if info.topDown {
					y++
				} else {
					y--
				}
			case bmpRLEDelta:
				if len(src) < bmpRLEMinLen {
					return nil, errBMPTruncated
				}
				x += int(src[0])
				if info.topDown {
					y += int(src[1])
				} else {
					y -= int(src[1])
				}
				src = src[2:]
			default:
				// Absolute mode
				runLen := int(b2)
				var readBytes int
				if info.compression == bmpCompressionRLE8 {
					readBytes = runLen
				} else {
					readBytes = (runLen + 1) / bmpBPPHalf
				}
				readBytesAlign := (readBytes + bmpRLEAlignMask) & ^bmpRLEAlignMask

				if len(src) < readBytesAlign {
					return nil, errBMPTruncated
				}

				for i := range runLen {
					if x >= info.width || y < 0 || y >= info.height {
						x++
						continue
					}
					var idx byte
					if info.compression == bmpCompressionRLE8 {
						idx = src[i]
					} else {
						byteOff := i / bmpBPPHalf
						shift := bmpNibbleShift * ((i + 1) % bmpBPPHalf)
						idx = (src[byteOff] >> shift) & bmpNibbleMask
					}

					setPalettePixel(rgba, x, y, info.width, idx, palette)
					x++
				}
				src = src[readBytesAlign:]
			}
		} else {
			// Encoded mode
			runLen := int(b1)
			for i := range runLen {
				if x >= info.width || y < 0 || y >= info.height {
					x++
					continue
				}
				var idx byte
				if info.compression == bmpCompressionRLE8 {
					idx = b2
				} else {
					shift := bmpNibbleShift * ((i + 1) % bmpBPPHalf)
					idx = (b2 >> shift) & bmpNibbleMask
				}

				setPalettePixel(rgba, x, y, info.width, idx, palette)
				x++
			}
		}
	}

	return rgba, nil
}

func setPalettePixel(rgba []byte, x, y, width int, idx byte, palette []byte) {
	dstOff := (y*width + x) * rgbaChannels
	pBase := int(idx) * bmpPalEntrySize
	if pBase+2 >= len(palette) {
		return
	}
	rgba[dstOff] = palette[pBase+2]
	rgba[dstOff+1] = palette[pBase+1]
	rgba[dstOff+2] = palette[pBase]
	rgba[dstOff+3] = 0xFF
}

func applyBMPMask(val, mask uint32) byte {
	if mask == 0 {
		return 0
	}
	shift := uint32(0)
	for mask>>shift&1 == 0 {
		shift++
	}
	bits := uint32(0)
	for mask>>(shift+bits)&1 == 1 {
		bits++
	}
	masked := (val & mask) >> shift
	if bits == bmpBPP8 {
		return byte(masked)
	}
	return byte((masked * bmpColorMax) / ((1 << bits) - 1))
}
