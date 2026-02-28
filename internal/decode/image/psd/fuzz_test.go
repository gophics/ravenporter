package psd_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/psd"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("../testdata/minimal.psd")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("8BPS"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &psd.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
