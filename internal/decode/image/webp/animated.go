package webp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"strconv"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pixel"
	"github.com/gophics/ravenporter/ir"
)

const (
	webpRIFFHeaderSize      = 12
	webpChunkHeaderSize     = 8
	webpChunkTypeSize       = 4
	webpChunkSizeOffset     = 4
	webpChunkDataOffset     = 8
	webpVP8XSize            = 10
	webpANIMSize            = 6
	webpANMFHeaderSize      = 16
	webpFrameXOffset        = 0
	webpFrameYOffset        = 3
	webpFrameWidthOffset    = 6
	webpFrameHeightOffset   = 9
	webpFrameDurationOffset = 12
	webpFrameFlagsOffset    = 15
	webpCanvasWidthOffset   = 4
	webpCanvasHeightOffset  = 7
	webpFrameCoordScale     = 2
	webpDelayDenominator    = 1000
	webpShift8              = 8
	webpShift16             = 16
	webpShift24             = 24

	webpAnimationFlag = 1 << 1
	webpAlphaFlag     = 1 << 4

	webpDoNotBlendFlag          = 1 << 1
	webpDisposeToBackgroundFlag = 1

	chunkTypeALPH = "ALPH"
	chunkTypeANIM = "ANIM"
	chunkTypeANMF = "ANMF"
	chunkTypeVP8  = "VP8 "
	chunkTypeVP8L = "VP8L"
	chunkTypeVP8X = "VP8X"
)

type webpAnimFrame struct {
	x                   int
	y                   int
	w                   int
	h                   int
	durationMS          int
	hasAlpha            bool
	doNotBlend          bool
	disposeToBackground bool
	frameData           []byte
}

func decodeAnimatedWebP(raw []byte, maxPixels int) (*ir.Asset, error) {
	canvasW, canvasH, bg, frames, ok := parseAnimatedWebP(raw)
	if !ok {
		return nil, imgutil.DecodeErrStr(webpName, errAnimatedWebP)
	}
	if err := imgutil.CheckPixelLimit(canvasW, canvasH, maxPixels); err != nil {
		return nil, imgutil.DecodeErrStr(webpName, err)
	}

	asset := ir.NewAsset(ir.FormatWebP)
	asset.Images = make([]*ir.ImageAsset, 0, len(frames))

	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	fill := image.NewUniform(bg)
	draw.Draw(canvas, canvas.Bounds(), fill, image.Point{}, draw.Src)

	var prevRect image.Rectangle
	disposePrev := false

	for i := range frames {
		frame := &frames[i]
		if disposePrev {
			draw.Draw(canvas, prevRect, fill, image.Point{}, draw.Src)
		}

		frameRect := image.Rect(frame.x, frame.y, frame.x+frame.w, frame.y+frame.h)
		if !frameRect.In(canvas.Bounds()) {
			return nil, imgutil.DecodeErrStr(webpName, errAnimatedWebP)
		}

		frameBlob := buildAnimatedFrameBlob(frame)
		frameImg, _, err := image.Decode(bytes.NewReader(frameBlob))
		if err != nil {
			return nil, imgutil.DecodeErrStr(webpName, err)
		}

		op := draw.Over
		if frame.doNotBlend {
			op = draw.Src
		}
		draw.Draw(canvas, frameRect, frameImg, image.Point{}, op)

		pixels := make([]byte, len(canvas.Pix))
		copy(pixels, canvas.Pix)
		precomputed := &ir.PixelBuffer{
			Data:     pixels,
			DataType: ir.DataTypeUint8,
			BitDepth: ir.BitDepth8,
		}

		asset.Images = append(asset.Images, &ir.ImageAsset{
			Name:       webpName + "_" + strconv.Itoa(i),
			Format:     ir.ImageWebP,
			Width:      canvasW,
			Height:     canvasH,
			Channels:   ir.ChannelRGBA,
			ColorSpace: ir.ColorSRGB,
			MipLevels:  1,
			Metadata: map[string]string{
				imgutil.MetaKeyDelayNum: strconv.Itoa(frame.durationMS),
				imgutil.MetaKeyDelayDen: strconv.Itoa(webpDelayDenominator),
			},
			PixelDecode: func(_ *ir.ImageAsset) (*ir.PixelBuffer, error) {
				return precomputed, nil
			},
		})

		prevRect = frameRect
		disposePrev = frame.disposeToBackground
	}

	return asset, nil
}

func isAnimatedWebP(raw []byte) bool {
	if !isRIFFWebP(raw) {
		return false
	}

	for pos := webpRIFFHeaderSize; pos+webpChunkHeaderSize <= len(raw); {
		chunkSize := int(binread.ReadU32LE(raw[pos+webpChunkSizeOffset:]))
		chunkEnd := pos + webpChunkDataOffset + chunkSize
		if chunkEnd > len(raw) {
			return false
		}
		if string(raw[pos:pos+webpChunkTypeSize]) == chunkTypeVP8X {
			return chunkSize == webpVP8XSize && raw[pos+webpChunkDataOffset]&webpAnimationFlag != 0
		}
		pos = nextRIFFChunk(chunkEnd, chunkSize)
	}
	return false
}

func parseAnimatedWebP(raw []byte) (
	canvasW int,
	canvasH int,
	bg color.RGBA,
	frames []webpAnimFrame,
	ok bool,
) {
	if !isRIFFWebP(raw) {
		return 0, 0, color.RGBA{}, nil, false
	}
	var animated bool

	for pos := webpRIFFHeaderSize; pos+webpChunkHeaderSize <= len(raw); {
		chunkType := string(raw[pos : pos+webpChunkTypeSize])
		chunkSize := int(binread.ReadU32LE(raw[pos+webpChunkSizeOffset:]))
		dataStart := pos + webpChunkDataOffset
		chunkEnd := dataStart + chunkSize
		if chunkEnd > len(raw) {
			return 0, 0, color.RGBA{}, nil, false
		}

		payload := raw[dataStart:chunkEnd]
		switch chunkType {
		case chunkTypeVP8X:
			if chunkSize != webpVP8XSize {
				return 0, 0, color.RGBA{}, nil, false
			}
			animated = payload[0]&webpAnimationFlag != 0
			canvasW = readU24LE(payload[webpCanvasWidthOffset:]) + 1
			canvasH = readU24LE(payload[webpCanvasHeightOffset:]) + 1
		case chunkTypeANIM:
			if chunkSize < webpANIMSize {
				return 0, 0, color.RGBA{}, nil, false
			}
			bg = color.RGBA{R: payload[2], G: payload[1], B: payload[0], A: payload[3]}
		case chunkTypeANMF:
			frame, ok := parseANMFChunk(payload)
			if !ok {
				return 0, 0, color.RGBA{}, nil, false
			}
			frames = append(frames, frame)
		}

		pos = nextRIFFChunk(chunkEnd, chunkSize)
	}

	if !animated || canvasW <= 0 || canvasH <= 0 || len(frames) == 0 {
		return 0, 0, color.RGBA{}, nil, false
	}
	return canvasW, canvasH, bg, frames, true
}

func parseANMFChunk(payload []byte) (webpAnimFrame, bool) {
	if len(payload) < webpANMFHeaderSize {
		return webpAnimFrame{}, false
	}

	frameData := payload[webpANMFHeaderSize:]
	hasAlpha, hasBitstream := scanFrameData(frameData)
	if !hasBitstream {
		return webpAnimFrame{}, false
	}

	flags := payload[webpFrameFlagsOffset]
	return webpAnimFrame{
		x:                   readU24LE(payload[webpFrameXOffset:]) * webpFrameCoordScale,
		y:                   readU24LE(payload[webpFrameYOffset:]) * webpFrameCoordScale,
		w:                   readU24LE(payload[webpFrameWidthOffset:]) + 1,
		h:                   readU24LE(payload[webpFrameHeightOffset:]) + 1,
		durationMS:          readU24LE(payload[webpFrameDurationOffset:]),
		hasAlpha:            hasAlpha,
		doNotBlend:          flags&webpDoNotBlendFlag != 0,
		disposeToBackground: flags&webpDisposeToBackgroundFlag != 0,
		frameData:           frameData,
	}, true
}

func scanFrameData(data []byte) (hasAlpha, hasBitstream bool) {
	for pos := 0; pos+webpChunkHeaderSize <= len(data); {
		chunkType := string(data[pos : pos+webpChunkTypeSize])
		chunkSize := int(binread.ReadU32LE(data[pos+webpChunkSizeOffset:]))
		chunkEnd := pos + webpChunkDataOffset + chunkSize
		if chunkEnd > len(data) {
			return false, false
		}

		switch chunkType {
		case chunkTypeALPH:
			hasAlpha = true
		case chunkTypeVP8, chunkTypeVP8L:
			hasBitstream = true
		}

		pos = nextRIFFChunk(chunkEnd, chunkSize)
	}

	return hasAlpha, hasBitstream
}

func buildAnimatedFrameBlob(frame *webpAnimFrame) []byte {
	vp8x := buildRIFFChunk(chunkTypeVP8X, buildVP8XPayload(frame.w, frame.h, frame.hasAlpha))
	riffSize := webpChunkTypeSize + len(vp8x) + len(frame.frameData)

	blob := make([]byte, 0, webpRIFFHeaderSize+len(vp8x)+len(frame.frameData))
	blob = append(blob, magicWebP...)
	blob = appendU32LE(blob, checkedU32(riffSize))
	blob = append(blob, markerWebP...)
	blob = append(blob, vp8x...)
	blob = append(blob, frame.frameData...)
	return blob
}

func buildVP8XPayload(width, height int, hasAlpha bool) []byte {
	payload := make([]byte, webpVP8XSize)
	if hasAlpha {
		payload[0] = webpAlphaFlag
	}
	writeU24LE(payload[webpCanvasWidthOffset:], width-1)
	writeU24LE(payload[webpCanvasHeightOffset:], height-1)
	return payload
}

func buildRIFFChunk(chunkType string, payload []byte) []byte {
	chunk := make([]byte, 0, webpChunkHeaderSize+len(payload)+len(payload)%2)
	chunk = append(chunk, chunkType...)
	chunk = appendU32LE(chunk, checkedU32(len(payload)))
	chunk = append(chunk, payload...)
	if len(payload)%2 != 0 {
		chunk = append(chunk, 0)
	}
	return chunk
}

func appendU32LE(dst []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(dst, v)
}

//nolint:gosec // WebP stores this field as a 24-bit little-endian integer.
func writeU24LE(dst []byte, v int) {
	dst[0] = byte(v)
	dst[1] = byte(v >> webpShift8)
	dst[2] = byte(v >> webpShift16)
}

func readU24LE(data []byte) int {
	return int(data[0]) | int(data[1])<<webpShift8 | int(data[2])<<webpShift16
}

func nextRIFFChunk(chunkEnd, chunkSize int) int {
	if chunkSize%2 == 0 {
		return chunkEnd
	}
	return chunkEnd + 1
}

func rgbaPixels(img image.Image, width, height int) []byte {
	return pixel.ToRGBA(img, width, height)
}

func isRIFFWebP(raw []byte) bool {
	if len(raw) < webpRIFFHeaderSize {
		return false
	}
	return bytes.Equal(raw[:4], magicWebP) &&
		bytes.Equal(raw[riffWebPOffset:webpRIFFHeaderSize], markerWebP)
}

func checkedU32(v int) uint32 {
	if v < 0 {
		return 0
	}
	u := uint64(v)
	if u > uint64(^uint32(0)) {
		return 0
	}
	return uint32(u)
}

var errAnimatedWebP = errors.New("webp: invalid animated container")
