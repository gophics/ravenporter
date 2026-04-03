package cache

import (
	"bytes"
	"testing"

	"github.com/gophics/ravenporter"
)

func FuzzRead(f *testing.F) {
	result := &ravenporter.Result{
		Asset:  fullScene(),
		Report: fullReport(),
	}

	var cooked bytes.Buffer
	if err := Write(&cooked, result); err != nil {
		f.Fatalf("seed cache write failed: %v", err)
	}

	f.Add(cooked.Bytes())
	f.Add([]byte("not-a-cache"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = Read(bytes.NewReader(data), int64(len(data)))
	})
}
