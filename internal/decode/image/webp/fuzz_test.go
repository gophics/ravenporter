package webp_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/webp"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/webp_small.webp")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("RIFF\x00\x00\x00\x00WEBP"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &webp.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
