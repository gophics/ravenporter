package ktx

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
	ktxFormatName = "KTX"
	ktxName       = "ktx"
	extKTX        = ".ktx"
	extKTX2       = ".ktx2"
)

const (
	minKTXSize = 12
)

var magicKTX1 = []byte{0xAB, 0x4B, 0x54, 0x58}
var magicKTX2 = []byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x32, 0x30, 0xBB, 0x0D, 0x0A, 0x1A, 0x0A}

const (
	ktx1HeaderSize    = 64
	ktx1WidthOff      = 36
	ktx1HeightOff     = 40
	ktx1DepthOff      = 44
	ktx1LayersOff     = 48
	ktx1FacesOff      = 52
	ktx1LevelCountOff = 56

	ktx2HeaderSize    = 80
	ktx2VkFormatOff   = 12
	ktx2WidthOff      = 20
	ktx2HeightOff     = 24
	ktx2DepthOff      = 28
	ktx2LayersOff     = 32
	ktx2FacesOff      = 36
	ktx2LevelCountOff = 40
	ktx2SuperOff      = 44
	ktx2IndexByteOff  = 80

	ktx2SuperNone = 0
	ktx2SuperZstd = 2
	ktx2SuperZlib = 3

	ktx2LevelEntrySize = 24
	ktx2LevelFieldSize = 8
	ktxCubeFaceCount   = 6
)

const (
	vkBC1Unorm  = 131
	vkBC1Srgb   = 132
	vkBC2Unorm  = 135
	vkBC2Srgb   = 136
	vkBC3Unorm  = 137
	vkBC3Srgb   = 138
	vkBC4Unorm  = 139
	vkBC5Unorm  = 141
	vkBC6HUF16  = 143
	vkBC6HSF16  = 144
	vkBC7Unorm  = 145
	vkBC7Srgb   = 146
	vkASTC4x4   = 157
	vkASTC4x4S  = 158
	vkETC2RGB8  = 147
	vkETC2RGB8S = 148

	ktx1GlFormatOff         = 24
	ktx1GlInternalFormatOff = 28

	glS3TCDXT1RGB  = 0x83F0
	glS3TCDXT1RGBA = 0x83F1
	glS3TCDXT3RGBA = 0x83F2
	glS3TCDXT5RGBA = 0x83F3

	glS3TCDXT1SRGB  = 0x8C4C
	glS3TCDXT3SRGBA = 0x8C4D
	glS3TCDXT5SRGBA = 0x8C4E

	glBPTCBC7Unorm = 0x8E8C
	glBPTCBC7SRGB  = 0x8E8D
	glBPTCBC6HSF   = 0x8E8E
	glBPTCBC6HUF   = 0x8E8F

	glETC2RGB8   = 0x9274
	glETC2SRGB8  = 0x9275
	glETC2RGBA8  = 0x9278
	glETC2SRGBA8 = 0x9279

	glASTC4x4     = 0x93B0
	glASTC4x4SRGB = 0x93D0
)

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatKTX, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	return imgutil.ProbeBytes(r, magicKTX1)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(ktxName, err)
	}

	if len(raw) < minKTXSize {
		return nil, imgutil.DecodeErrStr(ktxName, errors.New("file too short for ktx"))
	}
	if !bytes.Equal(raw[:4], magicKTX1) && !isKTX2(raw) {
		return nil, imgutil.DecodeErrStr(ktxName, errors.New("invalid ktx magic bytes"))
	}

	isV2 := isKTX2(raw)

	var w, h, mips int
	var comp ir.GPUCompression
	var topology ir.ImageTopology
	var depth, layers int
	if isV2 {
		w, h = ktx2Dimensions(raw)
		mips = ktx2MipCount(raw)
		comp = ktx2CompressionFormat(raw)
		topology, depth, layers = ktx2Topology(raw)
	} else {
		w, h = ktx1Dimensions(raw)
		mips = ktx1MipCount(raw)
		comp = ktx1CompressionFormat(raw)
		topology, depth, layers = ktx1Topology(raw)
	}

	if err := imgutil.CheckPixelLimit(w, h, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(ktxName, err)
	}
	if isV2 {
		raw, err = ktx2RewriteSupercompression(raw)
		if err != nil {
			return nil, imgutil.DecodeErrStr(ktxName, err)
		}
	}

	decoded := &ir.ImageAsset{
		Name:              ktxName,
		Format:            ir.ImageKTX,
		Width:             w,
		Height:            h,
		Channels:          ir.ChannelRGBA,
		ColorSpace:        ir.ColorSRGB,
		MipLevels:         mips,
		Topology:          topology,
		Depth:             depth,
		Layers:            layers,
		CompressionFormat: comp,
		Compressed:        raw,
	}

	return imgutil.BuildAsset(decoded, ir.FormatKTX), nil
}

func isKTX2(data []byte) bool {
	if len(data) < len(magicKTX2) {
		return false
	}
	for i, b := range magicKTX2 {
		if data[i] != b {
			return false
		}
	}
	return true
}

func ktx1Dimensions(data []byte) (w, h int) {
	if len(data) < ktx1HeaderSize {
		return 0, 0
	}
	w = int(binread.ReadU32LE(data[ktx1WidthOff:]))
	h = int(binread.ReadU32LE(data[ktx1HeightOff:]))
	return w, h
}

func ktx1MipCount(data []byte) int {
	if len(data) < ktx1HeaderSize {
		return 1
	}
	if mips := int(binread.ReadU32LE(data[ktx1LevelCountOff:])); mips > 0 {
		return mips
	}
	return 1
}

func ktx1Topology(data []byte) (topology ir.ImageTopology, depth, layers int) {
	if len(data) < ktx1HeaderSize {
		return ir.ImageTopology2D, 1, 1
	}

	depth = int(binread.ReadU32LE(data[ktx1DepthOff:]))
	layers = int(binread.ReadU32LE(data[ktx1LayersOff:]))
	faces := int(binread.ReadU32LE(data[ktx1FacesOff:]))

	switch {
	case depth > 0:
		return ir.ImageTopology3D, depth, 1
	case faces == ktxCubeFaceCount && layers > 0:
		return ir.ImageTopologyCubeArray, 1, layers
	case faces == ktxCubeFaceCount:
		return ir.ImageTopologyCube, 1, 1
	case layers > 0:
		return ir.ImageTopology2DArray, 1, layers
	default:
		return ir.ImageTopology2D, 1, 1
	}
}

func ktx1CompressionFormat(data []byte) ir.GPUCompression {
	if len(data) < ktx1HeaderSize {
		return ir.GPUCompressionNone
	}

	glFmt := binread.ReadU32LE(data[ktx1GlFormatOff:])
	if glFmt != 0 {
		return ir.GPUCompressionNone
	}

	glIntFmt := binread.ReadU32LE(data[ktx1GlInternalFormatOff:])
	switch glIntFmt {
	case glS3TCDXT1RGB, glS3TCDXT1RGBA, glS3TCDXT1SRGB:
		return ir.GPUCompressionBC1
	case glS3TCDXT3RGBA, glS3TCDXT3SRGBA:
		return ir.GPUCompressionBC2
	case glS3TCDXT5RGBA, glS3TCDXT5SRGBA:
		return ir.GPUCompressionBC3
	case glBPTCBC6HSF, glBPTCBC6HUF:
		return ir.GPUCompressionBC6H
	case glBPTCBC7Unorm, glBPTCBC7SRGB:
		return ir.GPUCompressionBC7
	case glETC2RGB8, glETC2SRGB8, glETC2RGBA8, glETC2SRGBA8:
		return ir.GPUCompressionETC2
	case glASTC4x4, glASTC4x4SRGB:
		return ir.GPUCompressionASTC4x4
	default:
		return ir.GPUCompressionASTC4x4
	}
}

func ktx2Dimensions(data []byte) (w, h int) {
	if len(data) < ktx2HeaderSize {
		return 0, 0
	}
	w = int(binread.ReadU32LE(data[ktx2WidthOff:]))
	h = int(binread.ReadU32LE(data[ktx2HeightOff:]))
	return w, h
}

func ktx2MipCount(data []byte) int {
	if len(data) < ktx2HeaderSize {
		return 1
	}
	if mips := int(binread.ReadU32LE(data[ktx2LevelCountOff:])); mips > 0 {
		return mips
	}
	return 1
}

func ktx2Topology(data []byte) (topology ir.ImageTopology, depth, layers int) {
	if len(data) < ktx2HeaderSize {
		return ir.ImageTopology2D, 1, 1
	}

	depth = int(binread.ReadU32LE(data[ktx2DepthOff:]))
	layers = int(binread.ReadU32LE(data[ktx2LayersOff:]))
	faces := int(binread.ReadU32LE(data[ktx2FacesOff:]))

	switch {
	case depth > 0:
		return ir.ImageTopology3D, depth, 1
	case faces == ktxCubeFaceCount && layers > 0:
		return ir.ImageTopologyCubeArray, 1, layers
	case faces == ktxCubeFaceCount:
		return ir.ImageTopologyCube, 1, 1
	case layers > 0:
		return ir.ImageTopology2DArray, 1, layers
	default:
		return ir.ImageTopology2D, 1, 1
	}
}

func ktx2CompressionFormat(data []byte) ir.GPUCompression {
	if len(data) < ktx2HeaderSize {
		return ir.GPUCompressionNone
	}
	vkFmt := binread.ReadU32LE(data[ktx2VkFormatOff:])

	switch vkFmt {
	case vkBC1Unorm, vkBC1Srgb:
		return ir.GPUCompressionBC1
	case vkBC2Unorm, vkBC2Srgb:
		return ir.GPUCompressionBC2
	case vkBC3Unorm, vkBC3Srgb:
		return ir.GPUCompressionBC3
	case vkBC4Unorm:
		return ir.GPUCompressionBC4
	case vkBC5Unorm:
		return ir.GPUCompressionBC5
	case vkBC6HUF16, vkBC6HSF16:
		return ir.GPUCompressionBC6H
	case vkBC7Unorm, vkBC7Srgb:
		return ir.GPUCompressionBC7
	case vkASTC4x4, vkASTC4x4S:
		return ir.GPUCompressionASTC4x4
	case vkETC2RGB8, vkETC2RGB8S:
		return ir.GPUCompressionETC2
	default:
		return ir.GPUCompressionNone
	}
}

func ktx2RewriteSupercompression(data []byte) ([]byte, error) {
	if len(data) < ktx2HeaderSize {
		return data, nil
	}
	superScheme := binread.ReadU32LE(data[ktx2SuperOff:])
	if superScheme != ktx2SuperZstd && superScheme != ktx2SuperZlib {
		return data, nil
	}

	levelCount := ktx2MipCount(data)
	indexEnd := ktx2IndexByteOff + levelCount*ktx2LevelEntrySize
	if indexEnd > len(data) {
		return nil, errors.New("invalid ktx2 level index")
	}

	rewritten := make([]byte, len(data), ktx2RewriteCapacity(data, levelCount))
	copy(rewritten, data)
	binread.PutU32LE(rewritten[ktx2SuperOff:], ktx2SuperNone)

	for i := range levelCount {
		entryOff := ktx2IndexByteOff + i*ktx2LevelEntrySize
		byteOff := int(binread.ReadU64LE(data[entryOff:]))                              //nolint:gosec
		byteLen := int(binread.ReadU64LE(data[entryOff+ktx2LevelFieldSize:]))           //nolint:gosec
		uncompressedLen := int(binread.ReadU64LE(data[entryOff+ktx2LevelFieldSize*2:])) //nolint:gosec

		endOff := byteOff + byteLen
		if byteOff < 0 || byteLen < 0 || uncompressedLen < 0 || endOff > len(data) || endOff < byteOff {
			return nil, errors.New("invalid ktx2 level range")
		}
		if uncompressedLen == 0 {
			return nil, errors.New("invalid ktx2 level size")
		}

		rewritten = binread.AppendAligned(rewritten, ktx2LevelFieldSize)
		levelOffset := len(rewritten)
		rewritten = append(rewritten, make([]byte, uncompressedLen)...)
		level := rewritten[levelOffset : levelOffset+uncompressedLen]
		if err := ktx2InflateLevelInto(level, superScheme, data[byteOff:endOff], uncompressedLen); err != nil {
			return nil, err
		}
		binread.PutU64LE(rewritten[entryOff:], uint64(levelOffset))
		binread.PutU64LE(rewritten[entryOff+ktx2LevelFieldSize:], uint64(uncompressedLen))
		binread.PutU64LE(rewritten[entryOff+ktx2LevelFieldSize*2:], uint64(uncompressedLen))
	}
	return rewritten, nil
}

func ktx2RewriteCapacity(data []byte, levelCount int) int {
	capacity := len(data)
	cursor := len(data)
	for i := range levelCount {
		entryOff := ktx2IndexByteOff + i*ktx2LevelEntrySize
		if entryOff+ktx2LevelEntrySize > len(data) {
			return capacity
		}
		uncompressedLen := int(binread.ReadU64LE(data[entryOff+ktx2LevelFieldSize*2:])) //nolint:gosec
		if uncompressedLen <= 0 {
			return capacity
		}
		cursor = alignKTX2Offset(cursor)
		cursor += uncompressedLen
	}
	if cursor > capacity {
		capacity = cursor
	}
	return capacity
}

func ktx2InflateLevelInto(dst []byte, scheme uint32, payload []byte, uncompressedLen int) error {
	switch scheme {
	case ktx2SuperZstd:
		level, err := pool.ZstdDecodeAll(payload)
		if err != nil {
			return err
		}
		if len(level) != uncompressedLen {
			return errors.New("invalid ktx2 zstd level size")
		}
		copy(dst, level)
		return nil
	case ktx2SuperZlib:
		if err := ktx2ZlibReader.DecompressInto(dst, payload); err != nil {
			return err
		}
		return nil
	default:
		return errors.New("unsupported ktx2 supercompression scheme")
	}
}

var ktx2ZlibReader pool.ZlibReader

func alignKTX2Offset(off int) int {
	if rem := off % ktx2LevelFieldSize; rem != 0 {
		return off + ktx2LevelFieldSize - rem
	}
	return off
}

func (d *Decoder) Extensions() []string { return []string{extKTX, extKTX2} }
func (d *Decoder) FormatName() string   { return ktxFormatName }
