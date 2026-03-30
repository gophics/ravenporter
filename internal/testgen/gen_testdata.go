//go:build ignore

// Command gen_testdata generates minimal valid binary test files for decoder testing.
// Run: go run gen_testdata.go
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"

	"github.com/andybalholm/brotli"
)

func main() {
	genFontTestdata()
	genImageTestdata()
	genSceneTestdata()
	genAudioTestdata()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func writeFile(dir, name string, data []byte) {
	must(os.MkdirAll(dir, 0o755))
	must(os.WriteFile(filepath.Join(dir, name), data, 0o644))
}

// --- Font ---

func genFontTestdata() {
	// TTF: minimal SFNT with name + OS/2 tables.
	ttf := make([]byte, 512)
	ttf[0], ttf[1], ttf[2], ttf[3] = 0x00, 0x01, 0x00, 0x00 // TrueType magic
	binary.BigEndian.PutUint16(ttf[4:6], 2)                 // numTables = 2
	binary.BigEndian.PutUint16(ttf[6:8], 32)                // searchRange

	// Table entry 0: "name" at offset 128, length 64.
	copy(ttf[12:16], "name")
	binary.BigEndian.PutUint32(ttf[20:24], 128) // offset
	binary.BigEndian.PutUint32(ttf[24:28], 64)  // length

	// Table entry 1: "OS/2" at offset 192, length 78.
	copy(ttf[28:32], "OS/2")
	binary.BigEndian.PutUint32(ttf[36:40], 192) // offset
	binary.BigEndian.PutUint32(ttf[40:44], 78)  // length

	// Name table at 128: format=0, count=1, stringOffset=18.
	binary.BigEndian.PutUint16(ttf[128:], 0)  // format
	binary.BigEndian.PutUint16(ttf[130:], 1)  // count
	binary.BigEndian.PutUint16(ttf[132:], 18) // stringOffset

	// Name record: platformID=3, encodingID=1, languageID=0, nameID=1, len=8, off=0.
	binary.BigEndian.PutUint16(ttf[134:], 3) // platformID
	binary.BigEndian.PutUint16(ttf[136:], 1) // encodingID
	binary.BigEndian.PutUint16(ttf[138:], 0) // languageID
	binary.BigEndian.PutUint16(ttf[140:], 1) // nameID (font family)
	binary.BigEndian.PutUint16(ttf[142:], 8) // length
	binary.BigEndian.PutUint16(ttf[144:], 0) // offset
	copy(ttf[146:154], "TestFont")           // string data

	// OS/2 table at 192: sTypoAscender=800, sTypoDescender=-200, sTypoLineGap=90.
	binary.BigEndian.PutUint16(ttf[260:], 800)    // sTypoAscender (192+68=260)
	binary.BigEndian.PutUint16(ttf[262:], 0xFF38) // sTypoDescender -200 (192+70=262)
	binary.BigEndian.PutUint16(ttf[264:], 90)     // sTypoLineGap (192+72=264)

	writeFile("decode/font/testdata", "minimal.ttf", ttf)
	writeFile("decode/font/ttf/testdata", "minimal.ttf", ttf)

	// OTF: "OTTO" magic.
	otf := make([]byte, 64)
	copy(otf[0:4], "OTTO")
	writeFile("decode/font/testdata", "minimal.otf", otf)
	writeFile("decode/font/otf/testdata", "minimal.otf", otf)

	// WOFF: wrap TTF tables in a valid WOFF container with zlib compression.
	woffData := genWOFF(ttf)
	writeFile("decode/font/testdata", "minimal.woff", woffData)
	writeFile("decode/font/woff/testdata", "minimal.woff", woffData)

	// WOFF2: wrap TTF tables in a valid WOFF2 container with brotli compression.
	woff2Data := genWOFF2(ttf)
	writeFile("decode/font/testdata", "minimal.woff2", woff2Data)
	writeFile("decode/font/woff2/testdata", "minimal.woff2", woff2Data)
}

func genWOFF(sfnt []byte) []byte {
	numTables := int(binary.BigEndian.Uint16(sfnt[4:6]))

	type tblEntry struct {
		tag              [4]byte
		checksum         uint32
		origOff, origLen int
		compData         []byte
	}

	entries := make([]tblEntry, numTables)
	for i := range numTables {
		eOff := 12 + i*16
		var e tblEntry
		copy(e.tag[:], sfnt[eOff:eOff+4])
		e.checksum = binary.BigEndian.Uint32(sfnt[eOff+4 : eOff+8])
		e.origOff = int(binary.BigEndian.Uint32(sfnt[eOff+8 : eOff+12]))
		e.origLen = int(binary.BigEndian.Uint32(sfnt[eOff+12 : eOff+16]))

		raw := sfnt[e.origOff : e.origOff+e.origLen]
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		_, _ = zw.Write(raw)
		_ = zw.Close()
		if buf.Len() < e.origLen {
			e.compData = buf.Bytes()
		} else {
			e.compData = raw
		}
		entries[i] = e
	}

	woffTableDirStart := 44
	woffTableEntSize := 20
	dataStart := woffTableDirStart + numTables*woffTableEntSize

	totalSize := dataStart
	for _, e := range entries {
		totalSize += (len(e.compData) + 3) &^ 3
	}

	out := make([]byte, totalSize)
	copy(out[0:4], "wOFF")
	copy(out[4:8], sfnt[0:4])
	binary.BigEndian.PutUint32(out[8:12], uint32(totalSize))
	binary.BigEndian.PutUint16(out[12:14], uint16(numTables))

	dataOff := dataStart
	for i, e := range entries {
		eOff := woffTableDirStart + i*woffTableEntSize
		copy(out[eOff:], e.tag[:])
		binary.BigEndian.PutUint32(out[eOff+4:], uint32(dataOff))
		binary.BigEndian.PutUint32(out[eOff+8:], uint32(len(e.compData)))
		binary.BigEndian.PutUint32(out[eOff+12:], uint32(e.origLen))
		binary.BigEndian.PutUint32(out[eOff+16:], e.checksum)
		copy(out[dataOff:], e.compData)
		dataOff += (len(e.compData) + 3) &^ 3
	}

	return out
}

func genWOFF2(sfnt []byte) []byte {
	numTables := int(binary.BigEndian.Uint16(sfnt[4:6]))

	type tblInfo struct {
		tag     [4]byte
		origLen int
		origOff int
	}

	tables := make([]tblInfo, numTables)
	for i := range numTables {
		eOff := 12 + i*16
		var t tblInfo
		copy(t.tag[:], sfnt[eOff:eOff+4])
		t.origOff = int(binary.BigEndian.Uint32(sfnt[eOff+8 : eOff+12]))
		t.origLen = int(binary.BigEndian.Uint32(sfnt[eOff+12 : eOff+16]))
		tables[i] = t
	}

	knownTags := []string{
		"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post",
		"cvt ", "fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT",
		"EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea",
		"vmtx", "BASE", "GDEF", "GPOS", "GSUB", "EBSC", "JSTF", "MATH",
		"CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar",
		"bdat", "bloc", "bsln", "cvar", "fdsc", "feat", "fmtx", "fvar",
		"gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop",
		"trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
	}

	findKnownIdx := func(tag string) int {
		for i, t := range knownTags {
			if t == tag {
				return i
			}
		}
		return -1
	}

	var rawConcat []byte
	for _, t := range tables {
		rawConcat = append(rawConcat, sfnt[t.origOff:t.origOff+t.origLen]...)
	}

	var compBuf bytes.Buffer
	bw := brotli.NewWriter(&compBuf)
	_, _ = bw.Write(rawConcat)
	_ = bw.Close()
	compressed := compBuf.Bytes()

	var dir []byte
	totalSfntSize := 12 + numTables*16
	for _, t := range tables {
		totalSfntSize += (t.origLen + 3) &^ 3
	}

	for _, t := range tables {
		tag := string(t.tag[:])
		idx := findKnownIdx(tag)
		if idx >= 0 {
			dir = append(dir, byte(idx))
		} else {
			dir = append(dir, 0x3F)
			dir = append(dir, t.tag[:]...)
		}
		dir = append(dir, encodeBase128(uint32(t.origLen))...)
	}

	headerSize := 48
	out := make([]byte, headerSize+len(dir)+len(compressed))
	copy(out[0:4], "wOF2")
	copy(out[4:8], sfnt[0:4])
	binary.BigEndian.PutUint32(out[8:12], uint32(len(out)))
	binary.BigEndian.PutUint16(out[12:14], uint16(numTables))
	binary.BigEndian.PutUint32(out[16:20], uint32(totalSfntSize))
	binary.BigEndian.PutUint32(out[20:24], uint32(len(compressed)))

	copy(out[headerSize:], dir)
	copy(out[headerSize+len(dir):], compressed)

	return out
}

func encodeBase128(val uint32) []byte {
	if val == 0 {
		return []byte{0}
	}
	var result [5]byte
	n := 0
	for val > 0 {
		result[n] = byte(val & 0x7F)
		val >>= 7
		n++
	}
	out := make([]byte, n)
	for i := range n {
		b := result[n-1-i]
		if i < n-1 {
			b |= 0x80
		}
		out[i] = b
	}
	return out
}

// --- Image ---

func genImageTestdata() {
	dir := "decode/image/testdata"

	// DDS: "DDS " + DDS_HEADER(124 bytes).
	dds := make([]byte, 128)
	copy(dds[0:4], "DDS ")
	binary.LittleEndian.PutUint32(dds[4:], 124)         // dwSize
	binary.LittleEndian.PutUint32(dds[8:], 0x1|0x2|0x4) // flags
	binary.LittleEndian.PutUint32(dds[16:], 32)         // dwHeight
	binary.LittleEndian.PutUint32(dds[20:], 64)         // dwWidth
	writeFile(dir, "minimal.dds", dds)

	// KTX: 12-byte magic + header.
	ktx := make([]byte, 64)
	copy(ktx[0:12], []byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x31, 0x31, 0xBB, 0x0D, 0x0A, 0x1A, 0x0A})
	binary.LittleEndian.PutUint32(ktx[12:], 0x04030201) // endianness
	binary.LittleEndian.PutUint32(ktx[36:], 128)        // pixelWidth
	binary.LittleEndian.PutUint32(ktx[40:], 64)         // pixelHeight
	writeFile(dir, "minimal.ktx", ktx)

	// PSD: "8BPS" + header.
	psd := make([]byte, 26)
	copy(psd[0:4], "8BPS")
	binary.BigEndian.PutUint16(psd[4:6], 1)     // version
	binary.BigEndian.PutUint16(psd[12:14], 3)   // channels
	binary.BigEndian.PutUint32(psd[14:18], 200) // height
	binary.BigEndian.PutUint32(psd[18:22], 100) // width
	binary.BigEndian.PutUint16(psd[22:24], 8)   // bits per channel
	binary.BigEndian.PutUint16(psd[24:26], 3)   // color mode RGB
	writeFile(dir, "minimal.psd", psd)

	// EXR: magic bytes + padding.
	exr := make([]byte, 68)
	copy(exr[0:4], []byte{0x76, 0x2F, 0x31, 0x01})
	writeFile(dir, "minimal.exr", exr)

	// TGA: 2×2 uncompressed 24-bit BGR, top-left origin.
	w, h, bpp := 2, 2, 3
	tga := make([]byte, 18+w*h*bpp)
	tga[2] = 2 // type: uncompressed RGB
	binary.LittleEndian.PutUint16(tga[12:14], uint16(w))
	binary.LittleEndian.PutUint16(tga[14:16], uint16(h))
	tga[16] = 24   // bits per pixel
	tga[17] = 0x20 // origin: top-left
	for i := range w * h {
		off := 18 + i*bpp
		tga[off] = 255 // B
		tga[off+1] = 0 // G
		tga[off+2] = 0 // R
	}
	writeFile(dir, "blue_2x2.tga", tga)

	// TGA: 2×2 uncompressed 32-bit BGRA.
	tga32 := make([]byte, 18+w*h*4)
	tga32[2] = 2
	binary.LittleEndian.PutUint16(tga32[12:14], uint16(w))
	binary.LittleEndian.PutUint16(tga32[14:16], uint16(h))
	tga32[16] = 32
	tga32[17] = 0x20
	for i := range w * h {
		off := 18 + i*4
		tga32[off] = 255   // B
		tga32[off+1] = 0   // G
		tga32[off+2] = 0   // R
		tga32[off+3] = 255 // A
	}
	writeFile(dir, "blue_2x2_32.tga", tga32)
}

// --- Scene ---

func genSceneTestdata() {
	dir := "decode/model/scene/testdata"

	// Alembic Ogawa: minimal valid archive with root group.
	abc := make([]byte, 256)
	copy(abc[0:5], "Ogawa")
	abc[5] = 0xFF
	binary.LittleEndian.PutUint16(abc[6:8], 1)   // version
	binary.LittleEndian.PutUint64(abc[8:16], 16) // root group position

	// Root group at 16: 6 children (all Data, pointing to offset 128).
	binary.LittleEndian.PutUint64(abc[16:], 6)
	dataFlag := uint64(1) << 63
	for i := range 6 {
		binary.LittleEndian.PutUint64(abc[24+i*8:], dataFlag|128)
	}
	binary.LittleEndian.PutUint64(abc[128:], 0) // data size=0
	writeFile(dir, "minimal.abc", abc)

	// Ogawa with embedded mesh (vertex array).
	abcMesh := make([]byte, 512)
	copy(abcMesh[0:5], "Ogawa")
	abcMesh[5] = 0xFF
	binary.LittleEndian.PutUint16(abcMesh[6:8], 1)
	binary.LittleEndian.PutUint64(abcMesh[8:16], 16)

	// Root group at 16: 6 children.
	binary.LittleEndian.PutUint64(abcMesh[16:], 6)
	childStart := 24
	// Children 0,1: version data at 200.
	binary.LittleEndian.PutUint64(abcMesh[childStart:], dataFlag|200)
	binary.LittleEndian.PutUint64(abcMesh[childStart+8:], dataFlag|200)
	// Child 2: root object group at 128.
	binary.LittleEndian.PutUint64(abcMesh[childStart+16:], 128)
	// Children 3-5: data at 200.
	for i := 3; i < 6; i++ {
		binary.LittleEndian.PutUint64(abcMesh[childStart+i*8:], dataFlag|200)
	}

	// Version data at 200: size=4, value=1000.
	binary.LittleEndian.PutUint64(abcMesh[200:], 4)
	binary.LittleEndian.PutUint32(abcMesh[208:], 1000)

	// Object group at 128: 3 children.
	binary.LittleEndian.PutUint64(abcMesh[128:], 3)
	binary.LittleEndian.PutUint64(abcMesh[136:], 256)          // compound props group
	binary.LittleEndian.PutUint64(abcMesh[144:], dataFlag|200) // sub-objects
	binary.LittleEndian.PutUint64(abcMesh[152:], dataFlag|280) // child headers

	// Object name at 280: size=12, nameLen=4, "Cube".
	binary.LittleEndian.PutUint64(abcMesh[280:], 12)
	binary.LittleEndian.PutUint32(abcMesh[288:], 4)
	copy(abcMesh[292:296], "Cube")

	// Compound properties at 256: 1 Data child with float32 vertex array.
	binary.LittleEndian.PutUint64(abcMesh[256:], 1)
	binary.LittleEndian.PutUint64(abcMesh[264:], dataFlag|320)

	// Vertex data at 320: 3 verts × 3 floats × 4 bytes = 36.
	binary.LittleEndian.PutUint64(abcMesh[320:], 36)
	verts := [][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	for i, v := range verts {
		off := 328 + i*12
		binary.LittleEndian.PutUint32(abcMesh[off:], math.Float32bits(v[0]))
		binary.LittleEndian.PutUint32(abcMesh[off+4:], math.Float32bits(v[1]))
		binary.LittleEndian.PutUint32(abcMesh[off+8:], math.Float32bits(v[2]))
	}
	writeFile(dir, "cube.abc", abcMesh)
}

// --- Audio ---

func genAudioTestdata() {
	dir := "decode/audio/testdata"

	// OGG: "OggS" + basic page header.
	ogg := make([]byte, 64)
	copy(ogg[0:4], "OggS")
	ogg[4] = 0 // version
	ogg[5] = 2 // BOS
	writeFile(dir, "minimal.ogg", ogg)

	// MP3: ID3v2 header.
	mp3 := make([]byte, 64)
	copy(mp3[0:3], "ID3")
	mp3[3] = 4 // version
	writeFile(dir, "minimal.mp3", mp3)

	// FLAC: "fLaC" + STREAMINFO block.
	flac := make([]byte, 64)
	copy(flac[0:4], "fLaC")
	flac[4] = 0x80 // last block + type 0
	flac[7] = 34   // block len
	writeFile(dir, "minimal.flac", flac)
}
