package threemf_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/threemf"
)

func FuzzDecode3MF(f *testing.F) {
	seed, err := os.ReadFile("testdata/box.3mf")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("PK\x03\x04"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		dec := &threemf.Decoder{}
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
