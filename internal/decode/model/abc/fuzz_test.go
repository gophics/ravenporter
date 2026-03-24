package abc

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/cube.abc")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("Ogawa\x00\x00\x00"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		dec := &Decoder{}
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
