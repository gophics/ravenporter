package wav

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWavMP3Integration(t *testing.T) {
	buf := new(bytes.Buffer)

	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(0))
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(0x0055))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint32(44100))
	binary.Write(buf, binary.LittleEndian, uint32(44100*4))
	binary.Write(buf, binary.LittleEndian, uint16(4))
	binary.Write(buf, binary.LittleEndian, uint16(16))

	mp3Bytes := []byte{0xFF, 0xFB, 0x90, 0x00, 0x00, 0x00}
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(mp3Bytes)))
	buf.Write(mp3Bytes)

	data := buf.Bytes()
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))

	d := &Decoder{}
	r := bytes.NewReader(data)
	scene, err := d.Decode(r, detect.DecodeOptions{MaxFileSize: 10 * 1024 * 1024})

	require.NoError(t, err)
	require.Len(t, scene.AudioClips, 1)
	assert.Equal(t, "WAV Audio", scene.AudioClips[0].Name)
	assert.Equal(t, ir.AudioWAV, scene.AudioClips[0].Format)
}

func BenchmarkWavMP3Integration(b *testing.B) {
	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(0))
	buf.WriteString("WAVEfmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(0x0055))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint32(44100))
	binary.Write(buf, binary.LittleEndian, uint32(44100*4))
	binary.Write(buf, binary.LittleEndian, uint16(4))
	binary.Write(buf, binary.LittleEndian, uint16(16))

	mp3Bytes := []byte{0xFF, 0xFB, 0x90, 0x00, 0x00, 0x00}
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(mp3Bytes)))
	buf.Write(mp3Bytes)

	data := buf.Bytes()
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))

	opts := detect.DecodeOptions{MaxFileSize: 10 * 1024 * 1024}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		d := &Decoder{}
		r := bytes.NewReader(data)
		_, _ = d.Decode(r, opts)
	}
}
