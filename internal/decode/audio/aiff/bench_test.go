package aiff

import (
	"bytes"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func BenchmarkDecode_16bit_Mono(b *testing.B) {
	pcm := make([]byte, 44100*2)
	data := buildAIFF(1, 44100, 16, 44100, pcm)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecode_16bit_Stereo(b *testing.B) {
	pcm := make([]byte, 44100*4)
	data := buildAIFF(2, 44100, 16, 44100, pcm)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecode_24bit_Mono(b *testing.B) {
	pcm := make([]byte, 44100*3)
	data := buildAIFF(1, 44100, 24, 44100, pcm)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecode_AIFC_Sowt(b *testing.B) {
	pcm := make([]byte, 44100*2)
	data := buildAIFC("sowt", 1, 44100, 16, 44100, pcm)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecode_AIFC_Alaw(b *testing.B) {
	pcm := make([]byte, 8000)
	data := buildAIFC("alaw", 1, 8000, 8, 8000, pcm)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkDecode_AIFC_IMA4(b *testing.B) {
	pcm := make([]byte, 34*100)
	data := buildAIFC("ima4", 1, 6400, 16, 22050, pcm)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkProbe(b *testing.B) {
	data := []byte("FORM\x00\x00\x00\x00AIFF")
	r := bytes.NewReader(data)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		r.Reset(data)
		d.Probe(r)
	}
}
