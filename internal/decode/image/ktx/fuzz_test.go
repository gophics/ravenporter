package ktx_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/ktx"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("../testdata/minimal.ktx")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("\xabKTX 11\xbb\x0d\x0a\x1a\x0a"))
	f.Add([]byte("\xabKTX 20\xbb\x0d\x0a\x1a\x0a"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &ktx.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
