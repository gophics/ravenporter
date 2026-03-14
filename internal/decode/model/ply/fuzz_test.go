package ply

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("testdata/ascii_triangle.ply")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("ply\nformat ascii 1.0\nend_header\n"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &Decoder{}
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
