package wav

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeIMANibble(t *testing.T) {
	tests := []struct {
		name    string
		nibble  int
		initPre int
		initIdx int
		want    int16
	}{
		{"zero_nibble", 0x0, 0, 0, 0},
		{"positive_step", 0x4, 0, 7, 15},
		{"negative_step", 0xC, 100, 7, 85},
		{"clamp_high", 0x7, 32760, 88, 32767},
		{"clamp_low", 0xF, -32760, 88, -32768},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &imaState{predictor: tc.initPre, index: tc.initIdx}
			got := decodeIMANibble(tc.nibble, s)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDecodeIMANibble_IndexClamp(t *testing.T) {
	s := &imaState{predictor: 0, index: 0}
	decodeIMANibble(0x0, s)
	assert.Equal(t, 0, s.index)

	s = &imaState{predictor: 0, index: imaIndexMax}
	decodeIMANibble(0x7, s)
	assert.Equal(t, imaIndexMax, s.index)
}

func TestDecodeIMABlock_Mono(t *testing.T) {
	block := []byte{
		0x00, 0x00, 0x07, 0x00, // header: sample=0, index=7, reserved=0
		0x10, // two nibbles: lo=0x0, hi=0x1
		0x32, // two nibbles: lo=0x2, hi=0x3
	}

	states := make([]imaState, 1)
	dst := decodeIMABlock(block, 1, 5, states, nil)

	assert.Len(t, dst, 5)
	assert.InDelta(t, 0.0, dst[0], 0.001)
}

func TestDecodeIMABlock_Silence(t *testing.T) {
	block := make([]byte, 8)
	block[2] = 0 // index = 0 (step = 7)

	states := make([]imaState, 1)
	dst := decodeIMABlock(block, 1, 9, states, nil)

	assert.NotEmpty(t, dst)
	for _, s := range dst {
		assert.InDelta(t, 0.0, s, 0.1)
	}
}

func TestDecodeIMABlock_ShortBlock(t *testing.T) {
	states := make([]imaState, 1)
	dst := decodeIMABlock([]byte{0x00}, 1, 5, states, nil)
	assert.Empty(t, dst)
}

func TestDecodeIMABlock_Stereo(t *testing.T) {
	block := make([]byte, 16)
	// ch0 header
	block[0] = 0x00 // sample low
	block[1] = 0x00 // sample high
	block[2] = 0x07 // index = 7
	block[3] = 0x00 // reserved
	// ch1 header
	block[4] = 0x00
	block[5] = 0x00
	block[6] = 0x07
	block[7] = 0x00

	states := make([]imaState, 2)
	dst := decodeIMABlock(block, 2, 9, states, nil)
	assert.NotEmpty(t, dst)
}

func BenchmarkDecodeIMABlock_Mono(b *testing.B) {
	block := make([]byte, 1024)
	block[0] = 0x00
	block[1] = 0x00
	block[2] = 0x07
	block[3] = 0x00

	states := make([]imaState, 1)
	dst := make([]float32, 0, 2048)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		dst = decodeIMABlock(block, 1, 2048, states, dst[:0])
	}
}
