package webp

import (
	"bytes"
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/imgutil"
	"github.com/gophics/ravenporter/ir"
	_ "golang.org/x/image/webp" // register WebP codec
)

const (
	webpFormatName = "WebP"
	webpName       = "webp"
	extWebP        = ".webp"
	riffWebPOffset = 8
	riffProbeSize  = 12
)

var magicWebP = []byte("RIFF")
var markerWebP = []byte("WEBP")

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatWebP, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool {
	pos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	defer func() { _, _ = r.Seek(pos, io.SeekStart) }() //nolint:errcheck // reset pos

	var buf [riffProbeSize]byte
	n, err := r.Read(buf[:])
	if err != nil || n < riffProbeSize {
		return false
	}
	return bytes.HasPrefix(buf[:], magicWebP) && bytes.Equal(buf[riffWebPOffset:riffProbeSize], markerWebP)
}

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	return imgutil.DecodeStdlibImage(r, opts, webpName, ir.ImageWebP, ir.FormatWebP)
}

func (d *Decoder) Extensions() []string { return []string{extWebP} }
func (d *Decoder) FormatName() string   { return webpFormatName }
