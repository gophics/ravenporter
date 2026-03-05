package mp3

import (
	"bytes"
	"os"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func BenchmarkDecode(b *testing.B) {
	data, err := os.ReadFile("../testdata/minimal.mp3")
	if err != nil {
		b.Skip("testdata not available")
	}
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = d.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	}
}

func BenchmarkParseFrameHeader(b *testing.B) {
	frame := buildFrame(0x3, 0x0, 0x9, 0x0)
	data := make([]byte, 1024)
	copy(data, frame)
	b.ReportAllocs()
	for b.Loop() {
		parseFrameHeader(data)
	}
}

func BenchmarkCalcDuration(b *testing.B) {
	frame := buildFrame(0x3, 0x0, 0x9, 0x0)
	frameSize := (spfLayer3MPEG1 / 8 * 128 * bitrateMultiple) / 44100
	data := make([]byte, frameSize*100)
	for i := 0; i < len(data)-len(frame); i += frameSize {
		copy(data[i:], frame)
	}
	b.ReportAllocs()
	for b.Loop() {
		calcDuration(data, 44100)
	}
}

func BenchmarkProbe(b *testing.B) {
	data := magicID3
	r := bytes.NewReader(data)
	d := &Decoder{}
	b.ReportAllocs()
	for b.Loop() {
		r.Reset(data)
		d.Probe(r)
	}
}

func BenchmarkSkipID3v2(b *testing.B) {
	data := buildID3v2(1024)
	b.ReportAllocs()
	for b.Loop() {
		skipID3v2(data)
	}
}

func BenchmarkRequantize(b *testing.B) {
	var origFreq [freqLines]float64
	for i := range freqLines {
		origFreq[i] = float64(i % 10)
	}
	gi := &granuleInfo{globalGain: 210}
	var sf [39]int
	b.ReportAllocs()
	for b.Loop() {
		var freq [freqLines]float64
		copy(freq[:], origFreq[:])
		requantize(gi, freq[:], sf[:], 0, &mp3Info{version: 3})
	}
}

func BenchmarkAliasReduce(b *testing.B) {
	var freq [freqLines]float64
	for i := range freqLines {
		freq[i] = float64(i) * 0.001
	}
	b.ReportAllocs()
	for b.Loop() {
		aliasReduce(freq[:])
	}
}

func BenchmarkIMDCTLong(b *testing.B) {
	var input [samplesPerSB]float64
	var overlap [samplesPerSB]float64
	var output [samplesPerSB]float64
	for i := range samplesPerSB {
		input[i] = float64(i) * 0.01
	}
	b.ReportAllocs()
	for b.Loop() {
		imdctLong(input[:], overlap[:], output[:], blockTypeLong)
	}
}

func BenchmarkIMDCTShort(b *testing.B) {
	var input [samplesPerSB]float64
	var overlap [samplesPerSB]float64
	var output [samplesPerSB]float64
	for i := range samplesPerSB {
		input[i] = float64(i) * 0.01
	}
	b.ReportAllocs()
	for b.Loop() {
		imdctShort(input[:], overlap[:], output[:])
	}
}

func BenchmarkSynthesis(b *testing.B) {
	var s synthesizer
	var samples [subbands]float64
	for i := range subbands {
		samples[i] = float64(i) * 0.01
	}
	out := make([]float32, subbands)
	b.ReportAllocs()
	for b.Loop() {
		s.processSamples(samples, 0, out, 0)
	}
}
