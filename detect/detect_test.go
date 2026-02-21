package detect_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode"
	"github.com/gophics/ravenporter/ir"
)

func TestDetectGLBMagic(t *testing.T) {
	data := []byte("glTF\x02\x00\x00\x00\x00\x00\x00\x00")
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.True(t, got == ir.FormatGLB || got == ir.FormatGLTF, "expected glb or gltf")
}

func TestDetectFBXMagic(t *testing.T) {
	data := []byte("Kaydara FBX Binary  \x00\x1a\x00\x00\x00")
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatFBX, got)
}

func TestDetect3MF(t *testing.T) {
	data := append([]byte{0x50, 0x4B, 0x03, 0x04, 0, 0, 0, 0}, []byte("...[Content_Types].xml...")...)
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.Format3MF, got)
}

func TestDetectPLY(t *testing.T) {
	data := []byte("ply\nformat ascii 1.0\n")
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatPLY, got)
}

func TestDetectBVH(t *testing.T) {
	data := []byte("HIERARCHY\nROOT Hips\n")
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatBVH, got)
}

func TestDetectSTL(t *testing.T) {
	data := []byte("solid cube\nfacet normal 0 0 1\n")
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatSTL, got)
}

func TestDetectOBJ(t *testing.T) {
	data := []byte("# OBJ file\nv 0.0 0.0 0.0\nv 1.0 0.0 0.0\nf 1 2 3\n")
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatOBJ, got)
}

func TestDetectGLTFJSON(t *testing.T) {
	data := []byte(`{"asset":{"version":"2.0"},"scene":0}`)
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), ".gltf")
	require.NoError(t, err)
	assert.True(t, got == ir.FormatGLB || got == ir.FormatGLTF, "expected glb or gltf")
}

func TestDetectCOLLADA(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><COLLADA xmlns="http://www.collada.org/2005/11/COLLADASchema">`)
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatDAE, got)
}

func TestDetectUnknown(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatUnknown, got)
}

func TestDetectImageFormats(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		expect ir.FormatID
	}{
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, ir.FormatPNG},
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, ir.FormatJPEG},
		{"BMP", []byte{0x42, 0x4D, 0x00, 0x00, 0x00, 0x00}, ir.FormatBMP},
		{"HDR", []byte("#?RADIANCE\n"), ir.FormatHDR},
		{"DDS", []byte{0x44, 0x44, 0x53, 0x20, 0x7C, 0x00}, ir.FormatDDS},
		{"EXR", []byte{0x76, 0x2F, 0x31, 0x01, 0x02, 0x00}, ir.FormatEXR},
		{"PSD", []byte{0x38, 0x42, 0x50, 0x53, 0x00, 0x01}, ir.FormatPSD},
		{"KTX", []byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x31}, ir.FormatKTX},
		{"TIFF LE", []byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00}, ir.FormatTIFF},
		{"TIFF BE", []byte{0x4D, 0x4D, 0x00, 0x2A, 0x00, 0x08}, ir.FormatTIFF},
		{"WebP", append([]byte("RIFF\x00\x00\x00\x00WEBP"), make([]byte, 4)...), ir.FormatWebP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decode.DefaultRegistry().Detect(bytes.NewReader(tt.data), "")
			require.NoError(t, err)
			assert.Equal(t, tt.expect, got, "format mismatch for %s", tt.name)
		})
	}
}

func TestDetectAudioFormats(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		expect ir.FormatID
	}{
		{"WAV", append([]byte("RIFF\x00\x00\x00\x00WAVE"), make([]byte, 4)...), ir.FormatWAV},
		{"AIFF", append([]byte("FORM\x00\x00\x00\x00AIFF"), make([]byte, 4)...), ir.FormatAIFF},
		{"FLAC", []byte("fLaC\x00\x00\x00\x22"), ir.FormatFLAC},
		{"MP3 ID3", []byte{0x49, 0x44, 0x33, 0x03, 0x00}, ir.FormatMP3},
		{"OGG Vorbis", append([]byte("OggS\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01\x1e\x01vorbis"), make([]byte, 20)...), ir.FormatOGG},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decode.DefaultRegistry().Detect(bytes.NewReader(tt.data), "")
			require.NoError(t, err)
			assert.Equal(t, tt.expect, got, "format mismatch for %s", tt.name)
		})
	}
}

func TestDetectFontFormats(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		expect ir.FormatID
	}{
		{"TTF", []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x0A}, ir.FormatTTF},
		{"OTF", []byte{0x4F, 0x54, 0x54, 0x4F, 0x00, 0x0A}, ir.FormatOTF},
		{"WOFF", []byte{0x77, 0x4F, 0x46, 0x46, 0x00, 0x01}, ir.FormatWOFF},
		{"WOFF2", []byte{0x77, 0x4F, 0x46, 0x32, 0x00, 0x01}, ir.FormatWOFF2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decode.DefaultRegistry().Detect(bytes.NewReader(tt.data), "")
			require.NoError(t, err)
			assert.Equal(t, tt.expect, got, "format mismatch for %s", tt.name)
		})
	}
}

func TestDetect3DS(t *testing.T) {
	// 0x4D4D main chunk with plausible size.
	data := []byte{0x4D, 0x4D, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00}
	got, err := decode.DefaultRegistry().Detect(bytes.NewReader(data), "")
	require.NoError(t, err)
	assert.Equal(t, ir.Format3DS, got)
}

func TestRegistryRoundTrip(t *testing.T) {
	reg := detect.NewRegistry()
	_, ok := reg.Lookup(ir.FormatGLB)
	assert.False(t, ok)
	assert.Empty(t, reg.Formats())
}

func TestRegistryExtensionsSortedUnique(t *testing.T) {
	reg := detect.NewRegistry(
		detect.Registration{Format: ir.FormatGLB, Decoder: &stubDecoder{extensions: []string{".glb", ".gltf"}}},
		detect.Registration{Format: ir.FormatGLTF, Decoder: &stubDecoder{extensions: []string{".GLTF", ".vrp"}}},
	)

	assert.Equal(t, []string{".glb", ".gltf", ".vrp"}, reg.Extensions())
}

func TestDecodeOptionsSanitize(t *testing.T) {
	opts := detect.DecodeOptions{}
	opts.Sanitize()

	require.NotNil(t, opts.Context)
	require.NotNil(t, opts.FS)
	assert.Positive(t, opts.MaxVertices)
	assert.Positive(t, opts.MaxImagePixels)
	assert.Positive(t, opts.MaxAudioSamples)
}

func TestRegistryDetectByExtensionFallback(t *testing.T) {
	reg := detect.NewRegistry(
		detect.Registration{Format: ir.FormatOBJ, Decoder: &stubDecoder{extensions: []string{".obj"}}},
	)

	format, err := reg.Detect(bytes.NewReader([]byte("not a known magic")), "mesh.obj")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatOBJ, format)
}

func TestRegistryDetectReturnsSeekErrors(t *testing.T) {
	reg := detect.NewRegistry(
		detect.Registration{Format: ir.FormatOBJ, Decoder: &stubDecoder{extensions: []string{".obj"}}},
	)

	format, err := reg.Detect(seekErrorReader{err: errors.New("seek failed")}, "mesh.obj")
	require.Error(t, err)
	assert.Equal(t, ir.FormatUnknown, format)
	assert.Contains(t, err.Error(), "seek failed")
}

func TestRegistryDetectUsesRegistrationOrderForProbes(t *testing.T) {
	reg := detect.NewRegistry(
		detect.Registration{
			Format: ir.FormatOBJ,
			Decoder: &stubDecoder{
				extensions: []string{".mesh"},
				probe: func(io.ReadSeeker) bool {
					return true
				},
			},
		},
		detect.Registration{
			Format: ir.FormatPLY,
			Decoder: &stubDecoder{
				extensions: []string{".mesh"},
				probe: func(io.ReadSeeker) bool {
					return true
				},
			},
		},
	)

	format, err := reg.Detect(bytes.NewReader([]byte("mesh")), "asset.mesh")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatOBJ, format)
}

func TestRegistryDetectUsesRegistrationOrderForExtensions(t *testing.T) {
	reg := detect.NewRegistry(
		detect.Registration{Format: ir.FormatOBJ, Decoder: &stubDecoder{extensions: []string{".mesh"}}},
		detect.Registration{Format: ir.FormatPLY, Decoder: &stubDecoder{extensions: []string{".mesh"}}},
	)

	format, err := reg.Detect(bytes.NewReader([]byte("unknown")), "asset.mesh")
	require.NoError(t, err)
	assert.Equal(t, ir.FormatOBJ, format)
}

func TestRegistryFormatsSorted(t *testing.T) {
	reg := detect.NewRegistry()
	reg.Register(ir.FormatOBJ, &stubDecoder{extensions: []string{".obj"}})
	reg.RegisterAll(
		detect.Registration{Format: ir.FormatPNG, Decoder: &stubDecoder{extensions: []string{".png"}}},
		detect.Registration{Format: ir.FormatGLTF, Decoder: &stubDecoder{extensions: []string{".gltf"}}},
	)

	assert.Equal(t, []ir.FormatID{ir.FormatGLTF, ir.FormatOBJ, ir.FormatPNG}, reg.Formats())
	assert.True(t, reg.SupportsExtension(".OBJ"))
	assert.False(t, reg.SupportsExtension(".abc"))
}

type stubDecoder struct {
	extensions []string
	probe      func(io.ReadSeeker) bool
}

func (d *stubDecoder) Probe(r io.ReadSeeker) bool {
	if d.probe != nil {
		return d.probe(r)
	}
	return false
}

func (d *stubDecoder) Decode(_ detect.ReadSeekerAt, _ detect.DecodeOptions) (*ir.Asset, error) {
	return nil, nil
}

func (d *stubDecoder) Extensions() []string { return d.extensions }

func (d *stubDecoder) FormatName() string { return "stub" }

type seekErrorReader struct {
	err error
}

func (r seekErrorReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (r seekErrorReader) Seek(_ int64, _ int) (int64, error) {
	return 0, r.err
}
