package opus

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("../testdata/minimal.opus")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("OggS"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
