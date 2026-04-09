package decutil_test

import (
	"encoding/binary"
	"testing"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
)

// buildVorbisComment constructs raw Vorbis comment bytes for testing.
func buildVorbisComment(vendor string, tags map[string]string) []byte {
	var buf []byte
	// Vendor length + vendor string.
	vLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vLen, uint32(len(vendor)))
	buf = append(buf, vLen...)
	buf = append(buf, vendor...)

	// Comment count.
	cLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(cLen, uint32(len(tags)))
	buf = append(buf, cLen...)

	// Each entry: len(4LE) + "KEY=VALUE".
	for k, v := range tags {
		entry := k + "=" + v
		eLen := make([]byte, 4)
		binary.LittleEndian.PutUint32(eLen, uint32(len(entry)))
		buf = append(buf, eLen...)
		buf = append(buf, entry...)
	}
	return buf
}

func TestParseVorbisComment(t *testing.T) {
	tests := []struct {
		name   string
		vendor string
		tags   map[string]string
		want   ir.AudioMetadata
	}{
		{
			name:   "All Fields",
			vendor: "TestEncoder",
			tags: map[string]string{
				"TITLE":   "Test Song",
				"ARTIST":  "Test Artist",
				"ALBUM":   "Test Album",
				"GENRE":   "Electronic",
				"COMMENT": "A test comment",
			},
			want: ir.AudioMetadata{
				Title:   "Test Song",
				Artist:  "Test Artist",
				Album:   "Test Album",
				Genre:   "Electronic",
				Comment: "A test comment",
			},
		},
		{
			name:   "Case Insensitive",
			vendor: "enc",
			tags: map[string]string{
				"title":  "lower",
				"Artist": "mixed",
			},
			want: ir.AudioMetadata{
				Title:  "lower",
				Artist: "mixed",
			},
		},
		{
			name:   "Empty Struct",
			vendor: "",
			tags:   nil,
			want:   ir.AudioMetadata{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data []byte
			if tt.tags != nil || tt.vendor != "" {
				data = buildVorbisComment(tt.vendor, tt.tags)
			}
			meta := decutil.ParseVorbisComment(data)
			assert.Equal(t, tt.want, meta)
		})
	}
}

// buildID3v2 constructs raw ID3v2 bytes for testing.
func buildID3v2(frames [][2]string) []byte {
	rawFrames := make([][]byte, 0, len(frames))
	for _, f := range frames {
		id, text := f[0], f[1]
		rawFrames = append(rawFrames, buildID3v2Frame(id, append([]byte{3}, text...)))
	}
	return buildID3v2Raw(rawFrames...)
}

func buildID3v2Frame(id string, payload []byte) []byte {
	header := make([]byte, 10)
	copy(header[:4], id)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))
	return append(header, payload...)
}

func buildID3v2Raw(frames ...[]byte) []byte {
	var frameData []byte
	for _, frame := range frames {
		frameData = append(frameData, frame...)
	}

	size := len(frameData)
	header := []byte{
		'I', 'D', '3',
		4, 0, 0, // major=4, minor=0, flags=0
		byte(size >> 21 & 0x7F),
		byte(size >> 14 & 0x7F),
		byte(size >> 7 & 0x7F),
		byte(size & 0x7F),
	}

	return append(header, frameData...)
}

func TestParseID3v2Tags(t *testing.T) {
	tests := []struct {
		name   string
		frames [][2]string
		want   ir.AudioMetadata
	}{
		{
			name: "All Fields",
			frames: [][2]string{
				{"TIT2", "Song Title"},
				{"TPE1", "Song Artist"},
				{"TALB", "Song Album"},
				{"TCON", "Pop"},
			},
			want: ir.AudioMetadata{
				Title:  "Song Title",
				Artist: "Song Artist",
				Album:  "Song Album",
				Genre:  "Pop",
			},
		},
		{
			name:   "No Header",
			frames: nil,
			want:   ir.AudioMetadata{},
		},
		{
			name:   "Empty Data",
			frames: nil,
			want:   ir.AudioMetadata{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data []byte
			if tt.name == "No Header" {
				data = []byte("not an id3 file")
			} else if tt.name != "Empty Data" {
				data = buildID3v2(tt.frames)
			}

			meta := decutil.ParseID3v2Tags(data)
			assert.Equal(t, tt.want, meta)
		})
	}
}
