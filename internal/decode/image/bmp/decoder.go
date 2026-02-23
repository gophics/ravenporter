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

	bmpDIBHeaderMinSize = 40

	bmpDIBWidth       = 4
	bmpDIBHeight      = 8
	bmpDIBBPP         = 14
	bmpDIBCompression = 16

	bmpBPP4  = 4
	bmpBPP8  = 8
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
}

func parseBMPHeaders(data []byte) (bmpInfo, error) {
	if len(data) < bmpFileHeaderSize+bmpDIBHeaderMinSize {
		return bmpInfo{}, errBMPTruncated
	}

	pixelOff := int(binread.ReadU32LE(data[bmpPixelOffOff:]))

	dib := data[bmpFileHeaderSize:]
	dibSize := int(binread.ReadU32LE(dib))
	width := int(int32(binread.ReadU32LE(dib[bmpDIBWidth:])))   //nolint:gosec // intentional sign
	height := int(int32(binread.ReadU32LE(dib[bmpDIBHeight:]))) //nolint:gosec // intentional sign
	bpp := int(binread.ReadU16LE(dib[bmpDIBBPP:]))
	compression := binread.ReadU32LE(dib[bmpDIBCompression:])

	topDown := height < 0
	if topDown {
		height = -height
	}

	if width <= 0 || height <= 0 {
		return bmpInfo{}, errBMPUnsupported
	}

	colors := 0
	if dibSize > bmpDIBColorsOff {
		colors = int(binread.ReadU32LE(dib[bmpDIBColorsOff:]))
	}
	if colors == 0 && bpp <= bmpMaxPalBPP {
		colors = 1 << bpp
	}

	if bpp != bmpBPP4 && bpp != bmpBPP8 && bpp != bmpBPP24 && bpp != bmpBPP32 {
		return bmpInfo{}, errBMPUnsupported
	}

	isValidComp := compression == bmpCompressionRGB || compression == bmpCompressionBF ||
		compression == bmpCompressionRLE4 || compression == bmpCompressionRLE8
	if !isValidComp {
		return bmpInfo{}, errBMPUnsupported
	}

	return bmpInfo{
		width:       width,
		height:      height,
		topDown:     topDown,
		bpp:         bpp,
		compression: compression,
		pixelOff:    pixelOff,
		colors:      colors,
		dibSize:     dibSize,
	}, nil
}

func readBMPPixels(data []byte, info bmpInfo) ([]byte, error) {
	if info.compression == bmpCompressionRLE4 || info.compression == bmpCompressionRLE8 {
		return readBMPRLE(data, info)
	}
	if info.bpp <= bmpMaxPalBPP {
		return readBMPPalette(data, info)
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
	if palOff+info.colors*4 > len(data) {
		return nil, errBMPTruncated
	}
	palette := data[palOff : palOff+info.colors*4]

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
			pBase := int(idx) * bmpPalEntrySize

			rgba[dstOff] = palette[pBase+2]
			rgba[dstOff+1] = palette[pBase+1]
			rgba[dstOff+2] = palette[pBase]
			rgba[dstOff+3] = 0xFF
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
