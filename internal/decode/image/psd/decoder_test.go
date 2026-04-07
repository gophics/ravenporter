package psd_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/image/psd"
	"github.com/gophics/ravenporter/ir"
)

var psdData []byte

func init() {
	var err error
	psdData, err = os.ReadFile("../testdata/minimal.psd")
	if err != nil {
		panic("failed to load psdData: " + err.Error())
	}
}

func TestPSDProbe(t *testing.T) {
	dec := &psd.Decoder{}
	assert.True(t, dec.Probe(bytes.NewReader(psdData)))
	assert.False(t, dec.Probe(bytes.NewReader([]byte("no"))))
}

func TestPSDDecode(t *testing.T) {
	dec := &psd.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(psdData), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	img := scene.Images[0]
	assert.Equal(t, ir.ImagePSD, img.Format)
	assert.Equal(t, 100, img.Width)
	assert.Equal(t, 200, img.Height)
}

func TestPSDDecodePixels(t *testing.T) {
	dec := &psd.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(psdData), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)

	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, ir.DataTypeUint8, pb.DataType)
	assert.Equal(t, ir.BitDepth8, pb.BitDepth)
	assert.Len(t, pb.Data, 100*200*4)
}

func TestPSDDecodeWithLayers(t *testing.T) {
	data := make([]byte, 26)
	copy(data, "8BPS")
	data[22] = 0
	data[23] = 16

	data = append(data, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 16,
		0, 0, 0, 12, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)

	dec := &psd.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)

	img := scene.Images[0]
	assert.Equal(t, "true", img.Metadata["PSDFloatLDR"])
	assert.Equal(t, "2", img.Metadata["LayerCount"])
}

func TestPSDCompositePixels(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantLen int
		wantErr bool
		wantR0  byte
	}{
		{
			name:    "raw_rgb_1x1",
			data:    buildPSDFull(1, 1, 3, 8, 3, 0, []byte{0xFF, 0x80, 0x40}),
			wantLen: 4,
			wantR0:  0xFF,
		},
		{
			name:    "raw_rgba_2x1",
			data:    buildPSDFull(2, 1, 4, 8, 3, 0, []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22}),
			wantLen: 8,
			wantR0:  0xAA,
		},
		{
			name:    "rle_rgb_2x1",
			data:    buildPSDRLE(2, 1, 3),
			wantLen: 8,
			wantR0:  0x80,
		},
		{
			name:    "raw_gray_1x1",
			data:    buildPSDFull(1, 1, 1, 8, 1, 0, []byte{0xCC}),
			wantLen: 4,
			wantR0:  0xCC,
		},
		{
			name:    "raw_cmyk_1x1",
			data:    buildPSDFull(1, 1, 4, 8, 4, 0, []byte{0, 0, 0, 0}),
			wantLen: 4,
			wantR0:  0xFF,
		},
		{
			name:    "raw_16bit_rgb_1x1",
			data:    buildPSD16bit(1, 1, 3, []byte{0xFF, 0xFF, 0x80, 0x00, 0x00, 0x00}),
			wantLen: 4,
			wantR0:  0xFF,
		},
		{
			name:    "raw_32bit_rgb_1x1",
			data:    buildPSD32bit(1, 1, 3, []float32{1.0, 0.5, 0.0}),
			wantLen: 4,
			wantR0:  0xFF,
		},
		{
			name:    "zip_rgb_2x1",
			data:    buildPSDZIP(2, 1, 3),
			wantLen: 8,
			wantR0:  0xAA,
		},
		{
			name:    "zip_prediction_rgb_2x1",
			data:    buildPSDZIPPrediction(2, 1, 3),
			wantLen: 8,
			wantR0:  0xAA,
		},
		{
			name:    "truncated_header",
			data:    []byte("8BPS"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &psd.Decoder{}
			scene, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			if err != nil {
				if tt.wantErr {
					return
				}
				require.NoError(t, err)
			}
			require.Len(t, scene.Images, 1)

			pb, decErr := scene.Images[0].DecodePixels()
			if tt.wantErr {
				assert.Error(t, decErr)
				return
			}
			require.NoError(t, decErr)
			require.NotNil(t, pb)
			assert.Len(t, pb.Data, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantR0, pb.Data[0])
			}
		})
	}
}

func TestPSDMetadata(t *testing.T) {
	data := buildPSDFull(1, 1, 3, 16, 3, 0, make([]byte, 6))
	dec := &psd.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	img := scene.Images[0]
	assert.Equal(t, "16", img.Metadata["BitDepth"])
	assert.Equal(t, "3", img.Metadata["ColorMode"])
}

func TestPSDPackBitsEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "noop_byte",
			data: buildPSDRLECustom(2, 1, 3, []byte{0x80, 0x01, 0xAA, 0xBB}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &psd.Decoder{}
			scene, err := dec.Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			require.NoError(t, err)
			pb, decErr := scene.Images[0].DecodePixels()
			require.NoError(t, decErr)
			require.NotNil(t, pb)
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	dec := &psd.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = dec.Decode(bytes.NewReader(psdData), opts)
	}
}

func BenchmarkDecodePixels(b *testing.B) {
	dec := &psd.Decoder{}
	scene, err := dec.Decode(bytes.NewReader(psdData), detect.DecodeOptions{})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		scene2, _ := dec.Decode(bytes.NewReader(psdData), detect.DecodeOptions{})
		_, _ = scene2.Images[0].DecodePixels()
	}
	_ = scene
}

// --- Test helpers ---

func buildPSDFull(w, h, ch, depth, mode int, comp uint16, pixelData []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString("8BPS")
	buf.Write([]byte{0, 1})
	buf.Write(make([]byte, 6))
	buf.Write([]byte{byte(ch >> 8), byte(ch)})
	buf.Write([]byte{byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h)})
	buf.Write([]byte{byte(w >> 24), byte(w >> 16), byte(w >> 8), byte(w)})
	buf.Write([]byte{byte(depth >> 8), byte(depth)})
	buf.Write([]byte{byte(mode >> 8), byte(mode)})

	buf.Write([]byte{0, 0, 0, 0}) // Color Mode Data.
	buf.Write([]byte{0, 0, 0, 0}) // Image Resources.
	buf.Write([]byte{0, 0, 0, 0}) // Layer & Mask Info.

	buf.Write([]byte{byte(comp >> 8), byte(comp)})
	buf.Write(pixelData)

	return buf.Bytes()
}

func buildPSD16bit(w, h, ch int, pixelData []byte) []byte {
	return buildPSDFull(w, h, ch, 16, 3, 0, pixelData)
}

func buildPSD32bit(w, h, ch int, floats []float32) []byte {
	var pixelData bytes.Buffer
	for _, f := range floats {
		_ = binary.Write(&pixelData, binary.BigEndian, f)
	}
	return buildPSDFull(w, h, ch, 32, 3, 0, pixelData.Bytes())
}

func buildPSDZIP(w, h, ch int) []byte {
	planarSize := w * h * ch
	planar := make([]byte, planarSize)
	for c := range ch {
		for i := range w * h {
			planar[c*w*h+i] = byte(0xAA + c*0x11)
		}
	}

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	_, _ = zw.Write(planar)
	_ = zw.Close()

	return buildPSDFull(w, h, ch, 8, 3, 2, compressed.Bytes())
}

func buildPSDZIPPrediction(w, h, ch int) []byte {
	planarSize := w * h * ch
	planar := make([]byte, planarSize)
	for c := range ch {
		for i := range w * h {
			planar[c*w*h+i] = byte(0xAA + c*0x11)
		}
	}

	// Delta encode horizontally (inverse of undoPrediction).
	encoded := make([]byte, planarSize)
	for c := range ch {
		for y := range h {
			offset := c*h*w + y*w
			encoded[offset] = planar[offset]
			for x := w - 1; x > 0; x-- {
				encoded[offset+x] = planar[offset+x] - planar[offset+x-1]
			}
		}
	}

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	_, _ = zw.Write(encoded)
	_ = zw.Close()

	return buildPSDFull(w, h, ch, 8, 3, 3, compressed.Bytes())
}

func buildPSDRLE(w, h, ch int) []byte {
	totalScanlines := h * ch

	var singleRow bytes.Buffer
	singleRow.WriteByte(0x80 | byte(w-1))
	singleRow.WriteByte(0x80)
	rowLen := singleRow.Len()

	var rowTable bytes.Buffer
	for range totalScanlines {
		rowTable.Write([]byte{byte(rowLen >> 8), byte(rowLen)})
	}

	var rlePayload bytes.Buffer
	for range totalScanlines {
		rlePayload.Write(singleRow.Bytes())
	}

	var imageData bytes.Buffer
	imageData.Write(rowTable.Bytes())
	imageData.Write(rlePayload.Bytes())

	return buildPSDFull(w, h, ch, 8, 3, 1, imageData.Bytes())
}

func buildPSDRLECustom(w, h, ch int, rowRLE []byte) []byte {
	totalScanlines := h * ch

	var rowTable bytes.Buffer
	for range totalScanlines {
		rl := len(rowRLE)
		rowTable.Write([]byte{byte(rl >> 8), byte(rl)})
	}

	var rlePayload bytes.Buffer
	for range totalScanlines {
		rlePayload.Write(rowRLE)
	}

	var imageData bytes.Buffer
	imageData.Write(rowTable.Bytes())
	imageData.Write(rlePayload.Bytes())

	return buildPSDFull(w, h, ch, 8, 3, 1, imageData.Bytes())
}
