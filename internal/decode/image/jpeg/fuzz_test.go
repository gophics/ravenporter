package jpeg_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/jpeg"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/red4x4.jpg")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("\xff\xd8\xff"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &jpeg.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
