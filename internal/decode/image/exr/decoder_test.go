package exr_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"os"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/exr"
	"github.com/gophics/ravenporter/ir"
)

var exrData []byte

func init() {
	var err error
	exrData, err = os.ReadFile("../testdata/minimal.exr")
	if err != nil {
		panic("failed to load exrData: " + err.Error())
	}
}

func TestEXRProbe(t *testing.T) {
	dec := &exr.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(exrData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestEXRDecode(t *testing.T) {
	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(exrData), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	assert.Equal(t, ir.ImageEXR, scene.Images[0].Format)
	assert.Equal(t, ir.ColorLinear, scene.Images[0].ColorSpace)
	assert.NotEmpty(t, scene.Images[0].Compressed)
}

func TestEXRScanlineRaw(t *testing.T) {
	// 1x1 Float32 uncompressed
	pixelData := []byte{0x00, 0x00, 0x80, 0x3f} // Float32 1.0 (Little Endian)
	data := buildSyntheticEXR(1, 1, 0, false, pixelData)

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.True(t, len(pb.Data) >= 16)
}

func TestEXRScanlineZIP(t *testing.T) {
	// 1x1 Float32 ZIP compressed
	pixelData := []byte{0x00, 0x00, 0x80, 0x3f}

	var zlibBuf bytes.Buffer
	zw := zlib.NewWriter(&zlibBuf)
	_, _ = zw.Write(pixelData)
	_ = zw.Close()

	data := buildSyntheticEXR(1, 1, 2, false, zlibBuf.Bytes()) // comp 2 = ZIPS

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.True(t, len(pb.Data) >= 16)
}

func TestEXRScanlineFloat16(t *testing.T) {
	// 1x1 Float16 (Half) uncompressed
	pixelData := []byte{0x00, 0x3C} // Float16 1.0 = 0x3C00 (Little Endian)
	data := buildSyntheticEXR(1, 1, 0, false, pixelData)

	// Modify the synthetic exr to report Float16 in the channels attribute manually
	// "R" \x00 <type:uint32>...
	// In buildSyntheticEXR we wrote type 2 (Float32). Replace the first `02 00 00 00` with `01 00 00 00`.
	data = bytes.Replace(data, []byte{2, 0, 0, 0}, []byte{1, 0, 0, 0}, 1)

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.True(t, len(pb.Data) >= 16)
}

func TestEXRScanlinePIZ_InvalidData(t *testing.T) {
	// 1x1 PIZ compressed (id = 4), with garbage data to verify routing
	pixelData := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	data := buildSyntheticEXR(1, 1, 4, false, pixelData)

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	// Should fail gracefully because PIZ data is invalid, not panic.
	// Note: scanline errors return nil decData but ok=true so DecodePixels returns no error.
	_, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
}

func TestEXRTiledRaw(t *testing.T) {
	// 1x1 Float32 Tiled
	pixelData := []byte{0x00, 0x00, 0x80, 0x3f}
	data := buildSyntheticEXR(1, 1, 0, true, pixelData)

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.True(t, len(pb.Data) >= 16)
}

func TestEXRIsNotGPUCompressed(t *testing.T) {
	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(exrData), detect.DecodeOptions{})
	require.NoError(t, err)
	assert.False(t, scene.Images[0].IsGPUCompressed())
}

func TestEXRDeepAndMultiPartDetection(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x76, 0x2f, 0x31, 0x01})
	putU32LE4(&buf, 2|0x0800|0x1000) // Deep (0x800) + MultiPart (0x1000)

	writeAttr(&buf, "dataWindow", "box2i", 16)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, 3)
	putU32LE4(&buf, 3)
	buf.WriteByte(0)

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	img := scene.Images[0]
	assert.Equal(t, ir.ImageEXR, img.Format)
}

func TestEXRTiledDecode(t *testing.T) {
	var buf bytes.Buffer

	buf.Write([]byte{0x76, 0x2f, 0x31, 0x01})
	putU32LE4(&buf, 2|0x200)

	writeAttr(&buf, "dataWindow", "box2i", 16)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, 3)
	putU32LE4(&buf, 3)

	writeAttr(&buf, "tiles", "tiledesc", 9)
	putU32LE4(&buf, 32)
	putU32LE4(&buf, 32)
	buf.WriteByte(0)

	chlist := buildChannelList([]string{"A", "B", "G", "R"})
	writeAttr(&buf, "channels", "chlist", len(chlist))
	buf.Write(chlist)

	writeAttr(&buf, "compression", "compression", 1)
	buf.WriteByte(0)

	buf.WriteByte(0)

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(buf.Bytes()), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	img := scene.Images[0]
	assert.Equal(t, 4, img.Width, "tiled EXR width")
	assert.Equal(t, 4, img.Height, "tiled EXR height")
	assert.Equal(t, ir.ChannelRGBA, img.Channels)
}

func writeAttr(buf *bytes.Buffer, name, typeName string, size int) {
	buf.WriteString(name)
	buf.WriteByte(0)
	buf.WriteString(typeName)
	buf.WriteByte(0)
	putU32LE4(buf, uint32(size))
}

func putU32LE4(buf *bytes.Buffer, v uint32) {
	buf.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}

func buildChannelList(names []string) []byte {
	var b bytes.Buffer
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte(0)
		putU32LE4(&b, 1)
		b.WriteByte(0)
		b.Write([]byte{0, 0, 0})
		putU32LE4(&b, 1)
		putU32LE4(&b, 1)
	}
	b.WriteByte(0)
	return b.Bytes()
}

func buildSyntheticEXR(w, h, comp int, tiled bool, data []byte) []byte { //nolint:unparam // test helper
	var buf bytes.Buffer
	buf.Write([]byte{0x76, 0x2f, 0x31, 0x01})

	flags := uint32(2) // Version 2
	if tiled {
		flags |= 0x200
	}
	putU32LE4(&buf, flags)

	writeAttr(&buf, "dataWindow", "box2i", 16)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, uint32(w-1))
	putU32LE4(&buf, uint32(h-1))

	writeAttr(&buf, "channels", "chlist", 19)
	buf.WriteString("R")
	buf.WriteByte(0)
	putU32LE4(&buf, 2) // Float32
	putU32LE4(&buf, 0) // pLinear
	putU32LE4(&buf, 1) // xSampling
	putU32LE4(&buf, 1) // ySampling
	buf.WriteByte(0)

	writeAttr(&buf, "compression", "compression", 1)
	buf.WriteByte(byte(comp))

	if tiled {
		writeAttr(&buf, "tiles", "tiledesc", 9)
		putU32LE4(&buf, uint32(w)) // tile W
		putU32LE4(&buf, uint32(h)) // tile H
		buf.WriteByte(0)           // mode ONE_LEVEL
	}

	buf.WriteByte(0) // end header

	hdrEnd := buf.Len()

	if !tiled {
		// Offset table: h * 8 bytes
		scanlineSize := 8 + len(data)
		offset := hdrEnd + h*8
		for range h {
			buf.Write([]byte{byte(offset), byte(offset >> 8), byte(offset >> 16), byte(offset >> 24), 0, 0, 0, 0})
			offset += scanlineSize
		}
		// Scanlines
		for y := range h {
			putU32LE4(&buf, uint32(y))
			putU32LE4(&buf, uint32(len(data)))
			buf.Write(data)
		}
	} else {
		// Tiled offset table (for 1 tile = 8 bytes)
		// Tiled offset table (for 1 tile = 8 bytes)
		offset := hdrEnd + 8
		buf.Write([]byte{byte(offset), byte(offset >> 8), byte(offset >> 16), byte(offset >> 24), 0, 0, 0, 0})

		// 1 Tile
		putU32LE4(&buf, 0) // dx
		putU32LE4(&buf, 0) // dy
		putU32LE4(&buf, 0) // lx
		putU32LE4(&buf, 0) // ly
		putU32LE4(&buf, uint32(len(data)))
		buf.Write(data)
	}

	return buf.Bytes()
}

func buildChunkedEXR(w, h, comp, chunkH int, pixelData [][]byte) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x76, 0x2f, 0x31, 0x01})
	putU32LE4(&buf, 2)

	writeAttr(&buf, "dataWindow", "box2i", 16)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, 0)
	putU32LE4(&buf, uint32(w-1))
	putU32LE4(&buf, uint32(h-1))

	writeAttr(&buf, "channels", "chlist", 19)
	buf.WriteString("R")
	buf.WriteByte(0)
	putU32LE4(&buf, 2) // Float32
	putU32LE4(&buf, 0)
	putU32LE4(&buf, 1)
	putU32LE4(&buf, 1)
	buf.WriteByte(0)

	writeAttr(&buf, "compression", "compression", 1)
	buf.WriteByte(byte(comp))
	buf.WriteByte(0)

	hdrEnd := buf.Len()
	numChunks := len(pixelData)

	chunkSizes := make([]int, numChunks)
	for i, d := range pixelData {
		chunkSizes[i] = len(d)
	}

	offset := hdrEnd + numChunks*8
	for _, sz := range chunkSizes {
		buf.Write([]byte{byte(offset), byte(offset >> 8), byte(offset >> 16), byte(offset >> 24), 0, 0, 0, 0})
		offset += 8 + sz
	}

	for i, d := range pixelData {
		y := i * chunkH
		putU32LE4(&buf, uint32(y))
		putU32LE4(&buf, uint32(len(d)))
		buf.Write(d)
	}

	return buf.Bytes()
}

func TestEXRScanlineZIP_MultiLine(t *testing.T) {
	const (
		width     = 2
		height    = 4
		nChan     = 1
		floatSize = 4
	)

	rowSize := width * floatSize * nChan
	chunk0Raw := make([]byte, rowSize*height)
	for y := range height {
		for x := range width {
			off := (y*width + x) * floatSize
			val := float32(y*width+x+1) / 10.0
			bits := math.Float32bits(val)
			binary.LittleEndian.PutUint32(chunk0Raw[off:], bits)
		}
	}

	encoded := predictorEncode(chunk0Raw)
	var zlibBuf bytes.Buffer
	zw := zlib.NewWriter(&zlibBuf)
	_, _ = zw.Write(encoded)
	_ = zw.Close()

	data := buildChunkedEXR(width, height, 3, 16, [][]byte{zlibBuf.Bytes()})

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)

	floats := unsafe.Slice((*float32)(unsafe.Pointer(&pb.Data[0])), len(pb.Data)/4)
	for y := range height {
		for x := range width {
			idx := (y*width + x) * 4
			got := floats[idx]
			want := float32(y*width+x+1) / 10.0
			assert.InDelta(t, want, got, 0.001, "pixel [%d,%d] R mismatch", x, y)
		}
	}
}

func predictorEncode(data []byte) []byte {
	if len(data) < 2 {
		return data
	}
	half := len(data) / 2
	interleaved := make([]byte, len(data))
	for i := range half {
		interleaved[i] = data[2*i]
		interleaved[half+i] = data[2*i+1]
	}
	if len(data)%2 != 0 {
		interleaved[len(data)-1] = data[len(data)-1]
	}
	encoded := make([]byte, len(interleaved))
	encoded[0] = interleaved[0]
	for i := 1; i < len(interleaved); i++ {
		encoded[i] = interleaved[i] - interleaved[i-1]
	}
	return encoded
}

func TestEXRScanlineRLE(t *testing.T) {
	pixelData := []byte{0x00, 0x00, 0x80, 0x3f} // Float32 1.0
	rleEncoded := []byte{0x03, 0x00, 0x00, 0x80, 0x3f}

	data := buildSyntheticEXR(1, 1, 1, false, rleEncoded)

	dec := &exr.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]

	pb, decErr := img.DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.True(t, len(pb.Data) >= len(pixelData))
}

func BenchmarkDecode(b *testing.B) {
	dec := &exr.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(exrData), opts)
	}
}

func BenchmarkDecodeZIP(b *testing.B) {
	pixelData := []byte{0x00, 0x00, 0x80, 0x3f}
	var zlibBuf bytes.Buffer
	zw := zlib.NewWriter(&zlibBuf)
	_, _ = zw.Write(pixelData)
	_ = zw.Close()
	data := buildSyntheticEXR(1, 1, 2, false, zlibBuf.Bytes())

	dec := &exr.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(data), opts)
	}
}

func BenchmarkDecodePixelsZIP(b *testing.B) {
	pixelData := []byte{0x00, 0x00, 0x80, 0x3f}
	var zlibBuf bytes.Buffer
	zw := zlib.NewWriter(&zlibBuf)
	_, _ = zw.Write(pixelData)
	_ = zw.Close()
	data := buildSyntheticEXR(1, 1, 2, false, zlibBuf.Bytes())

	dec := &exr.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		scene, _ := dec.Decode(bytes.NewReader(data), opts)
		_, _ = scene.Images[0].DecodePixels()
	}
}
