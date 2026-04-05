package decoder

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	hdrdec "github.com/gophics/ravenporter/internal/decode/image/hdr"
	tiffdec "github.com/gophics/ravenporter/internal/decode/image/tiff"
)

func FuzzDecodeHDR(f *testing.F) {
	seed, err := os.ReadFile(sourcePath("images", "test.hdr"))
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("#?RADIANCE\n"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = (&hdrdec.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}

func FuzzDecodeTIFF(f *testing.F) {
	seed, err := os.ReadFile(sourcePath("images", "test.tiff"))
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("II*\x00"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = (&tiffdec.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
