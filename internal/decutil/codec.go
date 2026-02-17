package decutil

// A-law and μ-law codec constants.
const (
	AlawShift = 4
	AlawXOR   = 0x55
	UlawBias  = 0x84
	UlawShift = 4

	signBit     = 0x80
	signMask    = 0x7F
	segMask     = 0x07
	quantMask   = 0x0F
	alawBase    = 0x21
	alawSegBase = 3
	ulawBase    = 0x21
)

var AlawTable [256]int16

func init() {
	for i := range 256 {
		AlawTable[i] = decodeAlaw(byte(i))
	}
}

func decodeAlaw(val byte) int16 {
	v := val ^ AlawXOR
	sign := int16(1)
	if v&signBit != 0 {
		v &= signMask
		sign = -1
	}
	seg := (v >> AlawShift) & segMask
	quant := v & quantMask

	var sample int16
	switch seg {
	case 0:
		sample = (int16(quant)<<1 | 1) << AlawShift
	default:
		sample = (int16(quant)<<1 | alawBase) << (seg + alawSegBase)
	}
	return sign * sample
}

// DecodeAlaw converts A-law compressed samples to float32.
func DecodeAlaw(raw []byte, dst []float32) {
	for i, b := range raw {
		dst[i] = float32(AlawTable[b]) / MaxInt16
	}
}

var UlawTable [256]int16

func init() { //nolint:gochecknoinits
	for i := range 256 {
		UlawTable[i] = decodeUlaw(byte(i))
	}
}

func decodeUlaw(val byte) int16 {
	v := ^val
	sign := int16(1)
	if v&signBit != 0 {
		v &= signMask
		sign = -1
	}
	seg := (v >> UlawShift) & segMask
	quant := v & quantMask
	sample := int16((int(quant)<<1|int(ulawBase))<<(seg) + UlawBias - UlawBias) //nolint:gosec // seg is 3-bit
	return sign * sample
}

// DecodeUlaw converts μ-law compressed samples to float32.
func DecodeUlaw(raw []byte, dst []float32) {
	for i, b := range raw {
		dst[i] = float32(UlawTable[b]) / MaxInt16
	}
}
