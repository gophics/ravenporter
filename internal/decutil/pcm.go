package decutil

import "github.com/gophics/ravenporter/internal/binread"

// PCM sample decoding constants shared across WAV and AIFF decoders.
const (
	BitsPerByte = 8
	Bytes8      = 1
	Bytes16     = 2
	Bytes24     = 3
	Bytes32     = 4
	Bytes64     = 8

	MaxInt8    = 128.0
	MaxInt16   = 32768.0
	MaxInt24   = 8388608.0
	MaxInt32   = 2147483648.0
	Max8Offset = 128

	Shift8     = 8
	Shift16    = 16
	SignBit24  = 0x800000
	SignMask24 = ^0xFFFFFF
)

// Decode8Bit converts unsigned 8-bit PCM samples to float32.
func Decode8Bit(raw []byte, dst []float32) {
	for i, b := range raw {
		dst[i] = (float32(b) - Max8Offset) / MaxInt8
	}
}

// Decode16LE converts signed 16-bit little-endian PCM samples to float32.
func Decode16LE(raw []byte, dst []float32) {
	for i := range len(dst) {
		off := i * Bytes16
		dst[i] = float32(binread.ReadI16LE(raw[off:])) / MaxInt16
	}
}

// Decode24LE converts signed 24-bit little-endian PCM samples to float32.
func Decode24LE(raw []byte, dst []float32) {
	for i := range len(dst) {
		off := i * Bytes24
		v := int32(raw[off]) | int32(raw[off+1])<<Shift8 | int32(raw[off+2])<<Shift16
		if v&SignBit24 != 0 {
			v |= SignMask24
		}
		dst[i] = float32(v) / MaxInt24
	}
}

// Decode32LE converts signed 32-bit little-endian PCM samples to float32.
func Decode32LE(raw []byte, dst []float32) {
	for i := range len(dst) {
		off := i * Bytes32
		dst[i] = float32(binread.ReadI32LE(raw[off:])) / MaxInt32
	}
}

// DecodeIEEEFloat converts little-endian IEEE 754 float samples.
func DecodeIEEEFloat(raw []byte, dst []float32, bytesPerSample int) {
	for i := range len(dst) {
		off := i * bytesPerSample
		switch bytesPerSample {
		case Bytes32:
			dst[i] = binread.ReadF32LE(raw[off:])
		case Bytes64:
			dst[i] = float32(binread.ReadF64LE(raw[off:]))
		}
	}
}

// DecodeIEEEFloatBE converts big-endian IEEE 754 float samples.
func DecodeIEEEFloatBE(raw []byte, dst []float32, bytesPerSample int) {
	for i := range len(dst) {
		off := i * bytesPerSample
		switch bytesPerSample {
		case Bytes32:
			dst[i] = binread.ReadF32BE(raw[off:])
		case Bytes64:
			dst[i] = float32(binread.ReadF64BE(raw[off:]))
		}
	}
}
