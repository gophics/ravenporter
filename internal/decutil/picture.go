package decutil

import (
	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/ir"
)

const (
	picMinSize      = 42
	picHeaderFields = 4
	picDimFields    = 16
)

// ExtractPictureComment parses a FLAC METADATA_BLOCK_PICTURE binary block
// and injects any valid description into the provided metadata struct.
func ExtractPictureComment(block []byte, meta *ir.AudioMetadata) {
	if len(block) < picMinSize {
		return
	}
	off := picHeaderFields // Skip PictureType (4 bytes)

	mimeLen := int(binread.ReadU32BE(block[off:]))
	off += picHeaderFields
	if off+mimeLen > len(block) {
		return
	}
	off += mimeLen

	if off+picHeaderFields > len(block) {
		return
	}
	descLen := int(binread.ReadU32BE(block[off:]))
	off += picHeaderFields
	if off+descLen > len(block) {
		return
	}
	desc := string(block[off : off+descLen])
	off += descLen

	off += picDimFields // Skip dimensions (width, height, depth, colors)
	if off+picHeaderFields > len(block) {
		return
	}

	if desc != "" && meta.Comment == "" {
		meta.Comment = desc
	}
}
