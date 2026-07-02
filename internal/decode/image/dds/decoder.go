package dds

import (
	"bytes"
	"errors"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	ddsFormatName = "DDS"
	ddsName       = "dds"
	extDDS        = ".dds"
)

var magicDDS = []byte{0x44, 0x44, 0x53, 0x20}

const (
	ddsHeaderSize  = 128
	ddsHeightOff   = 12
	ddsWidthOff    = 16
	ddsDepthOff    = 24
	ddsMipCountOff = 28
	ddsFlagsOff    = 80
	ddsFourCCOff   = 84
	ddsRGBBitOff   = 88
	ddsRBitMaskOff = 92
	ddsGBitMaskOff = 96
	ddsBBitMaskOff = 100
	ddsABitMaskOff = 104
	ddsCaps2Off    = 112
	ddsDXT10Off    = 128
	ddsDXT10Size   = 20

	fourCCDX10 = "DX10"

	dxgiBC1Unorm = 71
	dxgiBC1Srgb  = 72
	dxgiBC2Unorm = 74
	dxgiBC2Srgb  = 75
	dxgiBC3Unorm = 77
	dxgiBC3Srgb  = 78
	dxgiBC4Unorm = 80
	dxgiBC5Unorm = 83
	dxgiBC6HUF16 = 95
	dxgiBC6HSF16 = 96
	dxgiBC7Unorm = 98
	dxgiBC7Srgb  = 99

	ddspfAlphaPixels = 0x1
	ddspfAlpha       = 0x2
	ddspfFourCC      = 0x4
	ddspfRGB         = 0x40
	ddspfLuminance   = 0x20000

	ddsCaps2Cubemap = 0x00000200
	ddsCaps2Volume  = 0x00200000

	d3d10ResourceDimensionTexture1D = 2
	d3d10ResourceDimensionTexture2D = 3
	d3d10ResourceDimensionTexture3D = 4
	d3d10MiscFlagTextureCube        = 0x4

	rgbaChannels = 4
	colorMax     = 255
	bitsPerByte  = 8

	bpp8Bytes  = 1
	bpp16Bytes = 2
	bpp24Bytes = 3
	bpp32Bytes = 4
	shift16    = 16
)

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatDDS, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return imgutil.ProbeBytes(r, magicDDS) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(ddsName, err)
	}

	if len(raw) < ddsHeaderSize || !bytes.Equal(raw[:4], magicDDS) {
		return nil, imgutil.DecodeErrStr(ddsName, errors.New("invalid or truncated header"))
	}

	w, h := ddsDimensions(raw)
	compFormat := ddsCompressionFormat(raw)
	topology, depth, layers := ddsTopology(raw)

	if err := imgutil.CheckPixelLimit(w, h, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(ddsName, err)
	}

	decoded := &ir.ImageAsset{
		Name:              ddsName,
		Format:            ir.ImageDDS,
		Width:             w,
		Height:            h,
		Channels:          ir.ChannelRGBA,
		ColorSpace:        ir.ColorSRGB,
		MipLevels:         ddsMipCount(raw),
		Topology:          topology,
		Depth:             depth,
		Layers:            layers,
		CompressionFormat: compFormat,
		Compressed:        raw,
	}

	if compFormat == ir.GPUCompressionNone {
		decoded.PixelDecode = func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
			pixels, decErr := ddsDecodeUncompressed(d.Compressed, d.Width, d.Height)
			if decErr != nil {
				return nil, decErr
			}
			return &ir.PixelBuffer{Data: pixels, DataType: ir.DataTypeUint8, BitDepth: ir.BitDepth8}, nil
		}
	}

	return imgutil.BuildAsset(decoded, ir.FormatDDS), nil
}

func ddsDimensions(data []byte) (w, h int) {
	if len(data) < ddsHeaderSize {
		return 0, 0
	}
	h = int(binread.ReadU32LE(data[ddsHeightOff:]))
	w = int(binread.ReadU32LE(data[ddsWidthOff:]))
	return w, h
}

func ddsMipCount(data []byte) int {
	if len(data) < ddsHeaderSize {
		return 1
	}
	if mips := int(binread.ReadU32LE(data[ddsMipCountOff:])); mips > 0 {
		return mips
	}
	return 1
}

func ddsTopology(data []byte) (topology ir.ImageTopology, depth, layers int) {
	if len(data) < ddsHeaderSize {
		return ir.ImageTopology2D, 1, 1
	}

	depth = int(binread.ReadU32LE(data[ddsDepthOff:]))
	if depth <= 0 {
		depth = 1
	}

	if hasDX10Header(data) {
		if len(data) < ddsDXT10Off+ddsDXT10Size {
			return ir.ImageTopology2D, depth, 1
		}
		dim := binread.ReadU32LE(data[ddsDXT10Off+4:])
		miscFlag := binread.ReadU32LE(data[ddsDXT10Off+8:])
		layers = int(binread.ReadU32LE(data[ddsDXT10Off+12:]))
		if layers <= 0 {
			layers = 1
		}

		switch {
		case dim == d3d10ResourceDimensionTexture3D:
			return ir.ImageTopology3D, depth, 1
		case miscFlag&d3d10MiscFlagTextureCube != 0 && layers > 1:
			return ir.ImageTopologyCubeArray, 1, layers
		case miscFlag&d3d10MiscFlagTextureCube != 0:
			return ir.ImageTopologyCube, 1, 1
		case dim == d3d10ResourceDimensionTexture1D || dim == d3d10ResourceDimensionTexture2D:
			if layers > 1 {
				return ir.ImageTopology2DArray, 1, layers
			}
		}
		return ir.ImageTopology2D, 1, 1
	}

	caps2 := binread.ReadU32LE(data[ddsCaps2Off:])
	switch {
	case caps2&ddsCaps2Volume != 0 && depth > 1:
		return ir.ImageTopology3D, depth, 1
	case caps2&ddsCaps2Cubemap != 0:
		return ir.ImageTopologyCube, 1, 1
	default:
		return ir.ImageTopology2D, 1, 1
	}
}

const (
	fourCCDXT1 = "DXT1"
	fourCCDXT3 = "DXT3"
	fourCCDXT5 = "DXT5"
	fourCCATI1 = "ATI1"
	fourCCBC4U = "BC4U"
	fourCCATI2 = "ATI2"
	fourCCBC5U = "BC5U"
)

func ddsCompressionFormat(data []byte) ir.GPUCompression {
	if len(data) < ddsHeaderSize {
		return ir.GPUCompressionNone
	}

	flags := binread.ReadU32LE(data[ddsFlagsOff:])
	if flags&ddspfFourCC == 0 {
		return ir.GPUCompressionNone
	}

	fourCC := string(data[ddsFourCCOff : ddsFourCCOff+4])

	switch fourCC {
	case fourCCDXT1:
		return ir.GPUCompressionBC1
	case fourCCDXT3:
		return ir.GPUCompressionBC2
	case fourCCDXT5:
		return ir.GPUCompressionBC3
	case fourCCATI1, fourCCBC4U:
		return ir.GPUCompressionBC4
	case fourCCATI2, fourCCBC5U:
		return ir.GPUCompressionBC5
	case fourCCDX10:
		return ddsDXT10Format(data)
	default:
		return ir.GPUCompressionNone
	}
}

func ddsDXT10Format(data []byte) ir.GPUCompression {
	if len(data) < ddsDXT10Off+ddsDXT10Size {
		return ir.GPUCompressionNone
	}

	switch binread.ReadU32LE(data[ddsDXT10Off:]) {
	case dxgiBC1Unorm, dxgiBC1Srgb:
		return ir.GPUCompressionBC1
	case dxgiBC2Unorm, dxgiBC2Srgb:
		return ir.GPUCompressionBC2
	case dxgiBC3Unorm, dxgiBC3Srgb:
		return ir.GPUCompressionBC3
	case dxgiBC4Unorm:
		return ir.GPUCompressionBC4
	case dxgiBC5Unorm:
		return ir.GPUCompressionBC5
	case dxgiBC6HUF16, dxgiBC6HSF16:
		return ir.GPUCompressionBC6H
	case dxgiBC7Unorm, dxgiBC7Srgb:
		return ir.GPUCompressionBC7
	default:
		return ir.GPUCompressionNone
	}
}

func hasDX10Header(data []byte) bool {
	if len(data) < ddsHeaderSize {
		return false
	}
	flags := binread.ReadU32LE(data[ddsFlagsOff:])
	if flags&ddspfFourCC == 0 {
		return false
	}
	return string(data[ddsFourCCOff:ddsFourCCOff+4]) == fourCCDX10
}

func ddsDecodeUncompressed(data []byte, w, h int) ([]byte, error) {
	if len(data) < ddsHeaderSize {
		return nil, errors.New("invalid dds uncompressed header")
	}

	bpp := int(binread.ReadU32LE(data[ddsRGBBitOff:]))
	rMask := binread.ReadU32LE(data[ddsRBitMaskOff:])
	gMask := binread.ReadU32LE(data[ddsGBitMaskOff:])
	bMask := binread.ReadU32LE(data[ddsBBitMaskOff:])
	aMask := binread.ReadU32LE(data[ddsABitMaskOff:])

	pixelsOff := ddsHeaderSize
	if hasDX10Header(data) {
		pixelsOff += ddsDXT10Size
	}

	bytesPerPixel := bpp / bitsPerByte
	if bytesPerPixel == 0 || bytesPerPixel > rgbaChannels {
		return nil, errors.New("unsupported dds bpp")
	}

	expectedLen := w * h * bytesPerPixel
	if len(data) < pixelsOff+expectedLen {
		return nil, errors.New("truncated dds pixels")
	}

	src := data[pixelsOff : pixelsOff+expectedLen]
	dst := make([]byte, w*h*rgbaChannels)

	rShift, rBits := maskToShiftAndBits(rMask)
	gShift, gBits := maskToShiftAndBits(gMask)
	bShift, bBits := maskToShiftAndBits(bMask)
	aShift, aBits := maskToShiftAndBits(aMask)

	for i := range w * h {
		var val uint32
		switch bytesPerPixel {
		case bpp8Bytes:
			val = uint32(src[i])
		case bpp16Bytes:
			val = uint32(binread.ReadU16LE(src[i*bpp16Bytes:]))
		case bpp24Bytes:
			val = uint32(src[i*bpp24Bytes]) | uint32(src[i*bpp24Bytes+1])<<bitsPerByte | uint32(src[i*bpp24Bytes+2])<<shift16
		case bpp32Bytes:
			val = binread.ReadU32LE(src[i*bpp32Bytes:])
		}

		dstOff := i * rgbaChannels
		dst[dstOff] = applyDDSMask(val, rMask, rShift, rBits)
		dst[dstOff+1] = applyDDSMask(val, gMask, gShift, gBits)
		dst[dstOff+2] = applyDDSMask(val, bMask, bShift, bBits)
		if aMask != 0 {
			dst[dstOff+3] = applyDDSMask(val, aMask, aShift, aBits)
		} else {
			dst[dstOff+3] = colorMax
		}
	}

	return dst, nil
}

func maskToShiftAndBits(mask uint32) (shift, bits uint32) {
	if mask == 0 {
		return 0, 0
	}
	for mask&1 == 0 {
		shift++
		mask >>= 1
	}
	for mask&1 == 1 {
		bits++
		mask >>= 1
	}
	return shift, bits
}

func applyDDSMask(val, mask, shift, bits uint32) byte {
	if bits == 0 {
		return 0
	}
	v := (val & mask) >> shift
	if bits == bitsPerByte {
		return byte(v) //nolint:gosec // mask width is exactly one byte.
	}
	return byte((v * colorMax) / ((1 << bits) - 1)) //nolint:gosec // scaled mask value is clamped to one byte.
}

func (d *Decoder) Extensions() []string { return []string{extDDS} }
func (d *Decoder) FormatName() string   { return ddsFormatName }
