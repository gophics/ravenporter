package bmp_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/bmp"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/red4x4.bmp")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("BM"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &bmp.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
