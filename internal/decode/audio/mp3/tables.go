package mp3

import "math"

const (
	granules       = 2
	maxChannels    = 2
	subbands       = 32
	samplesPerSB   = 18
	freqLines      = 576
	sideInfoMono   = 17
	sideInfoStereo = 32

	blockTypeLong  = 0
	blockTypeStart = 1
	blockTypeShort = 2
	blockTypeStop  = 3

	modeJointStereo = 1
	sqrt2Inv        = 0.7071067811865476

	synthSlots    = 16
	synthBufLen   = 1024
	synthBufMask  = synthBufLen - 1
	synthMatrixN  = 64
	synthWindowN  = 512
	synthStride   = 2
	synthWinBands = 16

	// Side information field widths (ISO 11172-3 §2.4.1.7).
	bitsMainDataBegin    = 9
	bitsMainDataBeginLSF = 8
	bitsPrivateMono      = 5
	bitsPrivateMonoLSF   = 1
	bitsPrivateStereo    = 3
	bitsPrivateStereoLSF = 2
	bitsPart23Len        = 12
	bitsBigValues        = 9
	bitsGlobalGain       = 8
	bitsSfCompress       = 4
	bitsSfCompressLSF    = 9
	bitsBlockType        = 2
	bitsTableSelect      = 5
	bitsSubBlockGain     = 3
	bitsRegion0Count     = 4
	bitsRegion1Count     = 3
	scfsiBands           = 4

	// Implicit region counts for windowed (short/mixed) blocks.
	region0Short = 8
	region1Short = 36
	region0Mixed = 7
	region1Mixed = 13

	// Bit manipulation constants for Huffman tree traversal.
	huffNodeMask = 0xff00
	huffLoByte   = 0xff
	huffLoNibble = 0xf
	huffEscape   = 250
	huffMaxBits  = 32
	huffLinMax   = 15

	// Huffman count1 table base index.
	count1TableBase = 32
	count1Stride    = 4

	// Requantization constants (ISO 11172-3 §2.4.3.4).
	globalGainOffset  = 210
	requantExpDivisor = 4.0
	requantPow        = 4.0 / 3.0

	// Byte/bit addressing.
	bitsPerByte = 8
	topBitShift = 7

	// IMDCT transform sizes.
	imdctLongN        = 36
	imdctShortN       = 12
	imdctShortHalf    = 6
	imdctShortWindows = 3

	// Scalefactor boundary constants.
	sfShortBands   = 12
	sfSplitBand    = 6
	sfLongBands    = 11
	sfLongTail     = 21
	sfBandIdx32000 = 2

	// Alias reduction butterfly pairs.
	aliasPairs = 8

	// Reservoir initial capacity.
	reservoirCap = 4096

	// MPEG version identifier for MPEG-1 (ISO 11172-3).
	versionMPEG1 = 3

	// LSF scalefactor compress range thresholds (ISO 13818-3 §2.4.3.2).
	lsfCompRange0 = 400
	lsfCompRange1 = 500

	// LSF scalefactor compress decomposition divisors.
	lsfDiv5 = 5
	lsfDiv3 = 3

	// LSF scalefactor compress shift for slen decomposition.
	lsfSlenShift  = 4
	lsfSlenShift2 = 2

	// Scalefactor zone boundaries for LSF short blocks.
	sfZone3  = 3
	sfZone6  = 6
	sfZone9  = 9
	sfZone11 = 11
	sfZone16 = 16
	sfZone21 = 21

	// Intensity stereo escape position (disables IS for this band).
	isEscapePos = 7

	// Mode extension bitmask for MS stereo.
	modeExtMS = 2

	// CRC-16 constants (ISO 11172-3 §2.4.3.1).
	crcInit  = 0xFFFF
	crcPoly  = 0x8005
	crcShift = 8
	crcTop   = 0x8000

	// CRC header field offsets.
	crcByte0 = 4
	crcByte1 = 5

	// Side information sizes for MPEG-2 LSF.
	sideInfoMonoLSF   = 9
	sideInfoStereoLSF = 17

	// Mode extension shift for extracting mode_ext field.
	modeExtShift = 4
	modeExtMask  = 3

	// CRC protection bitmask.
	crcProtMask = 0x01

	// IS ratio table divisor (π/12 tan mapping).
	isRatioDivisor = 12

	// Requantization power table size.
	reqPowTabSize = 8207

	// IS ratio table size.
	isRatioTabSize = 7
)

// Scalefactor band boundaries for MPEG1 (long blocks, per sample rate).
var sfBandLong = [3][23]int{
	{0, 4, 8, 12, 16, 20, 24, 30, 36, 44, 52, 62, 74, 90, 110, 134, 162, 196, 240, 292, 356, 432, 576},
	{0, 4, 8, 12, 16, 20, 24, 30, 36, 42, 50, 60, 72, 88, 106, 128, 156, 190, 230, 276, 330, 384, 576},
	{0, 4, 8, 12, 16, 20, 24, 30, 36, 44, 54, 66, 82, 102, 126, 156, 194, 240, 296, 364, 448, 550, 576},
}

// Scalefactor band boundaries for MPEG1 (short blocks, per sample rate).
var sfBandShort = [3][14]int{
	{0, 4, 8, 12, 16, 22, 30, 40, 52, 66, 84, 106, 136, 192},
	{0, 4, 8, 12, 16, 22, 28, 38, 48, 62, 80, 104, 134, 192},
	{0, 4, 8, 12, 16, 22, 30, 42, 58, 78, 104, 138, 180, 192},
}

// Scalefactor band boundaries for MPEG2/2.5 LSF (long blocks).
var sfBandLongLSF = [3][23]int{
	{0, 6, 12, 18, 24, 30, 36, 44, 54, 66, 80, 96, 116, 140, 168, 200, 238, 284, 336, 396, 464, 522, 576},
	{0, 6, 12, 18, 24, 30, 36, 44, 54, 66, 80, 96, 114, 136, 162, 194, 232, 278, 330, 394, 464, 540, 576},
	{0, 6, 12, 18, 24, 30, 36, 44, 54, 66, 80, 96, 116, 140, 168, 200, 238, 284, 336, 396, 464, 522, 576},
}

// Scalefactor band boundaries for MPEG2/2.5 LSF (short blocks).
var sfBandShortLSF = [3][14]int{
	{0, 4, 8, 12, 18, 24, 32, 42, 56, 74, 100, 132, 174, 192},
	{0, 4, 8, 12, 18, 26, 36, 48, 62, 80, 104, 136, 180, 192},
	{0, 4, 8, 12, 18, 26, 36, 48, 62, 80, 104, 134, 174, 192},
}

func sfBandIndex(sr int) int {
	const (
		rate44100 = 44100
		rate48000 = 48000
		rate32000 = 32000
	)
	switch sr {
	case rate44100:
		return 0
	case rate48000:
		return 1
	case rate32000:
		return sfBandIdx32000
	default:
		return 0
	}
}

// IMDCT window coefficients (precomputed from spec).
var imdctWin [4][imdctLongN]float64

func init() { //nolint:gochecknoinits
	const (
		halfPhase    = 0.5
		winType1Flat = 24
		winType1Fall = 30
	)
	for i := range imdctLongN {
		imdctWin[0][i] = math.Sin(math.Pi / imdctLongN * (float64(i) + halfPhase))
	}
	for i := range imdctLongN {
		switch {
		case i < samplesPerSB:
			imdctWin[1][i] = math.Sin(math.Pi / imdctLongN * (float64(i) + halfPhase))
		case i < winType1Flat:
			imdctWin[1][i] = 1.0
		case i < winType1Fall:
			imdctWin[1][i] = math.Sin(math.Pi / imdctShortN * (float64(i-samplesPerSB) + halfPhase))
		default:
			imdctWin[1][i] = 0.0
		}
	}
	for i := range imdctShortN {
		imdctWin[2][i] = math.Sin(math.Pi / imdctShortN * (float64(i) + halfPhase))
	}
	for i := range imdctLongN {
		switch {
		case i < imdctShortHalf:
			imdctWin[3][i] = 0.0
		case i < imdctShortN:
			imdctWin[3][i] = math.Sin(math.Pi / imdctShortN * (float64(i-imdctShortHalf) + halfPhase))
		case i < samplesPerSB:
			imdctWin[3][i] = 1.0
		default:
			imdctWin[3][i] = math.Sin(math.Pi / imdctLongN * (float64(i) + halfPhase))
		}
	}
}

// Alias reduction butterfly coefficients (8 pairs, from ISO spec).
var csCoeff [aliasPairs]float64
var caCoeff [aliasPairs]float64

func init() { //nolint:gochecknoinits
	ci := [aliasPairs]float64{
		-0.6, -0.535, -0.33, -0.185,
		-0.095, -0.041, -0.0142, -0.0037,
	}
	for i := range aliasPairs {
		denom := math.Sqrt(1.0 + ci[i]*ci[i]) //nolint:gosec
		csCoeff[i] = 1.0 / denom
		caCoeff[i] = ci[i] / denom //nolint:gosec
	}
}

// Scalefactor length table (slen1, slen2 from sfcompress index).
var sfLenTable = [16][2]int{
	{0, 0}, {0, 1}, {0, 2}, {0, 3},
	{3, 0}, {1, 1}, {1, 2}, {1, 3},
	{2, 1}, {2, 2}, {2, 3}, {3, 1},
	{3, 2}, {3, 3}, {4, 2}, {4, 3},
}

// Pretab values for requantization (from ISO spec).
var pretab = [22]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 3, 3, 3, 2, 0}

// Precomputed IMDCT cosine tables for long (36-point) and short (12-point) transforms.
var imdctCosLong [imdctLongN][samplesPerSB]float64
var imdctCosShort [imdctShortN][imdctShortHalf]float64

func init() { //nolint:gochecknoinits
	for i := range imdctLongN {
		for k := range samplesPerSB {
			imdctCosLong[i][k] = math.Cos(math.Pi / 72 * (2*float64(i) + 1 + samplesPerSB) * (2*float64(k) + 1))
		}
	}
	for i := range imdctShortN {
		for k := range imdctShortHalf {
			imdctCosShort[i][k] = math.Cos(math.Pi / 24 * (2*float64(i) + 1 + imdctShortHalf) * (2*float64(k) + 1))
		}
	}
}

// Precomputed synthesis filterbank cosine table.
var synthCos [64][32]float64

func init() { //nolint:gochecknoinits
	for i := range 64 {
		for k := range 32 {
			synthCos[i][k] = math.Cos(math.Pi / 64 * float64(2*i+1+subbands) * float64(2*k+1))
		}
	}
}

// Synthesis window coefficients (Table B.3 from ISO 11172-3).
var synthWindow = [512]float64{
	0.000000000, -0.000015259, -0.000015259, -0.000015259,
	-0.000015259, -0.000015259, -0.000015259, -0.000030518,
	-0.000030518, -0.000030518, -0.000030518, -0.000045776,
	-0.000045776, -0.000061035, -0.000061035, -0.000076294,
	-0.000076294, -0.000091553, -0.000106812, -0.000106812,
	-0.000122070, -0.000137329, -0.000152588, -0.000167847,
	-0.000198364, -0.000213623, -0.000244141, -0.000259399,
	-0.000289917, -0.000320435, -0.000366211, -0.000396729,
	-0.000442505, -0.000473022, -0.000534058, -0.000579834,
	-0.000625610, -0.000686646, -0.000747681, -0.000808716,
	-0.000885010, -0.000961304, -0.001037598, -0.001113892,
	-0.001205444, -0.001296997, -0.001388550, -0.001480103,
	-0.001586914, -0.001693726, -0.001785278, -0.001907349,
	-0.002014160, -0.002120972, -0.002243042, -0.002349854,
	-0.002456665, -0.002578735, -0.002685547, -0.002792358,
	-0.002899170, -0.002990723, -0.003082275, -0.003173828,
	0.003250122, 0.003326416, 0.003387451, 0.003433228,
	0.003463745, 0.003479004, 0.003479004, 0.003463745,
	0.003417969, 0.003372192, 0.003280640, 0.003173828,
	0.003051758, 0.002883911, 0.002700806, 0.002487183,
	0.002227783, 0.001937866, 0.001617432, 0.001266479,
	0.000869751, 0.000442505, -0.000030518, -0.000549316,
	-0.001098633, -0.001693726, -0.002334595, -0.003005981,
	-0.003723145, -0.004486084, -0.005294800, -0.006118774,
	-0.007003784, -0.007919312, -0.008865356, -0.009841919,
	-0.010848999, -0.011886597, -0.012939453, -0.014022827,
	-0.015121460, -0.016235352, -0.017349243, -0.018463135,
	-0.019577026, -0.020690918, -0.021789551, -0.022857666,
	-0.023910522, -0.024932861, -0.025909424, -0.026840210,
	-0.027725220, -0.028533936, -0.029281616, -0.029937744,
	-0.030532837, -0.031005859, -0.031387329, -0.031661987,
	-0.031814575, -0.031845093, -0.031738281, -0.031478882,
	0.031082153, 0.030517578, 0.029785156, 0.028884888,
	0.027801514, 0.026535034, 0.025085449, 0.023422241,
	0.021575928, 0.019531250, 0.017257690, 0.014801025,
	0.012115479, 0.009231567, 0.006134033, 0.002822876,
	-0.000686646, -0.004394531, -0.008316040, -0.012420654,
	-0.016708374, -0.021179199, -0.025817871, -0.030609131,
	-0.035552979, -0.040634155, -0.045837402, -0.051132202,
	-0.056533813, -0.061996460, -0.067520142, -0.073059082,
	-0.078628540, -0.084182739, -0.089706421, -0.095169067,
	-0.100540161, -0.105819702, -0.110946655, -0.115921021,
	-0.120697021, -0.125259399, -0.129562378, -0.133590698,
	-0.137298584, -0.140670776, -0.143676758, -0.146255493,
	-0.148422241, -0.150115967, -0.151306152, -0.151962280,
	-0.152069092, -0.151596069, -0.150497437, -0.148773193,
	0.146362305, 0.143264771, 0.139450073, 0.134887695,
	0.129577637, 0.123474121, 0.116577148, 0.108856201,
	0.100311279, 0.090927124, 0.080688477, 0.069595337,
	0.057617188, 0.044784546, 0.031082153, 0.016510010,
	0.001068115, -0.015228271, -0.032379150, -0.050354004,
	-0.069168091, -0.088775635, -0.109161377, -0.130310059,
	-0.152206421, -0.174789429, -0.198059082, -0.221984863,
	-0.246505737, -0.271591187, -0.297210693, -0.323318481,
	-0.349868774, -0.376800537, -0.404083252, -0.431655884,
	-0.459472656, -0.487472534, -0.515609741, -0.543823242,
	-0.572036743, -0.600219727, -0.628295898, -0.656219482,
	-0.683914185, -0.711318970, -0.738372803, -0.765029907,
	-0.791213989, -0.816864014, -0.841949463, -0.866363525,
	-0.890090942, -0.913055420, -0.935195923, -0.956481934,
	-0.976852417, -0.996246338, -1.014617920, -1.031936646,
	-1.048156738, -1.063217163, -1.077117920, -1.089782715,
	-1.101211548, -1.111373901, -1.120223999, -1.127746582,
	-1.133926392, -1.138763428, -1.142211914, -1.144287109,
}

var reqPowTab [reqPowTabSize]float64
var isRatioTable [isRatioTabSize]float64

func init() { //nolint:gochecknoinits
	for i := range reqPowTab {
		reqPowTab[i] = math.Pow(float64(i), requantPow)
	}
	for i := range isRatioTabSize {
		isRatioTable[i] = math.Tan(float64(i) * math.Pi / isRatioDivisor)
	}
}
