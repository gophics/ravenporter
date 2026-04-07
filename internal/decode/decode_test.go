package decode

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
)

func TestRegistrationsExposeBuiltInMetadata(t *testing.T) {
	expected := map[ir.FormatID]struct {
		name       string
		extensions []string
	}{
		ir.FormatGLTF:    {name: "glTF 2.0", extensions: []string{".gltf", ".glb"}},
		ir.FormatGLB:     {name: "glTF 2.0", extensions: []string{".gltf", ".glb"}},
		ir.FormatFBX:     {name: "FBX", extensions: []string{".fbx"}},
		ir.FormatOBJ:     {name: "OBJ", extensions: []string{".obj"}},
		ir.FormatDAE:     {name: "COLLADA", extensions: []string{".dae"}},
		ir.FormatSTL:     {name: "STL", extensions: []string{".stl"}},
		ir.FormatPLY:     {name: "PLY", extensions: []string{".ply"}},
		ir.Format3MF:     {name: "3MF", extensions: []string{".3mf"}},
		ir.FormatBVH:     {name: "BVH", extensions: []string{".bvh"}},
		ir.Format3DS:     {name: "3D Studio", extensions: []string{".3ds"}},
		ir.FormatUSD:     {name: "USDA", extensions: []string{".usda", ".usd", ".usdc", ".usdz"}},
		ir.FormatAlembic: {name: "Alembic", extensions: []string{".abc"}},
		ir.FormatPNG:     {name: "PNG", extensions: []string{".png"}},
		ir.FormatJPEG:    {name: "JPEG", extensions: []string{".jpeg", ".jpg"}},
		ir.FormatBMP:     {name: "BMP", extensions: []string{".bmp"}},
		ir.FormatTGA:     {name: "TGA", extensions: []string{".tga"}},
		ir.FormatHDR:     {name: "HDR", extensions: []string{".hdr"}},
		ir.FormatWebP:    {name: "WebP", extensions: []string{".webp"}},
		ir.FormatTIFF:    {name: "TIFF", extensions: []string{".tiff", ".tif"}},
		ir.FormatEXR:     {name: "EXR", extensions: []string{".exr"}},
		ir.FormatDDS:     {name: "DDS", extensions: []string{".dds"}},
		ir.FormatKTX:     {name: "KTX", extensions: []string{".ktx", ".ktx2"}},
		ir.FormatPSD:     {name: "PSD", extensions: []string{".psd", ".psb"}},
		ir.FormatWAV:     {name: "WAV", extensions: []string{".wav"}},
		ir.FormatOGG:     {name: "OGG", extensions: []string{".ogg", ".oga"}},
		ir.FormatMP3:     {name: "MP3", extensions: []string{".mp3"}},
		ir.FormatFLAC:    {name: "FLAC", extensions: []string{".flac"}},
		ir.FormatAIFF:    {name: "AIFF", extensions: []string{".aiff", ".aif"}},
		ir.FormatOpus:    {name: "Opus", extensions: []string{".opus"}},
		ir.FormatTTF:     {name: "TTF", extensions: []string{".ttf"}},
		ir.FormatOTF:     {name: "OTF", extensions: []string{".otf"}},
		ir.FormatWOFF:    {name: "WOFF", extensions: []string{".woff"}},
		ir.FormatWOFF2:   {name: "WOFF2", extensions: []string{".woff2"}},
	}

	registrations := Registrations()
	require.Len(t, registrations, len(expected))

	formats := make([]ir.FormatID, 0, len(registrations))
	seen := make(map[ir.FormatID]struct{}, len(registrations))
	exts := make(map[string]struct{})
	for _, registration := range registrations {
		want, ok := expected[registration.Format]
		require.Truef(t, ok, "unexpected decoder registration for %q", registration.Format)
		require.NotNil(t, registration.Decoder)
		_, duplicate := seen[registration.Format]
		require.Falsef(t, duplicate, "duplicate decoder registration for %q", registration.Format)

		seen[registration.Format] = struct{}{}
		formats = append(formats, registration.Format)
		assert.Equal(t, want.name, registration.Decoder.FormatName())
		assert.Equal(t, want.extensions, registration.Decoder.Extensions())
		for _, ext := range want.extensions {
			exts[ext] = struct{}{}
		}
	}

	slices.Sort(formats)

	expectedFormats := make([]ir.FormatID, 0, len(expected))
	for format := range expected {
		expectedFormats = append(expectedFormats, format)
	}
	slices.Sort(expectedFormats)
	assert.Equal(t, expectedFormats, formats)

	expectedExts := make([]string, 0, len(exts))
	for ext := range exts {
		expectedExts = append(expectedExts, ext)
	}
	slices.Sort(expectedExts)
	assert.Equal(t, expectedExts, NewRegistry().Extensions())
}

func TestNewRegistryIsIndependent(t *testing.T) {
	defaultFormats := DefaultRegistry().Formats()
	registry := NewRegistry()

	require.NotNil(t, registry)
	assert.NotSame(t, DefaultRegistry(), registry)
	assert.Equal(t, defaultFormats, registry.Formats())
}
