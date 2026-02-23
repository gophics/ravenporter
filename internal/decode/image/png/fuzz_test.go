package png_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/png"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/red4x4.png")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("\x89PNG\r\n\x1a\n"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &png.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
