package flac

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("../testdata/minimal.flac")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("fLaC"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		dec := &Decoder{}
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
