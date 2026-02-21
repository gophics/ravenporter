package detect

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gophics/ravenporter/ir"
)

func TestMatchMagic(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want ir.FormatID
	}{
		// Audio.
		{"WAV", []byte("RIFF\x00\x00\x00\x00WAVE"), ir.FormatWAV},
		{"AIFF", []byte("FORM\x00\x00\x00\x00AIFF"), ir.FormatAIFF},
		{"AIFC", []byte("FORM\x00\x00\x00\x00AIFC"), ir.FormatAIFF},
		{"OGG", []byte("OggS\x00\x00\x00\x00\x00\x00\x00\x00"), ir.FormatOGG},
		{"Opus", append([]byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01"), []byte("OpusHead")...), ir.FormatOpus},
		{"FLAC", []byte("fLaC"), ir.FormatFLAC},
		{"MP3_ID3", []byte("ID3\x04"), ir.FormatMP3},
		{"MP3_Sync", []byte{0xFF, 0xFB, 0x90, 0x00}, ir.FormatMP3},

		// Image.
		{"PNG", []byte{0x89, 'P', 'N', 'G'}, ir.FormatPNG},
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0}, ir.FormatJPEG},
		{"BMP", []byte("BM\x00\x00"), ir.FormatBMP},
		{"DDS", []byte("DDS \x00"), ir.FormatDDS},
		{"EXR", []byte{0x76, 0x2F, 0x31, 0x01}, ir.FormatEXR},
		{"KTX", []byte{0xAB, 0x4B, 0x54, 0x58}, ir.FormatKTX},
		{"PSD", []byte("8BPS"), ir.FormatPSD},
		{"HDR_Radiance", []byte("#?RADIANCE\n"), ir.FormatHDR},
		{"HDR_RGBE", []byte("#?RGBE\n"), ir.FormatHDR},
		{"TIFF_LE", []byte{0x49, 0x49, 0x2A, 0x00}, ir.FormatTIFF},
		{"TIFF_BE", []byte{0x4D, 0x4D, 0x00, 0x2A}, ir.FormatTIFF},
		{"WebP", []byte("RIFF\x00\x00\x00\x00WEBP"), ir.FormatWebP},

		// Font.
		{"TTF", []byte{0x00, 0x01, 0x00, 0x00}, ir.FormatTTF},
		{"OTF", []byte("OTTO"), ir.FormatOTF},
		{"WOFF", []byte("wOFF"), ir.FormatWOFF},
		{"WOFF2", []byte("wOF2"), ir.FormatWOFF2},

		// Model.
		{"GLB", []byte("glTF\x02\x00\x00\x00"), ir.FormatGLB},
		{"FBX", []byte("Kaydara FBX Binary  \x00"), ir.FormatFBX},
		{"3MF", []byte{0x50, 0x4B, 0x03, 0x04}, ir.Format3MF},
		{"USDC", []byte("PXR-USDC"), ir.FormatUSD},
		{"USDA", []byte("#usda 1.0"), ir.FormatUSD},
		{"BVH", []byte("HIERARCHY\n"), ir.FormatBVH},
		{"PLY", []byte("ply\n"), ir.FormatPLY},
		{"Alembic", []byte("Ogawa\x00"), ir.FormatAlembic},

		// Disambiguation.
		{"RIFF_Unknown_Sub", []byte("RIFF\x00\x00\x00\x00AVI "), ir.FormatUnknown},
		{"FORM_Unknown_Sub", []byte("FORM\x00\x00\x00\x00XXXX"), ir.FormatUnknown},

		// Unknown / too short.
		{"Empty", []byte{}, ir.FormatUnknown},
		{"Short", []byte{0x01, 0x02}, ir.FormatUnknown},
		{"Garbage", []byte("ZZZZ"), ir.FormatUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchMagic(tc.buf)
			assert.Equal(t, tc.want, got)
		})
	}
}
