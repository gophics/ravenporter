//revive:disable-next-line:var-naming Package name matches the decoded image format.
package jpeg

import (
	"bytes"
	"image"
	_ "image/jpeg" // register JPEG codec
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/internal/pixel"
	"github.com/gophics/ravenporter/ir"
)

const (
	jpegFormatName = "JPEG"
	jpegName       = "jpeg"
	extJPEG        = ".jpeg"
	extJPG         = ".jpg"

	jpegMarkerPrefix = 0xFF
	jpegMarkerAPP2   = 0xE2
	jpegMarkerSOI    = 0xD8
	jpegMarkerSOS    = 0xDA
	jpegMarkerEOI    = 0xD9

	jpegSOILen        = 2
	jpegMarkerMinLen  = 4
	jpegSegLenMin     = 2
	jpegSegHdrLen     = 2
	jpegSegPayloadOff = 4
	jpegICCSeqOff     = 12
	jpegICCCountOff   = 13
	jpegICCDataOff    = 14
	jpegICCMinExtra   = 2
	jpegShift8        = 8
)

var (
	magicJPEG      = []byte{jpegMarkerPrefix, jpegMarkerSOI}
	iccProfileSig  = []byte("ICC_PROFILE\x00")
	iccProfileAttr = "ICCProfile"
)

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatJPEG, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return imgutil.ProbeBytes(r, magicJPEG) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	raw, err := imgutil.ReadAllBytes(r, opts.MaxFileSize)
	if err != nil {
		return nil, imgutil.DecodeErrStr(jpegName, err)
	}

	decoded := &ir.ImageAsset{
		Name:       jpegName,
		Format:     ir.ImageJPEG,
		Channels:   ir.ChannelRGBA,
		ColorSpace: ir.ColorSRGB,
		MipLevels:  1,
		Compressed: raw,
		Metadata:   make(map[string]string),
	}

	iccProfile := extractICCProfile(raw)
	if len(iccProfile) > 0 {
		decoded.Metadata[iccProfileAttr] = string(iccProfile)
	}

	img, _, decErr := image.Decode(bytes.NewReader(raw))
	if decErr != nil {
		return nil, imgutil.DecodeErrStr(jpegName, decErr)
	}
	bounds := img.Bounds()
	decoded.Width = bounds.Dx()
	decoded.Height = bounds.Dy()
	decoded.Channels = imgutil.ChannelCount(img)

	if err := imgutil.CheckPixelLimit(decoded.Width, decoded.Height, opts.MaxImagePixels); err != nil {
		return nil, imgutil.DecodeErrStr(jpegName, err)
	}

	decoded.PixelDecode = func(d *ir.ImageAsset) (*ir.PixelBuffer, error) {
		srcImg, _, err := image.Decode(bytes.NewReader(d.Compressed))
		if err != nil {
			return nil, err
		}
		return &ir.PixelBuffer{
			Data:     pixel.ToRGBA(srcImg, d.Width, d.Height),
			DataType: ir.DataTypeUint8,
			BitDepth: ir.BitDepth8,
		}, nil
	}

	return imgutil.BuildAsset(decoded, ir.FormatJPEG), nil
}

func (d *Decoder) Extensions() []string { return []string{extJPEG, extJPG} }
func (d *Decoder) FormatName() string   { return jpegFormatName }

func extractICCProfile(data []byte) []byte {
	if len(data) < jpegSOILen || data[0] != jpegMarkerPrefix || data[1] != jpegMarkerSOI {
		return nil
	}

	var iccParts [][]byte
	var expectedParts int
	pos := jpegSOILen

	for pos+jpegMarkerMinLen <= len(data) {
		if data[pos] != jpegMarkerPrefix {
			break
		}

		marker := data[pos+1]
		if marker == jpegMarkerSOS || marker == jpegMarkerEOI {
			break
		}

		length := int(data[pos+jpegSOILen])<<jpegShift8 | int(data[pos+jpegSOILen+1])
		if length < jpegSegLenMin {
			break
		}

		if pos+jpegSegHdrLen+length > len(data) {
			break
		}

		if marker == jpegMarkerAPP2 {
			payload := data[pos+jpegSegPayloadOff : pos+jpegSegHdrLen+length]
			if len(payload) >= len(iccProfileSig)+jpegICCMinExtra && bytes.HasPrefix(payload, iccProfileSig) {
				seqIdx := int(payload[jpegICCSeqOff])
				count := int(payload[jpegICCCountOff])

				if count > 0 && seqIdx > 0 && seqIdx <= count {
					if expectedParts == 0 {
						expectedParts = count
						iccParts = make([][]byte, expectedParts)
					}

					iccParts[seqIdx-1] = payload[jpegICCDataOff:]
				}
			}
		}

		pos += jpegSegHdrLen + length
	}

	if expectedParts == 0 {
		return nil
	}

	totalLen := 0
	for _, part := range iccParts {
		if part == nil {
			return nil // missing sequence piece
		}
		totalLen += len(part)
	}

	out := make([]byte, 0, totalLen)
	for _, part := range iccParts {
		out = append(out, part...)
	}
	return out
}
