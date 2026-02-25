package dds_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/dds"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("../testdata/minimal.dds")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("DDS "))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &dds.Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
