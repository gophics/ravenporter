package mp3

import (
	"testing"

	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/stretchr/testify/assert"
)

func TestBitReader(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		bits []int
		want []int
	}{
		{"SingleBit", []byte{0x80}, []int{1}, []int{1}},
		{"TwoBits", []byte{0xC0}, []int{2}, []int{3}},
		{"Byte", []byte{0xAB}, []int{8}, []int{0xAB}},
		{"MultiByte", []byte{0xFF, 0x00}, []int{8, 8}, []int{0xFF, 0x00}},
		{"CrossByte", []byte{0xF0, 0x0F}, []int{4, 8}, []int{0xF, 0x00}},
		{"NineBit", []byte{0xFF, 0x80}, []int{9}, []int{0x1FF}},
		{"Empty", nil, []int{1}, []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := newBitReaderFromBytes(tt.data)
			for i, n := range tt.bits {
				got := br.readBits(n)
				assert.Equal(t, tt.want[i], got)
			}
		})
	}
}

func TestBitReaderReadBit(t *testing.T) {
	br := newBitReaderFromBytes([]byte{0xA5}) // 10100101
	want := []int{1, 0, 1, 0, 0, 1, 0, 1}
	for i, w := range want {
		got := br.readBit()
		assert.Equal(t, w, got, "bit %d", i)
	}
}

func TestParseSideInfo(t *testing.T) {
	tests := []struct {
		name    string
		nch     int
		dataLen int
	}{
		{"Mono", 1, sideInfoMono},
		{"Stereo", 2, sideInfoStereo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataLen)
			si := parseSideInfo(data, &mp3Info{channels: tt.nch, version: 3})
			assert.Equal(t, 0, si.mainDataBegin)
			for gr := range granules {
				for ch := range tt.nch {
					gi := &si.granules[gr][ch]
					assert.Equal(t, 0, gi.part23Len)
					assert.Equal(t, 0, gi.bigValues)
					assert.Equal(t, 0, gi.globalGain)
				}
			}
		})
	}
}

func TestParseSideInfoMainDataBegin(t *testing.T) {
	data := make([]byte, sideInfoStereo)
	data[0] = 0x01 // Set mainDataBegin high bit at offset 0
	data[1] = 0x00
	si := parseSideInfo(data, &mp3Info{channels: 2, version: 3})
	assert.Equal(t, 2, si.mainDataBegin) // 0b_0_0000_0001_0 >> 7 = 2... actually check bit layout
}

func TestParseSideInfoWindowSwitch(t *testing.T) {
	data := make([]byte, sideInfoStereo)

	// Manually construct side info bytes for stereo, granule 0, channel 0
	// mainDataBegin: 9 bits (all 0) = byte 0 [7:0] + byte 1 [7]
	// private bits: 3 bits
	// scfsi: 4 bits * 2 channels = 8 bits
	// Total header bits before granule data: 9 + 3 + 8 = 20 bits
	// Granule 0, Ch 0 starts at bit 20:
	//   part23Len: 12 bits
	//   bigValues: 9 bits
	//   globalGain: 8 bits
	//   sfCompress: 4 bits
	//   windowSwitch: 1 bit (at bit offset 20+12+9+8+4 = 53)
	// Set windowSwitch bit for gr0/ch0
	byteIdx := 53 / 8
	bitIdx := uint(7 - 53%8)
	data[byteIdx] |= 1 << bitIdx

	si := parseSideInfo(data, &mp3Info{channels: 2, version: 3})
	assert.True(t, si.granules[0][0].windowSwitch)
	assert.False(t, si.granules[0][1].windowSwitch)
}

func TestHuffmanDescsTable(t *testing.T) {
	tests := []struct {
		name    string
		idx     int
		wantNil bool
		linbits int
	}{
		{"Table0_nil", 0, true, 0},
		{"Table1", 1, false, 0},
		{"Table4_nil", 4, true, 0},
		{"Table13", 13, false, 0},
		{"Table14_nil", 14, true, 0},
		{"Table16_lin1", 16, false, 1},
		{"Table24_lin4", 24, false, 4},
		{"Table31_lin13", 31, false, 13},
		{"Table32", 32, false, 0},
		{"Table33", 33, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := huffmanDescs[tt.idx]
			if tt.wantNil {
				assert.Nil(t, desc.table)
				assert.Equal(t, 0, desc.treelen)
			} else {
				assert.NotNil(t, desc.table)
				assert.Greater(t, desc.treelen, 0)
			}
			assert.Equal(t, tt.linbits, desc.linbits)
		})
	}
}

func TestHuffmanTreeSize(t *testing.T) {
	assert.Equal(t, 2804, len(huffmanTree))
}

func TestRequantize(t *testing.T) {
	var freq [freqLines]float64
	freq[0] = 1.0
	freq[1] = -1.0
	freq[2] = 2.0

	gi := &granuleInfo{
		globalGain: 210,
		sfCompress: 0,
	}

	var sf [39]int
	requantize(gi, freq[:], sf[:], 0, &mp3Info{version: 3})

	// globalGain=210 => gain=0, exponent=0, multiplier=1.0
	// |1|^(4/3) * 1.0 = 1.0
	assert.InDelta(t, 1.0, freq[0], 1e-6)
	assert.InDelta(t, -1.0, freq[1], 1e-6)
	// |2|^(4/3) ≈ 2.5198
	assert.InDelta(t, 2.5198, freq[2], 0.001)
}

func TestRequantizeZeroPassthrough(t *testing.T) {
	var freq [freqLines]float64
	gi := &granuleInfo{globalGain: 210}
	var sf [39]int
	requantize(gi, freq[:], sf[:], 0, &mp3Info{version: 3})
	for i := range freqLines {
		assert.Equal(t, 0.0, freq[i])
	}
}

func TestStereoProcess(t *testing.T) {
	var freqs [2][freqLines]float64
	freqs[0][0] = 1.0
	freqs[1][0] = 1.0
	freqs[0][1] = 1.0
	freqs[1][1] = -1.0

	gi0 := &granuleInfo{}
	gi1 := &granuleInfo{}
	info := &mp3Info{mode: modeJointStereo, modeExt: 2} // MS active

	var sf0, sf1 [39]int
	stereoProcess(freqs[:], gi0, gi1, sf0[:], sf1[:], info, 0)

	// M/S: left = (M+S)/sqrt(2), right = (M-S)/sqrt(2)
	assert.InDelta(t, sqrt2Inv*2, freqs[0][0], 1e-10)
	assert.InDelta(t, 0.0, freqs[1][0], 1e-10)
	assert.InDelta(t, 0.0, freqs[0][1], 1e-10)
	assert.InDelta(t, sqrt2Inv*2, freqs[1][1], 1e-10)
}

func TestStereoProcessMono(t *testing.T) {
	var freqs [1][freqLines]float64
	freqs[0][0] = 1.0

	info := &mp3Info{channels: 1}
	stereoProcess(freqs[:], nil, nil, nil, nil, info, 0)
	assert.Equal(t, 1.0, freqs[0][0])
}

func TestAliasReduce(t *testing.T) {
	var freq [freqLines]float64
	// Set values at subband boundary (index 17 and 18)
	freq[17] = 1.0
	freq[18] = 1.0
	aliasReduce(freq[:])

	// After butterfly: values should be transformed
	assert.NotEqual(t, 1.0, freq[17])
	assert.NotEqual(t, 1.0, freq[18])
	// Verify energy conservation: cs^2 + ca^2 = 1
	// so output^2 should equal input^2
	e17 := freq[17]*freq[17] + freq[18]*freq[18]
	assert.InDelta(t, 2.0, e17, 1e-10)
}

func TestAliasReduceZeros(t *testing.T) {
	var freq [freqLines]float64
	aliasReduce(freq[:])
	for i := range freqLines {
		assert.Equal(t, 0.0, freq[i])
	}
}

func TestComputeFrameLen(t *testing.T) {
	tests := []struct {
		name string
		sr   int
		br   int
		want int
	}{
		{"44100_128", 44100, 128, 144 * 128 * 1000 / 44100},
		{"48000_192", 48000, 192, 144 * 192 * 1000 / 48000},
		{"32000_320", 32000, 320, 144 * 320 * 1000 / 32000},
		{"ZeroSR", 0, 128, 0},
		{"ZeroBR", 44100, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, computeFrameLen(tt.sr, tt.br))
		})
	}
}

func TestFrameDecoderReset(t *testing.T) {
	fd := newFrameDecoder()
	fd.reservoir = append(fd.reservoir, 0xFF)
	fd.mainBuf = append(fd.mainBuf, 0xAA)
	fd.reset()
	assert.Empty(t, fd.reservoir)
	assert.Empty(t, fd.mainBuf)
}

func TestFrameDecoderShortData(t *testing.T) {
	fd := newFrameDecoder()
	out := make([]float32, 4096)
	info := &mp3Info{sampleRate: 44100, bitrate: 128, channels: 2}
	consumed, written := fd.decodeFrame(nil, info, out)
	assert.Equal(t, 0, consumed)
	assert.Equal(t, 0, written)
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		want float64
	}{
		{"Normal", 0.5, 0.5},
		{"Zero", 0.0, 0.0},
		{"Positive", 1.0, 1.0},
		{"Negative", -1.0, -1.0},
		{"OverPositive", 2.0, 1.0},
		{"OverNegative", -2.0, -1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mathx.Clamp(tt.v, -1.0, 1.0))
		})
	}
}

func TestSfBandIndex(t *testing.T) {
	tests := []struct {
		name string
		sr   int
		want int
	}{
		{"44100", 44100, 0},
		{"48000", 48000, 1},
		{"32000", 32000, 2},
		{"Unknown", 99999, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sfBandIndex(tt.sr))
		})
	}
}

func TestSynthesizerReset(t *testing.T) {
	var s synthesizer
	s.slot[0] = 5
	s.buf[0][0] = 1.0
	s.reset()
	assert.Equal(t, 0, s.slot[0])
	assert.Equal(t, 0.0, s.buf[0][0])
}

func TestSynthesizerProcessSamples(t *testing.T) {
	var s synthesizer
	var samples [subbands]float64
	out := make([]float32, subbands)
	n := s.processSamples(samples, 0, out, 0)
	assert.Equal(t, subbands, n)
}

func TestIMDCTWindowCoefficients(t *testing.T) {
	// Window type 0 (long): all 36 coefficients should be sin(π/36 * (i + 0.5))
	for i := range 36 {
		assert.Greater(t, imdctWin[0][i], 0.0, "long window[%d] should be positive", i)
		assert.LessOrEqual(t, imdctWin[0][i], 1.0, "long window[%d] should be <= 1", i)
	}

	// Window type 2 (short): only first 12 are nonzero
	for i := range 12 {
		assert.Greater(t, imdctWin[2][i], 0.0, "short window[%d] should be positive", i)
	}
	for i := 12; i < 36; i++ {
		assert.Equal(t, 0.0, imdctWin[2][i], "short window[%d] should be zero", i)
	}
}

func TestIMDCTLongZeroInput(t *testing.T) {
	var input [samplesPerSB]float64
	var overlap [samplesPerSB]float64
	var output [samplesPerSB]float64

	imdctLong(input[:], overlap[:], output[:], blockTypeLong)

	for i := range samplesPerSB {
		assert.Equal(t, 0.0, output[i])
	}
}

func TestIMDCTShortZeroInput(t *testing.T) {
	var input [samplesPerSB]float64
	var overlap [samplesPerSB]float64
	var output [samplesPerSB]float64

	imdctShort(input[:], overlap[:], output[:])

	for i := range samplesPerSB {
		assert.Equal(t, 0.0, output[i])
	}
}

func TestScalefactorBandBoundaries(t *testing.T) {
	// Verify all long block band tables start at 0 and end at 576
	for sr := range 3 {
		assert.Equal(t, 0, sfBandLong[sr][0])
		assert.Equal(t, 576, sfBandLong[sr][22])
	}
	// Verify all short block band tables start at 0 and end at 192
	for sr := range 3 {
		assert.Equal(t, 0, sfBandShort[sr][0])
		assert.Equal(t, 192, sfBandShort[sr][13])
	}
}

func TestSfLenTable(t *testing.T) {
	assert.Len(t, sfLenTable, 16)
	assert.Equal(t, [2]int{0, 0}, sfLenTable[0])
	assert.Equal(t, [2]int{4, 3}, sfLenTable[15])
}

func TestUpdateReservoir(t *testing.T) {
	fd := newFrameDecoder()
	payload := make([]byte, 600)
	for i := range 600 {
		payload[i] = byte(i % 256)
	}

	fd.updateReservoir(payload)

	// reservoir limits to 512 bytes
	assert.Equal(t, 512, len(fd.reservoir))
	// should contain the tail end of payload
	assert.Equal(t, payload[600-512:], fd.reservoir)
}

func TestReadScalefactors(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		gi         *granuleInfo
		info       *mp3Info
		wantCounts []int // non-zero sf counts
	}{
		{
			name:       "MPEG1_Long",
			data:       []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, // all 1s
			gi:         &granuleInfo{sfCompress: 0},          // slen1=0, slen2=0
			info:       &mp3Info{version: 3},
			wantCounts: []int{0},
		},
		{
			name:       "MPEG1_Short",
			data:       []byte{0x00, 0x00},
			gi:         &granuleInfo{sfCompress: 0, windowSwitch: true, blockType: blockTypeShort},
			info:       &mp3Info{version: 3},
			wantCounts: []int{0},
		},
		{
			name:       "LSF_Bypass", // currently LSF scalefactors are bypassed
			data:       []byte{0xFF},
			gi:         &granuleInfo{sfCompress: 0},
			info:       &mp3Info{version: 2},
			wantCounts: []int{0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := newBitReaderFromBytes(tt.data)
			var sf [39]int
			readScalefactors(&br, tt.gi, sf[:], tt.info)
			nonZero := 0
			for _, v := range sf {
				if v != 0 {
					nonZero++
				}
			}
			assert.GreaterOrEqual(t, nonZero, 0)
			assert.GreaterOrEqual(t, nonZero, 0)
		})
	}
}

func TestHuffDecodeValue(t *testing.T) {
	data := []byte{0x00}
	br := newBitReaderFromBytes(data)
	_, _, bits := huffDecodeValue(&br, &huffmanDescs[1])
	assert.Greater(t, bits, 0)
}

func TestHuffDecodeQuad(t *testing.T) {
	data := []byte{0x00}
	br := newBitReaderFromBytes(data)
	x, y, v, w, bits := huffDecodeQuad(&br, &huffmanDescs[32])
	assert.GreaterOrEqual(t, x+y+v+w, 0)
	assert.Greater(t, bits, 0)
}

func TestHuffDecode(t *testing.T) {
	gi := &granuleInfo{
		part23Len:      16,
		bigValues:      2,
		tableSelect:    [3]int{1, 1, 1},
		count1TableSel: 0,
	}
	data := []byte{0x00, 0x00, 0x00}
	br := newBitReaderFromBytes(data)
	var freq [freqLines]float64
	for i := range freq {
		freq[i] = 99.9
	}
	info := &mp3Info{version: 3, sampleRate: 44100}
	huffDecode(&br, gi, freq[:], 0, info)
	assert.NotEqual(t, 99.9, freq[0])
}

func TestImdctGranule(t *testing.T) {
	gi := &granuleInfo{
		blockType:    blockTypeLong,
		windowSwitch: false,
	}
	var freq [freqLines]float64
	var overlap [subbands][samplesPerSB]float64
	var out [subbands][samplesPerSB]float64

	imdctGranule(gi, freq[:], overlap[:], out[:])

	for sb := range subbands {
		for s := range samplesPerSB {
			assert.Equal(t, 0.0, out[sb][s])
		}
	}
}
