package detect

import (
	"bytes"

	"github.com/gophics/ravenporter/ir"
)

const dispatchBufSize = 64

// Magic prefixes for format detection.
var ( //nolint:gochecknoglobals // static dispatch table
	// Audio.
	magicRIFF    = []byte("RIFF")
	magicFORM    = []byte("FORM")
	magicOggS    = []byte("OggS")
	magicFLAC    = []byte("fLaC")
	magicID3     = []byte("ID3")
	magicOpus    = []byte("OpusHead")
	subWAVE      = []byte("WAVE")
	subWEBP      = []byte("WEBP")
	subAIFF      = []byte("AIFF")
	subAIFC      = []byte("AIFC")
	magicSyncMP3 = [2]byte{0xFF, 0xE0}

	// Image.
	magicPNG    = []byte{0x89, 'P', 'N', 'G'}
	magicJPEG   = []byte{0xFF, 0xD8, 0xFF}
	magicBMP    = []byte("BM")
	magicDDS    = []byte("DDS ")
	magicEXR    = []byte{0x76, 0x2F, 0x31, 0x01}
	magicKTX    = []byte{0xAB, 0x4B, 0x54, 0x58}
	magicPSD    = []byte("8BPS")
	magicHDR1   = []byte("#?RADIANCE")
	magicHDR2   = []byte("#?RGBE")
	magicTIFFLE = []byte{0x49, 0x49, 0x2A, 0x00}
	magicTIFFBE = []byte{0x4D, 0x4D, 0x00, 0x2A}

	// Font.
	magicTTF   = []byte{0x00, 0x01, 0x00, 0x00}
	magicOTF   = []byte("OTTO")
	magicWOFF  = []byte("wOFF")
	magicWOFF2 = []byte("wOF2")

	// Model.
	magicGLB   = []byte("glTF")
	magicFBX   = []byte("Kaydara FBX")
	magicPK    = []byte{0x50, 0x4B, 0x03, 0x04}
	magicUSDA  = []byte("#usda")
	magicUSDC  = []byte("PXR-USDC")
	magicBVH   = []byte("HIERARCHY")
	magicPLY   = []byte("ply")
	magicOgawa = []byte("Ogawa")
)

const riffSubOff = 8

// matchMagic identifies a format from raw header bytes without seeking.
func matchMagic(buf []byte) ir.FormatID {
	if id := matchAudio(buf); id != ir.FormatUnknown {
		return id
	}
	if id := matchImage(buf); id != ir.FormatUnknown {
		return id
	}
	if id := matchFont(buf); id != ir.FormatUnknown {
		return id
	}
	return matchModel(buf)
}

func matchAudio(buf []byte) ir.FormatID {
	switch {
	case hasPrefix(buf, magicRIFF) && hasSub(buf, subWAVE):
		return ir.FormatWAV
	case hasPrefix(buf, magicFORM) && (hasSub(buf, subAIFF) || hasSub(buf, subAIFC)):
		return ir.FormatAIFF
	case hasPrefix(buf, magicOggS):
		if bytes.Contains(buf, magicOpus) {
			return ir.FormatOpus
		}
		return ir.FormatOGG
	case hasPrefix(buf, magicFLAC):
		return ir.FormatFLAC
	case hasPrefix(buf, magicID3):
		return ir.FormatMP3
	case len(buf) >= 2 && buf[0] == magicSyncMP3[0] && buf[1]&magicSyncMP3[1] == magicSyncMP3[1]:
		return ir.FormatMP3
	}
	return ir.FormatUnknown
}

func matchImage(buf []byte) ir.FormatID {
	switch {
	case hasPrefix(buf, magicPNG):
		return ir.FormatPNG
	case hasPrefix(buf, magicJPEG):
		return ir.FormatJPEG
	case hasPrefix(buf, magicBMP):
		return ir.FormatBMP
	case hasPrefix(buf, magicDDS):
		return ir.FormatDDS
	case hasPrefix(buf, magicEXR):
		return ir.FormatEXR
	case hasPrefix(buf, magicKTX):
		return ir.FormatKTX
	case hasPrefix(buf, magicPSD):
		return ir.FormatPSD
	case hasPrefix(buf, magicHDR1), hasPrefix(buf, magicHDR2):
		return ir.FormatHDR
	case hasPrefix(buf, magicTIFFLE), hasPrefix(buf, magicTIFFBE):
		return ir.FormatTIFF
	case hasPrefix(buf, magicRIFF) && hasSub(buf, subWEBP):
		return ir.FormatWebP
	}
	return ir.FormatUnknown
}

func matchFont(buf []byte) ir.FormatID {
	switch {
	case hasPrefix(buf, magicTTF):
		return ir.FormatTTF
	case hasPrefix(buf, magicOTF):
		return ir.FormatOTF
	case hasPrefix(buf, magicWOFF):
		return ir.FormatWOFF
	case hasPrefix(buf, magicWOFF2):
		return ir.FormatWOFF2
	}
	return ir.FormatUnknown
}

func matchModel(buf []byte) ir.FormatID {
	switch {
	case hasPrefix(buf, magicGLB):
		return ir.FormatGLB
	case hasPrefix(buf, magicFBX):
		return ir.FormatFBX
	case hasPrefix(buf, magicPK):
		return ir.Format3MF
	case hasPrefix(buf, magicUSDC):
		return ir.FormatUSD
	case hasPrefix(buf, magicUSDA):
		return ir.FormatUSD
	case hasPrefix(buf, magicBVH):
		return ir.FormatBVH
	case hasPrefix(buf, magicPLY):
		return ir.FormatPLY
	case hasPrefix(buf, magicOgawa):
		return ir.FormatAlembic
	}
	return ir.FormatUnknown
}

func hasPrefix(buf, prefix []byte) bool {
	return bytes.HasPrefix(buf, prefix)
}

func hasSub(buf, sub []byte) bool {
	return len(buf) >= riffSubOff+len(sub) &&
		bytes.Equal(buf[riffSubOff:riffSubOff+len(sub)], sub)
}
