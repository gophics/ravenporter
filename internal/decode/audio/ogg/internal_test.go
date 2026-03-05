package ogg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyResidueVQ(t *testing.T) {
	tests := []struct {
		name        string
		residueType uint32
	}{
		{"Type0", 0},
		{"Type1", 1},
		{"Type2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := &frameDecoder{
				channelData: [][]float32{
					make([]float32, 10),
					make([]float32, 10),
				},
			}
			residue := &vorbisResidue{residueType: int(tt.residueType)}
			cb := &vorbisCodebook{
				entries:    2,
				dimensions: 2,
				lookupVals: []float32{0.0, 1.0, 2.0, 3.0},
			}
			chToDecode := []int{0, 1}

			// entry=0, begin=0, partitionCount=0, partitionSize=2, i=0
			applyResidueVQ(fd, residue, cb, 0, chToDecode, 0, 0, 2, 0)

			// Ensure some data got written
			sum := float32(0)
			for _, ch := range fd.channelData {
				for _, v := range ch {
					sum += v
				}
			}
			assert.True(t, sum > 0, "expected residue data to be applied")
		})
	}
}

func TestDecodeResidue_ZeroPartitions(t *testing.T) {
	_ = t
	// Should return early if partitionsToRead == 0
	fd := &frameDecoder{
		prevSize: 128,
		setup: &vorbisSetup{
			codebooks: []vorbisCodebook{{dimensions: 2}}, // prevent panic on line 57
		},
	}
	residue := &vorbisResidue{begin: 0, end: 0, partitionSize: 32, classbook: 0} // nToRead = 0

	decodeResidue(nil, fd, residue, nil)
	// If it doesn't crash, the zero partition early-return works
}
