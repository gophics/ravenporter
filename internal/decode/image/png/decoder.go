package png

import (
	"bytes"
	"hash/crc32"
	"image"
	"image/draw"
	_ "image/png" // Register stdlib PNG decoder.
	"io"
	"strconv"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pixel"
	"github.com/gophics/ravenporter/ir"
)

const (
	pngFormatName = "PNG"
	pngName       = "png"
	extPNG        = ".png"

	pngIHDROffset = 16
	pngHDimOffset = 4

	chunkTypeIHDR = "IHDR"
	chunkTypeIDAT = "IDAT"
	chunkTypeIEND = "IEND"
	chunkTypeacTL = "acTL"
	chunkTypefcTL = "fcTL"
	chunkTypefdAT = "fdAT"

	pngSigSize       = 8
	pngChunkLenSize  = 4
	pngChunkTypeSize = 4
	pngChunkCRCSize  = 4
	pngChunkOverhead = 12

	pngAcTLMinSize = 8
	pngFcTLMinSize = 26
	pngFdATSeqOver = 4

	metaKeyAPNG     = "APNG"
	metaKeyDelayNum = "DelayNum"
	metaKeyDelayDen = "DelayDen"
	metaValTrue     = "true"

	shift24 = 24
	shift16 = 16
	shift8  = 8
)

var magicPNG = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatPNG, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return imgutil.ProbeBytes(r, magicPNG) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(pngName, err)
	}

	w, h := pngDimensions(raw)
	isAnimated := isAPNG(raw)

	if err := imgutil.CheckPixelLimit(w, h, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(pngName, err)
	}

	if isAnimated {
		return decodeAPNG(raw, w, h)
	}

	decoded := &ir.ImageAsset{
		Name:       pngName,
		Format:     ir.ImagePNG,
		Width:      w,
		Height:     h,
		Channels:   ir.ChannelRGBA,
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
		PixelDecode: func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
			img, _, decErr := image.Decode(bytes.NewReader(d.Compressed))
			if decErr != nil {
				return nil, decErr
			}
			return &ir.PixelBuffer{
				Data:     pixel.ToRGBA(img, d.Width, d.Height),
				DataType: ir.DataTypeUint8,
				BitDepth: ir.BitDepth8,
			}, nil
		},
	}

	return imgutil.BuildAsset(decoded, ir.FormatPNG), nil
}

func decodeAPNG(raw []byte, baseW, baseH int) (*ir.Asset, error) {
	frames := extractAPNGFrames(raw)

	scene := &ir.Asset{}

	canvas := image.NewRGBA(image.Rect(0, 0, baseW, baseH))

	for i, frame := range frames {
		img, _, decErr := image.Decode(bytes.NewReader(frame.blob))
		if decErr != nil {
			return nil, imgutil.DecodeErrStr(pngName, decErr)
		}

		drawRect := image.Rect(frame.x, frame.y, frame.x+frame.w, frame.y+frame.h)
		draw.Draw(canvas, drawRect, img, image.Point{0, 0}, draw.Src)

		pixels := make([]byte, len(canvas.Pix))
		copy(pixels, canvas.Pix)

		precomputed := &ir.PixelBuffer{Data: pixels, DataType: ir.DataTypeUint8, BitDepth: ir.BitDepth8}

		decoded := &ir.ImageAsset{
			Name:       pngName + "_" + strconv.Itoa(i),
			Format:     ir.ImagePNG,
			Width:      baseW,
			Height:     baseH,
			Channels:   ir.ChannelRGBA,
			ColorSpace: ir.ColorSRGB,
			MipLevels:  1,
			Metadata: map[string]string{
				metaKeyDelayNum: strconv.Itoa(frame.delayNum),
				metaKeyDelayDen: strconv.Itoa(frame.delayDen),
			},
			PixelDecode: func(_ *ir.ImageAsset) (*ir.PixelBuffer, error) {
				return precomputed, nil
			},
		}
		scene.Images = append(scene.Images, decoded)
	}

	return scene, nil
}

type apngFrame struct {
	x, y, w, h int
	delayNum   int
	delayDen   int
	blob       []byte
}

func extractAPNGFrames(raw []byte) []apngFrame {
	if len(raw) < pngSigSize {
		return nil
	}

	var headerChunks [][]byte
	var frames []apngFrame
	var currentFrame *apngFrame
	var curIDATs [][]byte

	pos := pngSigSize
	ihdrData := []byte{}

parseLoop:
	for pos+pngChunkOverhead <= len(raw) {
		length := int(binread.ReadU32BE(raw[pos:]))
		chunkType := string(raw[pos+pngChunkLenSize : pos+pngChunkLenSize+pngChunkTypeSize])

		if pos+pngChunkOverhead+length > len(raw) {
			break
		}

		payloadStart := pos + pngChunkLenSize + pngChunkTypeSize
		payload := raw[payloadStart : payloadStart+length]
		chunkRaw := raw[pos : pos+pngChunkOverhead+length]

		switch chunkType {
		case chunkTypeIHDR:
			ihdrData = payload
		case chunkTypeacTL:
		case chunkTypefcTL:
			if currentFrame != nil {
				currentFrame.blob = buildFrameBlob(ihdrData, headerChunks, curIDATs, currentFrame.w, currentFrame.h)
				frames = append(frames, *currentFrame)
			}
			curIDATs = nil

			if length >= pngFcTLMinSize {
				currentFrame = &apngFrame{
					w:        int(binread.ReadU32BE(payload[4:])),
					h:        int(binread.ReadU32BE(payload[8:])),
					x:        int(binread.ReadU32BE(payload[12:])),
					y:        int(binread.ReadU32BE(payload[16:])),
					delayNum: int(binread.ReadU16BE(payload[20:])),
					delayDen: int(binread.ReadU16BE(payload[22:])),
				}
			}
		case chunkTypefdAT:
			if currentFrame != nil && length >= pngFdATSeqOver {
				fdatPayload := payload[pngFdATSeqOver:]
				curIDATs = append(curIDATs, buildChunk(chunkTypeIDAT, fdatPayload))
			}
		case chunkTypeIDAT:
			if currentFrame != nil {
				curIDATs = append(curIDATs, chunkRaw)
			}
		case chunkTypeIEND:
			if currentFrame != nil {
				currentFrame.blob = buildFrameBlob(ihdrData, headerChunks, curIDATs, currentFrame.w, currentFrame.h)
				frames = append(frames, *currentFrame)
				currentFrame = nil
			}
			break parseLoop
		default:
			if currentFrame == nil {
				headerChunks = append(headerChunks, chunkRaw)
			}
		}

		pos += pngChunkOverhead + length
	}

	return frames
}

func buildFrameBlob(ihdrPayload []byte, headers, idats [][]byte, w, h int) []byte {
	out := append([]byte(nil), magicPNG...)

	synthIHDR := append([]byte(nil), ihdrPayload...)

	synthIHDR[0] = byte(w >> shift24)
	synthIHDR[1] = byte(w >> shift16)
	synthIHDR[2] = byte(w >> shift8)
	synthIHDR[3] = byte(w)

	synthIHDR[4] = byte(h >> shift24)
	synthIHDR[5] = byte(h >> shift16)
	synthIHDR[6] = byte(h >> shift8)
	synthIHDR[7] = byte(h)

	out = append(out, buildChunk(chunkTypeIHDR, synthIHDR)...)

	for _, hdr := range headers {
		out = append(out, hdr...)
	}

	for _, idat := range idats {
		out = append(out, idat...)
	}

	out = append(out, buildChunk(chunkTypeIEND, nil)...)
	return out
}

func buildChunk(chunkType string, data []byte) []byte {
	length := len(data)
	buf := make([]byte, pngChunkOverhead+length)

	buf[0] = byte(length >> shift24)
	buf[1] = byte(length >> shift16)
	buf[2] = byte(length >> shift8)
	buf[3] = byte(length)

	copy(buf[pngChunkLenSize:], chunkType)
	if length > 0 {
		copy(buf[pngChunkLenSize+pngChunkTypeSize:], data)
	}

	crc := crc32.ChecksumIEEE(buf[pngChunkLenSize : pngChunkLenSize+pngChunkTypeSize+length])

	buf[pngChunkLenSize+pngChunkTypeSize+length] = byte(crc >> shift24)
	buf[pngChunkLenSize+pngChunkTypeSize+length+1] = byte(crc >> shift16)
	buf[pngChunkLenSize+pngChunkTypeSize+length+2] = byte(crc >> shift8)
	buf[pngChunkLenSize+pngChunkTypeSize+length+3] = byte(crc)

	return buf
}

func pngDimensions(data []byte) (w, h int) {
	if len(data) < pngIHDROffset+pngSigSize {
		return 0, 0
	}
	w = int(binread.ReadU32BE(data[pngIHDROffset:]))
	h = int(binread.ReadU32BE(data[pngIHDROffset+pngHDimOffset:]))
	return w, h
}

func isAPNG(data []byte) bool {
	pos := pngSigSize
	for pos+pngChunkOverhead <= len(data) {
		length := int(binread.ReadU32BE(data[pos:]))
		chunkType := string(data[pos+pngChunkLenSize : pos+pngChunkLenSize+pngChunkTypeSize])

		if chunkType == chunkTypeacTL {
			return true
		}
		if chunkType == chunkTypeIDAT {
			break
		}
		pos += pngChunkOverhead + length
	}
	return false
}

func (d *Decoder) Extensions() []string { return []string{extPNG} }
func (d *Decoder) FormatName() string   { return pngFormatName }
