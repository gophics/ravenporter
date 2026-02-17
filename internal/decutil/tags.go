package decutil

import (
	"encoding/binary"
	"strings"

	"github.com/gophics/ravenporter/ir"
)

// Standard Vorbis comment field names.
const (
	tagTitle   = "TITLE"
	tagArtist  = "ARTIST"
	tagAlbum   = "ALBUM"
	tagGenre   = "GENRE"
	tagComment = "COMMENT"
)

// ParseVorbisComment parses a Vorbis comment block from raw bytes (starting after
// the vendor string length field). Used by FLAC, OGG Vorbis, and Opus.
// The data layout is: vendor_len(4LE) + vendor + count(4LE) + [len(4LE) + "KEY=VALUE"]*N.
func ParseVorbisComment(data []byte) ir.AudioMetadata {
	var meta ir.AudioMetadata
	if len(data) < 4 { //nolint:mnd // vendor length
		return meta
	}
	vendorLen := int(binary.LittleEndian.Uint32(data[:4]))
	off := 4 + vendorLen //nolint:mnd // skip vendor length field
	if off+4 > len(data) {
		return meta
	}
	count := int(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4

	for i := 0; i < count && off+4 <= len(data); i++ {
		entryLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if off+entryLen > len(data) {
			break
		}
		entry := string(data[off : off+entryLen])
		off += entryLen

		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case tagTitle:
			meta.Title = value
		case tagArtist:
			meta.Artist = value
		case tagAlbum:
			meta.Album = value
		case tagGenre:
			meta.Genre = value
		case tagComment:
			meta.Comment = value
		}
	}
	return meta
}

// ID3v2 text frame IDs (v2.3/v2.4).
const (
	id3Title   = "TIT2"
	id3Artist  = "TPE1"
	id3Album   = "TALB"
	id3Genre   = "TCON"
	id3Comment = "COMM"
)

// ParseID3v2Tags extracts basic tags from an ID3v2 header.
// Expects the full file data; returns zero-value metadata if no ID3v2 present.
func ParseID3v2Tags(data []byte) ir.AudioMetadata {
	var meta ir.AudioMetadata
	if len(data) < 10 || string(data[:3]) != "ID3" {
		return meta
	}
	// ID3v2 size: 4 syncsafe bytes at offset 6.
	headerSize := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
	tagEnd := min(10+headerSize, len(data)) //nolint:mnd // ID3v2 header size
	pos := 10

	for pos+10 <= tagEnd {
		frameID := string(data[pos : pos+4])
		frameSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		pos += 10

		if frameSize <= 0 || pos+frameSize > tagEnd {
			break
		}
		// Text frames: first byte is encoding (0=ISO-8859-1, 3=UTF-8), rest is text.
		text := ""
		if frameSize > 1 {
			text = strings.TrimRight(string(data[pos+1:pos+frameSize]), "\x00")
		}

		switch frameID {
		case id3Title:
			meta.Title = text
		case id3Artist:
			meta.Artist = text
		case id3Album:
			meta.Album = text
		case id3Genre:
			meta.Genre = text
		}

		pos += frameSize
	}
	return meta
}
