package exr

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"math"
	"unsafe"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pixel"
	"github.com/gophics/ravenporter/internal/piz"
	"github.com/gophics/ravenporter/ir"
)

const (
	exrFormatName    = "EXR"
	exrName          = "exr"
	extEXR           = ".exr"
	exrHeaderMinSize = 12
	exrChannelStride = 16
	exrBox2iSize     = 16
	exrBox2iYMin     = 4
	exrBox2iXMax     = 8
	exrBox2iYMax     = 12
	exrVersionOffset = 4
	exrTiledBit      = 0x200
	exrNonImageBit   = 0x800
	exrMultiPartBit  = 0x1000
	exrDefaultTile   = 32

	exrPixelHalf = 1

	exrAttrDataWindow    = "dataWindow"
	exrAttrDisplayWindow = "displayWindow"
	exrAttrChannels      = "channels"
	exrAttrCompression   = "compression"
	exrAttrTiles         = "tiles"

	exrPixelStride     = 4
	exrScanlineHdrSize = 8
	exrHalfBytes       = 2
	exrFloatBytes      = 4
	exrPixelTypeHalf   = 1
	exrCompRLE         = 1
	exrCompZIPS        = 2
	exrCompZIP         = 3
	exrCompPIZ         = 4
	exrCompB44         = 6
	exrDefaultTileSize = 32

	exrChunkZIP = 16
	exrChunkPIZ = 32
)

var magicEXR = []byte{0x76, 0x2F, 0x31, 0x01}

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatEXR, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return imgutil.ProbeBytes(r, magicEXR) }

// exrMetadata holds internal attribute data to decode EXR blocks.
type exrMetadata struct {
	Compression int
	PixelType   int
	Tiled       bool
	TileW       int
	TileH       int
	MultiPart   bool
	Deep        bool
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(exrName, err)
	}

	w, h := exrDimensions(raw)

	if err := imgutil.CheckPixelLimit(w, h, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(exrName, err)
	}

	tiled := exrIsTiled(raw)
	multiPart := exrIsMultiPart(raw)
	deep := exrIsDeep(raw)
	tileW, tileH := exrTileSize(raw)

	meta := exrMetadata{
		Compression: exrCompression(raw),
		PixelType:   exrPixelType(raw),
		Tiled:       tiled,
		TileW:       tileW,
		TileH:       tileH,
		MultiPart:   multiPart,
		Deep:        deep,
	}

	decoded := &ir.ImageAsset{
		Name:       exrName,
		Format:     ir.ImageEXR,
		Width:      w,
		Height:     h,
		Channels:   exrChannelCount(raw),
		ColorSpace: ir.ColorLinear,
		MipLevels:  1,
		Compressed: raw,
	}
	decoded.PixelDecode = exrPixelDecode(meta)

	return imgutil.BuildAsset(decoded, ir.FormatEXR), nil
}

func exrDimensions(data []byte) (w, h int) {
	off := exrFindAttribute(data, exrAttrDataWindow)
	if off < 0 {
		off = exrFindAttribute(data, exrAttrDisplayWindow)
	}
	if off < 0 || off+exrBox2iSize > len(data) {
		return 0, 0
	}
	xMin := int(int32(binread.ReadU32LE(data[off:])))              //nolint:gosec
	yMin := int(int32(binread.ReadU32LE(data[off+exrBox2iYMin:]))) //nolint:gosec
	xMax := int(int32(binread.ReadU32LE(data[off+exrBox2iXMax:]))) //nolint:gosec
	yMax := int(int32(binread.ReadU32LE(data[off+exrBox2iYMax:]))) //nolint:gosec
	return xMax - xMin + 1, yMax - yMin + 1
}

func exrChannelCount(data []byte) ir.ChannelCount {
	off := exrFindAttribute(data, exrAttrChannels)
	if off < 0 {
		return ir.ChannelRGBA
	}

	count := 0
	pos := off
	for pos < len(data) && data[pos] != 0 {
		pos += binread.CStringLen(data[pos:])
		pos += exrChannelStride
		count++
	}

	switch {
	case count <= 1:
		return ir.ChannelGray
	case count == 3: //nolint:mnd
		return ir.ChannelRGB
	default:
		return ir.ChannelRGBA
	}
}

func exrFindAttribute(data []byte, name string) int {
	if len(data) < exrHeaderMinSize {
		return -1
	}

	pos := 8

	for pos < len(data) {
		attrName := binread.CString(data[pos:])
		pos += binread.CStringLen(data[pos:])
		if attrName == "" {
			return -1
		}

		pos += binread.CStringLen(data[pos:])

		if pos+4 > len(data) {
			return -1
		}
		attrSize := int(binread.ReadU32LE(data[pos:]))
		pos += 4

		if attrName == name {
			return pos
		}
		pos += attrSize
	}
	return -1
}

func exrCompression(data []byte) int {
	off := exrFindAttribute(data, exrAttrCompression)
	if off < 0 || off >= len(data) {
		return 0
	}
	return int(data[off])
}

func exrPixelType(data []byte) int {
	off := exrFindAttribute(data, exrAttrChannels)
	if off < 0 {
		return exrPixelHalf
	}
	pos := off
	if pos < len(data) && data[pos] != 0 {
		pos += binread.CStringLen(data[pos:])
		if pos+4 <= len(data) {
			return int(binread.ReadU32LE(data[pos:]))
		}
	}
	return exrPixelHalf
}

func (d *Decoder) Extensions() []string { return []string{extEXR} }
func (d *Decoder) FormatName() string   { return exrFormatName }

func exrIsTiled(data []byte) bool {
	if len(data) < exrVersionOffset+4 {
		return false
	}
	v := binread.ReadU32LE(data[exrVersionOffset:])
	return v&exrTiledBit != 0
}

func exrIsMultiPart(data []byte) bool {
	if len(data) < exrVersionOffset+4 {
		return false
	}
	v := binread.ReadU32LE(data[exrVersionOffset:])
	return v&exrMultiPartBit != 0
}

func exrIsDeep(data []byte) bool {
	if len(data) < exrVersionOffset+4 {
		return false
	}
	v := binread.ReadU32LE(data[exrVersionOffset:])
	return v&exrNonImageBit != 0
}

func exrTileSize(data []byte) (w, h int) {
	off := exrFindAttribute(data, exrAttrTiles)
	if off < 0 || off+8 > len(data) {
		return exrDefaultTile, exrDefaultTile
	}
	w = int(binread.ReadU32LE(data[off:]))
	h = int(binread.ReadU32LE(data[off+4:]))
	if w <= 0 || h <= 0 {
		return exrDefaultTile, exrDefaultTile
	}
	return w, h
}

func exrPixelDecode(meta exrMetadata) ir.PixelDecodeFunc {
	return func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
		pixelCount := d.Width * d.Height
		if pixelCount == 0 {
			return &ir.PixelBuffer{DataType: ir.DataTypeFloat32, BitDepth: ir.BitDepth32}, nil
		}

		raw := d.Compressed
		hdrEnd := findEXRHeaderEnd(raw)
		if hdrEnd < 0 {
			return exrZeroedFallback(pixelCount), nil
		}

		comprType := exrComprType(meta.Compression)
		if comprType == exrComprUnsupported {
			return exrZeroedFallback(pixelCount), nil
		}

		if meta.Tiled {
			return decodeEXRTiles(d, raw, hdrEnd, comprType, meta)
		}
		return decodeEXRScanlines(d, raw, hdrEnd, comprType, meta)
	}
}

func decodeEXRScanlines(d *ir.ImageAsset, raw []byte, hdrEnd int, comprType exrCompr, meta exrMetadata) (*ir.PixelBuffer, error) { //nolint:lll // args
	nChan := int(d.Channels)
	bytesPerChan := exrBytesPerChannel(meta.PixelType)
	ch := chunkHeight(comprType)
	numChunks := (d.Height + ch - 1) / ch
	scanStart := hdrEnd + numChunks*exrScanlineHdrSize
	result := make([]float32, d.Width*d.Height*exrPixelStride)
	rowSize := d.Width * bytesPerChan * nChan

	pos := scanStart
	for y := 0; y < d.Height; y += ch {
		linesInChunk := min(ch, d.Height-y)
		scanData, advance, ok := exrDecompressChunk(raw, pos, comprType, nChan, d.Width, bytesPerChan, linesInChunk)
		if !ok {
			break
		}
		for line := range linesInChunk {
			lineStart := line * rowSize
			lineEnd := lineStart + rowSize
			if lineEnd > len(scanData) {
				break
			}
			readScanlinePixels(scanData[lineStart:lineEnd], result, y+line, d.Width, nChan, bytesPerChan, meta.PixelType)
		}
		pos += advance
	}

	d.Channels = ir.ChannelRGBA
	return &ir.PixelBuffer{
		DataType: ir.DataTypeFloat32,
		Data:     unsafe.Slice((*byte)(unsafe.Pointer(&result[0])), len(result)*4), //nolint:gosec,mnd // slice cast
		BitDepth: ir.BitDepth32,
	}, nil
}

const exrTileHdrSize = 20 // tileX(4) + tileY(4) + levelX(4) + levelY(4) + dataSize(4)

func decodeEXRTiles(d *ir.ImageAsset, raw []byte, hdrEnd int, comprType exrCompr, meta exrMetadata) (*ir.PixelBuffer, error) {
	tileW, tileH := meta.TileW, meta.TileH
	if tileW <= 0 || tileH <= 0 {
		tileW, tileH = exrDefaultTileSize, exrDefaultTileSize
	}

	tilesX := (d.Width + tileW - 1) / tileW
	tilesY := (d.Height + tileH - 1) / tileH
	tileCount := tilesX * tilesY

	nChan := int(d.Channels)
	bytesPerChan := exrBytesPerChannel(meta.PixelType)
	pixelCount := d.Width * d.Height
	result := make([]float32, pixelCount*exrPixelStride)

	// Skip offset table (8 bytes per tile for single-level).
	pos := hdrEnd + tileCount*exrScanlineHdrSize

	for i := 0; i < tileCount; i++ {
		if pos+exrTileHdrSize > len(raw) {
			break
		}
		tx := int(binary.LittleEndian.Uint32(raw[pos:]))
		ty := int(binary.LittleEndian.Uint32(raw[pos+4:]))
		dataSize := int(binary.LittleEndian.Uint32(raw[pos+16:]))
		pos += exrTileHdrSize

		if pos+dataSize > len(raw) {
			break
		}

		curW := min(tileW, d.Width-tx*tileW)
		curH := min(tileH, d.Height-ty*tileH)

		var tileData []byte
		switch comprType {
		case exrComprRLE:
			dec := exrRLEDecompress(raw[pos:pos+dataSize], curW*curH*nChan*bytesPerChan)
			tileData = dec
		case exrComprZIPS, exrComprZIP16:
			dec, err := zlibDecompress(raw[pos : pos+dataSize])
			if err != nil {
				pos += dataSize
				continue
			}
			tileData = exrReconstructPredictor(dec)
		case exrComprPIZType:
			dec, err := piz.Decompress(raw[pos:pos+dataSize], nChan, curW, curH)
			if err != nil {
				pos += dataSize
				continue
			}
			tileData = dec
		default:
			tileData = raw[pos : pos+dataSize]
		}
		pos += dataSize

		readTilePixels(tileData, result, tx*tileW, ty*tileH, curW, curH, d.Width, nChan, bytesPerChan, meta.PixelType)
	}

	d.Channels = ir.ChannelRGBA
	return &ir.PixelBuffer{
		DataType: ir.DataTypeFloat32,
		Data:     unsafe.Slice((*byte)(unsafe.Pointer(&result[0])), len(result)*4), //nolint:gosec,mnd // fast slice cast
		BitDepth: ir.BitDepth32,
	}, nil
}

type exrCompr int

const (
	exrComprNone        exrCompr = 0
	exrComprRLE         exrCompr = 1
	exrComprZIPS        exrCompr = 2
	exrComprZIP16       exrCompr = 3
	exrComprPIZType     exrCompr = 4
	exrComprB44         exrCompr = 5
	exrComprUnsupported exrCompr = -1
)

func exrComprType(compression int) exrCompr {
	switch compression {
	case 0:
		return exrComprNone
	case exrCompRLE:
		return exrComprRLE
	case exrCompZIPS:
		return exrComprZIPS
	case exrCompZIP:
		return exrComprZIP16
	case exrCompPIZ:
		return exrComprPIZType
	case exrCompB44:
		return exrComprB44
	default:
		return exrComprUnsupported
	}
}

func chunkHeight(ct exrCompr) int {
	switch ct {
	case exrComprZIP16:
		return exrChunkZIP
	case exrComprPIZType:
		return exrChunkPIZ
	case exrComprB44:
		return b44BlockH
	default:
		return 1
	}
}

func exrRLEDecompress(src []byte, expectedSize int) []byte {
	out := make([]byte, 0, expectedSize)
	i := 0
	for i < len(src) && len(out) < expectedSize {
		n := int(int8(src[i]))
		i++
		if n >= 0 {
			count := n + 1
			if i+count > len(src) {
				break
			}
			out = append(out, src[i:i+count]...)
			i += count
		} else if n > -128 {
			count := 1 - n
			if i >= len(src) {
				break
			}
			v := src[i]
			i++
			for range count {
				out = append(out, v)
			}
		}
	}
	return out
}

func exrBytesPerChannel(pixelType int) int {
	if pixelType == exrPixelTypeHalf {
		return exrHalfBytes
	}
	return exrFloatBytes
}

func exrDecompressChunk(raw []byte, pos int, ct exrCompr, nChan, width, bytesPerChan, linesInChunk int) (data []byte, advance int, ok bool) { //nolint:lll // args
	if pos+exrScanlineHdrSize > len(raw) {
		return nil, 0, false
	}
	dataSize := int(binary.LittleEndian.Uint32(raw[pos+4:]))
	pos += exrScanlineHdrSize

	if pos+dataSize > len(raw) {
		return nil, 0, false
	}

	switch ct {
	case exrComprRLE:
		chunkSize := width * bytesPerChan * nChan * linesInChunk
		decompressed := exrRLEDecompress(raw[pos:pos+dataSize], chunkSize)
		return decompressed, exrScanlineHdrSize + dataSize, true
	case exrComprZIPS, exrComprZIP16:
		decompressed, err := zlibDecompress(raw[pos : pos+dataSize])
		if err != nil {
			return nil, exrScanlineHdrSize + dataSize, true
		}
		return exrReconstructPredictor(decompressed), exrScanlineHdrSize + dataSize, true
	case exrComprPIZType:
		decompressed, err := piz.Decompress(raw[pos:pos+dataSize], nChan, width, linesInChunk)
		if err != nil {
			return nil, exrScanlineHdrSize + dataSize, true
		}
		return decompressed, exrScanlineHdrSize + dataSize, true
	case exrComprB44:
		chunkSize := width * b44Half * nChan * linesInChunk
		decompressed := make([]byte, chunkSize)
		b44Decompress(decompressed, raw[pos:pos+dataSize], nChan, width, linesInChunk)
		return decompressed, exrScanlineHdrSize + dataSize, true
	default:
		chunkSize := width * bytesPerChan * nChan * linesInChunk
		if pos+chunkSize > len(raw) {
			return nil, 0, false
		}
		return raw[pos : pos+chunkSize], exrScanlineHdrSize + chunkSize, true
	}
}

func exrZeroedFallback(pixelCount int) *ir.PixelBuffer {
	result := make([]float32, pixelCount*exrPixelStride)
	return &ir.PixelBuffer{
		DataType: ir.DataTypeFloat32,
		Data:     unsafe.Slice((*byte)(unsafe.Pointer(&result[0])), len(result)*4), //nolint:gosec,mnd // fast slice cast
		BitDepth: ir.BitDepth32,
	}
}

func zlibDecompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	out, readErr := io.ReadAll(r)
	closeErr := r.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return out, nil
}

func exrReconstructPredictor(data []byte) []byte {
	if len(data) < 2 { //nolint:mnd // need at least 2 bytes
		return data
	}
	// Delta decode.
	out := make([]byte, len(data))
	out[0] = data[0]
	for i := 1; i < len(data); i++ {
		out[i] = out[i-1] + data[i]
	}
	// Interleave reorder: first half = even bytes, second half = odd bytes.
	half := len(out) / 2 //nolint:mnd // split point
	result := make([]byte, len(out))
	for i := range half {
		result[2*i] = out[i]
		result[2*i+1] = out[half+i]
	}
	if len(out)%2 != 0 {
		result[len(result)-1] = out[len(out)-1]
	}
	return result
}

func readScanlinePixels(data []byte, result []float32, y, w, nChan, bytesPerChan, pixelType int) {
	pos := 0
	for ch := range nChan {
		for x := range w {
			dstIdx := (y*w+x)*exrPixelStride + channelRemap(ch, nChan)
			if pos+bytesPerChan > len(data) {
				return
			}
			if pixelType == exrPixelTypeHalf {
				h := binary.LittleEndian.Uint16(data[pos:])
				result[dstIdx] = pixel.Float16to32(h)
			} else {
				bits := binary.LittleEndian.Uint32(data[pos:])
				result[dstIdx] = math.Float32frombits(bits)
			}
			pos += bytesPerChan
		}
	}
	if nChan < exrPixelStride {
		for x := range w {
			result[(y*w+x)*exrPixelStride+3] = 1.0
		}
	}
}

func readTilePixels(data []byte, result []float32, startX, startY, tileW, tileH, imgW, nChan, bytesPerChan, pixelType int) {
	pos := 0
	for ch := range nChan {
		for ty := range tileH {
			for tx := range tileW {
				gx := startX + tx
				gy := startY + ty
				dstIdx := (gy*imgW+gx)*exrPixelStride + channelRemap(ch, nChan)
				if pos+bytesPerChan > len(data) {
					return
				}
				if pixelType == exrPixelTypeHalf {
					h := binary.LittleEndian.Uint16(data[pos:])
					result[dstIdx] = pixel.Float16to32(h)
				} else {
					bits := binary.LittleEndian.Uint32(data[pos:])
					result[dstIdx] = math.Float32frombits(bits)
				}
				pos += bytesPerChan
			}
		}
	}
	if nChan < exrPixelStride {
		for ty := range tileH {
			for tx := range tileW {
				result[((startY+ty)*imgW+startX+tx)*exrPixelStride+3] = 1.0
			}
		}
	}
}

func findEXRHeaderEnd(data []byte) int {
	pos := 8
	for pos < len(data) {
		start := pos
		for pos < len(data) && data[pos] != 0 {
			pos++
		}
		if pos == start {
			// Found empty attribute name = end of header (null byte)
			return pos + 1
		}
		pos++ // skip attribute name null byte

		for pos < len(data) && data[pos] != 0 {
			pos++
		}
		pos++ // skip attribute type null byte

		if pos+4 > len(data) {
			return -1
		}
		size := int(binary.LittleEndian.Uint32(data[pos:]))
		pos += 4 + size //nolint:mnd // attribute header size
	}
	return -1
}

func channelRemap(ch, nChan int) int {
	if nChan <= 1 {
		return 0
	}
	order := [4]int{3, 2, 1, 0} // EXR: ABGR → RGBA
	if ch < len(order) {
		return order[ch]
	}
	return ch
}
