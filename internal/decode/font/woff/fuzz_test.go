package woff_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/font/woff"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/minimal.woff")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("wOFF"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &woff.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
