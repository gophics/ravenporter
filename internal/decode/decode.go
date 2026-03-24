// Package decode provides the built-in decoder catalog used by RavenPorter.
package decode

import (
	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/audio/aiff"
	"github.com/gophics/ravenporter/internal/decode/audio/flac"
	"github.com/gophics/ravenporter/internal/decode/audio/mp3"
	"github.com/gophics/ravenporter/internal/decode/audio/ogg"
	"github.com/gophics/ravenporter/internal/decode/audio/opus"
	"github.com/gophics/ravenporter/internal/decode/audio/wav"
	"github.com/gophics/ravenporter/internal/decode/font/otf"
	"github.com/gophics/ravenporter/internal/decode/font/ttf"
	"github.com/gophics/ravenporter/internal/decode/font/woff"
	"github.com/gophics/ravenporter/internal/decode/font/woff2"
	"github.com/gophics/ravenporter/internal/decode/image/bmp"
	"github.com/gophics/ravenporter/internal/decode/image/dds"
	"github.com/gophics/ravenporter/internal/decode/image/exr"
	"github.com/gophics/ravenporter/internal/decode/image/hdr"
	"github.com/gophics/ravenporter/internal/decode/image/jpeg"
	"github.com/gophics/ravenporter/internal/decode/image/ktx"
	"github.com/gophics/ravenporter/internal/decode/image/png"
	"github.com/gophics/ravenporter/internal/decode/image/psd"
	"github.com/gophics/ravenporter/internal/decode/image/tga"
	"github.com/gophics/ravenporter/internal/decode/image/tiff"
	"github.com/gophics/ravenporter/internal/decode/image/webp"
	"github.com/gophics/ravenporter/internal/decode/model/abc"
	"github.com/gophics/ravenporter/internal/decode/model/bvh"
	"github.com/gophics/ravenporter/internal/decode/model/dae"
	"github.com/gophics/ravenporter/internal/decode/model/fbx"
	"github.com/gophics/ravenporter/internal/decode/model/gltf"
	"github.com/gophics/ravenporter/internal/decode/model/obj"
	"github.com/gophics/ravenporter/internal/decode/model/ply"
	"github.com/gophics/ravenporter/internal/decode/model/stl"
	"github.com/gophics/ravenporter/internal/decode/model/tds"
	"github.com/gophics/ravenporter/internal/decode/model/threemf"
	"github.com/gophics/ravenporter/internal/decode/model/usda"
)

var defaultRegistry = detect.NewRegistry(Registrations()...)

// Registrations returns the built-in decoder catalog.
func Registrations() []detect.Registration {
	var registrations []detect.Registration
	registrations = append(registrations, aiff.Registrations()...)
	registrations = append(registrations, flac.Registrations()...)
	registrations = append(registrations, mp3.Registrations()...)
	registrations = append(registrations, ogg.Registrations()...)
	registrations = append(registrations, opus.Registrations()...)
	registrations = append(registrations, wav.Registrations()...)
	registrations = append(registrations, otf.Registrations()...)
	registrations = append(registrations, ttf.Registrations()...)
	registrations = append(registrations, woff.Registrations()...)
	registrations = append(registrations, woff2.Registrations()...)
	registrations = append(registrations, bmp.Registrations()...)
	registrations = append(registrations, dds.Registrations()...)
	registrations = append(registrations, exr.Registrations()...)
	registrations = append(registrations, hdr.Registrations()...)
	registrations = append(registrations, jpeg.Registrations()...)
	registrations = append(registrations, ktx.Registrations()...)
	registrations = append(registrations, png.Registrations()...)
	registrations = append(registrations, psd.Registrations()...)
	registrations = append(registrations, tga.Registrations()...)
	registrations = append(registrations, tiff.Registrations()...)
	registrations = append(registrations, webp.Registrations()...)
	registrations = append(registrations, abc.Registrations()...)
	registrations = append(registrations, bvh.Registrations()...)
	registrations = append(registrations, dae.Registrations()...)
	registrations = append(registrations, fbx.Registrations()...)
	registrations = append(registrations, gltf.Registrations()...)
	registrations = append(registrations, obj.Registrations()...)
	registrations = append(registrations, ply.Registrations()...)
	registrations = append(registrations, stl.Registrations()...)
	registrations = append(registrations, tds.Registrations()...)
	registrations = append(registrations, threemf.Registrations()...)
	registrations = append(registrations, usda.Registrations()...)
	return registrations
}

// DefaultRegistry returns the shared built-in decoder registry.
func DefaultRegistry() *detect.Registry {
	return defaultRegistry
}

// NewRegistry returns a fresh registry containing all built-in decoders.
func NewRegistry() *detect.Registry {
	return detect.NewRegistry(Registrations()...)
}
