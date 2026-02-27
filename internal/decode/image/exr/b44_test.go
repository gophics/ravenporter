package exr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnpackB44Block(t *testing.T) {
	tests := []struct {
		name string
		src  [b44Packed]byte
		want uint16
	}{
		{
			name: "all_zeros",
			src:  [b44Packed]byte{},
			want: 0,
		},
		{
			name: "base_value_only",
			src: func() [b44Packed]byte {
				var b [b44Packed]byte
				b[0] = 0x00
				b[1] = 0x3C
				return b
			}(),
			want: 0x3C00,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			block := unpackB44Block(tc.src[:])
			assert.Equal(t, tc.want, block[0])
		})
	}
}

func TestUnpackB44Block_ZeroDiffs(t *testing.T) {
	var src [b44Packed]byte
	src[0] = 0x00
	src[1] = 0x3C

	block := unpackB44Block(src[:])

	for i, v := range block {
		assert.Equal(t, uint16(0x3C00), v, "pixel %d must equal base when diffs are zero", i)
	}
}

func TestB44Decompress(t *testing.T) {
	tests := []struct {
		name  string
		nChan int
		w     int
		h     int
		build func() []byte
	}{
		{
			name:  "1x1_single_channel",
			nChan: 1,
			w:     1,
			h:     1,
			build: func() []byte { return make([]byte, b44Packed) },
		},
		{
			name:  "4x4_single_block",
			nChan: 1,
			w:     4,
			h:     4,
			build: func() []byte { return make([]byte, b44Packed) },
		},
		{
			name:  "5x5_partial_blocks",
			nChan: 1,
			w:     5,
			h:     5,
			build: func() []byte { return make([]byte, b44Packed*4) },
		},
		{
			name:  "4x4_two_channels",
			nChan: 2,
			w:     4,
			h:     4,
			build: func() []byte { return make([]byte, b44Packed*2) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.build()
			dst := make([]byte, tc.nChan*tc.w*tc.h*b44Half)
			b44Decompress(dst, src, tc.nChan, tc.w, tc.h)
			assert.Equal(t, tc.nChan*tc.w*tc.h*b44Half, len(dst))
		})
	}
}

func TestB44Decompress_FlatBlock(t *testing.T) {
	src := []byte{0x00, 0x3C, 0xFC}
	dst := make([]byte, 4*4*b44Half)

	b44Decompress(dst, src, 1, 4, 4)

	for i := 0; i < len(dst)-1; i += b44Half {
		val := uint16(dst[i]) | uint16(dst[i+1])<<8
		assert.Equal(t, uint16(0x3C00), val, "pixel %d must be flat value", i/b44Half)
	}
}

func TestB44Decompress_EmptyInput(t *testing.T) {
	dst := make([]byte, 4*4*b44Half)
	b44Decompress(dst, nil, 1, 4, 4)
	assert.Equal(t, 4*4*b44Half, len(dst))
}

func TestHalfAdd(t *testing.T) {
	tests := []struct {
		name string
		base uint16
		diff int
		want uint16
	}{
		{"positive", 100, 50, 150},
		{"negative", 100, -50, 50},
		{"clamp_low", 10, -20, 0},
		{"clamp_high", 0xFFF0, 0x20, 0xFFFF},
		{"zero", 0, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, halfAdd(tc.base, tc.diff))
		})
	}
}

func BenchmarkUnpackB44Block(b *testing.B) {
	var src [b44Packed]byte
	src[0] = 0x00
	src[1] = 0x3C
	src[2] = 0x08

	b.ReportAllocs()
	for b.Loop() {
		_ = unpackB44Block(src[:])
	}
}

func BenchmarkB44Decompress(b *testing.B) {
	src := make([]byte, b44Packed*4)
	dst := make([]byte, 8*8*b44Half)
	b.ReportAllocs()
	for b.Loop() {
		b44Decompress(dst, src, 1, 8, 8)
	}
}
