package tga_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/tga"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("../testdata/blue_2x2.tga")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add(make([]byte, 18))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &tga.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
