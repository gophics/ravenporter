package flac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeVerbatimInto(t *testing.T) {
	// 5 samples, 16bps = 5*16 = 80 bits = 10 bytes
	data := []byte{
		0x00, 0x01, // 1
		0x00, 0x02, // 2
		0x00, 0x03, // 3
		0x00, 0x04, // 4
		0x00, 0x05, // 5
	}
	br := newBitReader(data)
	dst := make([]int32, 5)

	ok := decodeVerbatimInto(br, dst, 16)
	assert.True(t, ok)
	assert.Equal(t, int32(1), dst[0])
	assert.Equal(t, int32(5), dst[4])
}

func TestDecodeFixedInto(t *testing.T) {
	// To test decodeFixedInto, we need:
	// 1. Warm-up samples matching 'order'.
	//    Let's test order=2. So 2 samples. We'll use bps=8.
	//    Sample 1 = 10, Sample 2 = 20.
	// 2. decodeResidualsInto.
	//    method: 2 bits (00)
	//    partOrder: 4 bits (0000) -> 1 partition
	//    param: 4 bits (0000)
	//    samples: unary encoded. We need 3 samples. Each sample: unary 0 (bit 1), sign bit (0). So 10 (binary).
	// BlockSize = 5.
	//
	// Bit sequence:
	// Warm-up (order=2 * 8bps) = 16 bits = 0x0A, 0x14
	// method (2 bits)=0, partOrder (4 bits)=0, param (4 bits)=0 => 10 bits of 0.
	// 3 samples * 2 bits = 6 bits of 10.
	// Total bits after warmup: 10 + 6 = 16 bits.
	// Let's pack:
	// 10 bits of 0: 00000000 00
	// 6 bits of 10: 101010
	// Together 16 bits: 00000000 00101010 = 0x00, 0x2A

	data := []byte{
		0x0A, 0x14, // warmup
		0x00, 0x2A, // residuals
	}

	fd := &frameDecoder{
		residBuf: make([]int32, 3),
	}
	br := newBitReader(data)
	dst := make([]int32, 5)

	ok := fd.decodeFixedInto(br, dst, 8, 2)
	assert.True(t, ok)
	assert.Equal(t, int32(10), dst[0])
	assert.Equal(t, int32(20), dst[1])
	assert.True(t, dst[2] != 0) // check prediction applied
}
