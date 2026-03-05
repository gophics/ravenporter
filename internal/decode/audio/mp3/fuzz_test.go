package mp3

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func FuzzDecode(f *testing.F) {
	seed, err := os.ReadFile("../testdata/minimal.mp3")
	if err != nil {
		f.Fatalf("failed to read required fuzz seed corpus: %v", err)
	}
	f.Add(seed)
	f.Add([]byte("ID3\x03\x00\x00\x00\x00\x00\x00"))
	f.Add([]byte{0xFF, 0xFB, 0x90, 0x44, 0x00, 0x00, 0x00})

	f.Fuzz(func(_ *testing.T, data []byte) {
		dec := &Decoder{}
		_, _ = dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	})
}
