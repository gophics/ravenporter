package usda

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/comprehensive.usdc")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("#usda 1.0\n"))
	f.Add([]byte("PXR-USDC"))
	f.Add([]byte("#usda 1.0\n\ndef Mesh \"M\" {\n    point3f[] points = []\n}\n"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		dec := &Decoder{}
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
