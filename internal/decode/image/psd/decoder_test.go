package psd_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
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
		name         string
		data         []byte
		wantLen      int
		wantErr      bool
		wantDataType ir.DataType
		wantBitDepth ir.BitDepth
		wantPrefix   []byte
		wantFloats   []float32
	}{
		{
			name:       "raw_rgb_1x1",
			data:       buildPSDFull(1, 1, 3, 8, 3, 0, []byte{0xFF, 0x80, 0x40}),
			wantLen:    4,
			wantPrefix: []byte{0xFF},
		},
		{
			name:       "raw_rgba_2x1",
			data:       buildPSDFull(2, 1, 4, 8, 3, 0, []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22}),
			wantLen:    8,
			wantPrefix: []byte{0xAA},
		},
		{
			name:       "rle_rgb_2x1",
			data:       buildPSDRLE(2, 1, 3),
			wantLen:    8,
			wantPrefix: []byte{0x80},
		},
		{
			name:       "raw_gray_1x1",
			data:       buildPSDFull(1, 1, 1, 8, 1, 0, []byte{0xCC}),
			wantLen:    4,
			wantPrefix: []byte{0xCC},
		},
		{
			name:       "raw_cmyk_1x1",
			data:       buildPSDFull(1, 1, 4, 8, 4, 0, []byte{0, 0, 0, 0}),
			wantLen:    4,
			wantPrefix: []byte{0xFF},
		},
		{
			name:       "raw_bitmap_1x1",
			data:       buildPSDFull(1, 1, 1, 1, 0, 0, []byte{0x80}),
			wantLen:    4,
			wantPrefix: []byte{0xFF},
		},
		{
			name: "raw_indexed_1x1",
			data: func() []byte {
				palette := make([]byte, 256*3)
				palette[1] = 0xFF
				return buildPSDFullWithColorModeData(1, 1, 1, 8, 2, palette, 0, []byte{0x01})
			}(),
			wantLen:    4,
			wantPrefix: []byte{0xFF},
		},
		{
			name:       "raw_duotone_1x1",
			data:       buildPSDFullWithColorModeData(1, 1, 1, 8, 8, []byte{0x01}, 0, []byte{0x66}),
			wantLen:    4,
			wantPrefix: []byte{0x66},
		},
		{
			name:       "raw_lab_1x1",
			data:       buildPSDFull(1, 1, 3, 8, 9, 0, []byte{0xFF, 0x80, 0x80}),
			wantLen:    4,
			wantPrefix: []byte{0xFF},
		},
		{
			name:       "raw_multichannel_1x1",
			data:       buildPSDFull(1, 1, 3, 8, 7, 0, []byte{0xFF, 0x00, 0x00}),
			wantLen:    4,
			wantPrefix: []byte{0x00},
		},
		{
			name:         "raw_16bit_rgb_1x1",
			data:         buildPSD16bit(1, 1, 3, []byte{0xFF, 0xFF, 0x80, 0x00, 0x00, 0x00}),
			wantLen:      8,
			wantDataType: ir.DataTypeUint16,
			wantBitDepth: ir.BitDepth16,
			wantPrefix:   []byte{0xFF, 0xFF, 0x00, 0x80, 0x00, 0x00, 0xFF, 0xFF},
		},
		{
			name:         "raw_32bit_rgb_1x1",
			data:         buildPSD32bit(1, 1, 3, []float32{1.0, 0.5, 0.0}),
			wantLen:      16,
			wantDataType: ir.DataTypeFloat32,
			wantBitDepth: ir.BitDepth32,
			wantFloats:   []float32{1.0, 0.5, 0.0, 1.0},
		},
		{
			name:       "zip_rgb_2x1",
			data:       buildPSDZIP(2, 1, 3),
			wantLen:    8,
			wantPrefix: []byte{0xAA},
		},
		{
			name:       "zip_prediction_rgb_2x1",
			data:       buildPSDZIPPrediction(2, 1, 3),
			wantLen:    8,
			wantPrefix: []byte{0xAA},
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
			wantType := tt.wantDataType
			wantDepth := tt.wantBitDepth
			if wantDepth == 0 {
				wantDepth = ir.BitDepth8
			}
			assert.Equal(t, wantType, pb.DataType)
			assert.Equal(t, wantDepth, pb.BitDepth)
			assert.Len(t, pb.Data, tt.wantLen)
			if len(tt.wantFloats) > 0 {
				assertFloat32Prefix(t, pb.Data, tt.wantFloats)
			}
			if len(tt.wantPrefix) > 0 {
				assert.Equal(t, tt.wantPrefix, pb.Data[:len(tt.wantPrefix)])
			}
		})
	}
}

func TestPSDCompositeValidation(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "indexed without palette",
			data: buildPSDFull(1, 1, 1, 8, 2, 0, []byte{0x00}),
		},
		{
			name: "bitmap with 8-bit depth",
			data: buildPSDFull(1, 1, 1, 8, 0, 0, []byte{0x00}),
		},
		{
			name: "rgb with 1-bit depth",
			data: buildPSDFull(1, 1, 3, 1, 3, 0, []byte{0x00}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := (&psd.Decoder{}).Decode(bytes.NewReader(tt.data), detect.DecodeOptions{})
			require.NoError(t, err)
			_, decErr := scene.Images[0].DecodePixels()
			assert.Error(t, decErr)
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

func TestPSDIndexedPaletteColors(t *testing.T) {
	palette := make([]byte, 256*3)
	palette[2] = 0x20
	palette[256+2] = 0x40
	palette[512+2] = 0x60
	data := buildPSDFullWithColorModeData(1, 1, 1, 8, 2, palette, 0, []byte{0x02})

	scene, err := (&psd.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, []byte{0x20, 0x40, 0x60, 0xFF}, pb.Data[:4])
}

func TestPSDIndexedAlpha(t *testing.T) {
	palette := make([]byte, 256*3)
	palette[4] = 0x10
	palette[256+4] = 0x20
	palette[512+4] = 0x30
	data := buildPSDFullWithColorModeData(1, 1, 2, 8, 2, palette, 0, []byte{0x04, 0x40})

	scene, err := (&psd.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, []byte{0x10, 0x20, 0x30, 0x40}, pb.Data[:4])
}

func TestPSDBitmapBlackPixel(t *testing.T) {
	data := buildPSDFull(1, 1, 1, 1, 0, 0, []byte{0x00})

	scene, err := (&psd.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0xFF}, pb.Data[:4])
}

func TestPSDMultichannelComposite(t *testing.T) {
	data := buildPSDFull(1, 1, 3, 8, 7, 0, []byte{0xFF, 0x00, 0x00})

	scene, err := (&psd.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, []byte{0x00, 0xFF, 0xFF, 0xFF}, pb.Data[:4])
}

func TestPSDLabAlpha(t *testing.T) {
	data := buildPSDFull(1, 1, 4, 8, 9, 0, []byte{0xFF, 0x80, 0x80, 0x40})

	scene, err := (&psd.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Equal(t, byte(0x40), pb.Data[3])
}

func TestPSBCompositePixels(t *testing.T) {
	data := buildPSBFull(1, 1, 3, 8, 3, 0, []byte{0xFF, 0x80, 0x40})
	scene, err := (&psd.Decoder{}).Decode(bytes.NewReader(data), detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, scene.Images, 1)
	pb, decErr := scene.Images[0].DecodePixels()
	require.NoError(t, decErr)
	require.NotNil(t, pb)
	assert.Len(t, pb.Data, 4)
	assert.Equal(t, byte(0xFF), pb.Data[0])
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

func BenchmarkDecodePixels16Bit(b *testing.B) {
	data := buildPSD16bit(64, 64, 3, make([]byte, 64*64*3*2))
	dec := &psd.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		scene, _ := dec.Decode(bytes.NewReader(data), opts)
		_, _ = scene.Images[0].DecodePixels()
	}
}

func BenchmarkDecodePixels32Bit(b *testing.B) {
	floatData := make([]float32, 64*64*3)
	for i := range floatData {
		floatData[i] = 0.5
	}
	data := buildPSD32bit(64, 64, 3, floatData)
	dec := &psd.Decoder{}
	opts := detect.DecodeOptions{}
	b.ReportAllocs()
	for b.Loop() {
		scene, _ := dec.Decode(bytes.NewReader(data), opts)
		_, _ = scene.Images[0].DecodePixels()
	}
}

// --- Test helpers ---

func buildPSDFull(w, h, ch, depth, mode int, comp uint16, pixelData []byte) []byte {
	return buildPSDFullWithColorModeData(w, h, ch, depth, mode, nil, comp, pixelData)
}

func buildPSDFullWithColorModeData(w, h, ch, depth, mode int, colorModeData []byte, comp uint16, pixelData []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString("8BPS")
	buf.Write([]byte{0, 1})
	buf.Write(make([]byte, 6))
	buf.Write([]byte{byte(ch >> 8), byte(ch)})
	buf.Write([]byte{byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h)})
	buf.Write([]byte{byte(w >> 24), byte(w >> 16), byte(w >> 8), byte(w)})
	buf.Write([]byte{byte(depth >> 8), byte(depth)})
	buf.Write([]byte{byte(mode >> 8), byte(mode)})

	buf.Write([]byte{byte(len(colorModeData) >> 24), byte(len(colorModeData) >> 16), byte(len(colorModeData) >> 8), byte(len(colorModeData))})
	buf.Write(colorModeData)
	buf.Write([]byte{0, 0, 0, 0}) // Image Resources.
	buf.Write([]byte{0, 0, 0, 0}) // Layer & Mask Info.

	buf.Write([]byte{byte(comp >> 8), byte(comp)})
	buf.Write(pixelData)

	return buf.Bytes()
}

func buildPSBFull(w, h, ch, depth, mode int, comp uint16, pixelData []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString("8BPS")
	buf.Write([]byte{0, 2})
	buf.Write(make([]byte, 6))
	buf.Write([]byte{byte(ch >> 8), byte(ch)})
	buf.Write([]byte{byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h)})
	buf.Write([]byte{byte(w >> 24), byte(w >> 16), byte(w >> 8), byte(w)})
	buf.Write([]byte{byte(depth >> 8), byte(depth)})
	buf.Write([]byte{byte(mode >> 8), byte(mode)})

	buf.Write([]byte{0, 0, 0, 0})
	buf.Write([]byte{0, 0, 0, 0})
	buf.Write(make([]byte, 8))

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

func assertFloat32Prefix(t *testing.T, data []byte, want []float32) {
	t.Helper()
	require.GreaterOrEqual(t, len(data), len(want)*4)
	for i := range want {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
		assert.InDelta(t, want[i], got, 0.0001)
	}
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
