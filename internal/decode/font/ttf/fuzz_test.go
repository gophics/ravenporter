package ttf_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/font/ttf"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/minimal.ttf")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("\x00\x01\x00\x00"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &ttf.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
