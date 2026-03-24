//go:build ignore

package main

import (
	"archive/zip"
	"encoding/binary"
	"math"
	"os"
)

func main() {
	genComprehensiveUSDC()
	genTestUSDZ()
}

func genComprehensiveUSDC() {
	tokens := []string{
		"typeName", "Mesh", "Camera", "DistantLight", "SphereLight",
		"DiskLight", "Xform", "points", "faceVertexIndices", "normals",
		"primvars:st", "faceVertexCounts", "focalLength", "horizontalAperture",
		"verticalAperture", "clippingRange", "inputs:color", "inputs:intensity",
		"inputs:shaping:cone:angle", "xformOp:translate", "xformOp:scale",
		"Triangle", "Quad", "MainCam", "Sun", "Lamp", "Spot", "World",
	}

	type field struct {
		tokenIdx uint32
		valueRep uint64
	}

	tokIdx := func(name string) uint32 {
		for i, t := range tokens {
			if t == name {
				return uint32(i)
			}
		}
		return 0
	}

	var buf []byte
	header := make([]byte, 88)
	copy(header, "PXR-USDC")
	header[8] = 0  // version major
	header[9] = 4  // version minor
	header[10] = 0 // version patch
	buf = append(buf, header...)

	appendF32Array := func(vals []float32) int {
		off := len(buf)
		b := make([]byte, 8+len(vals)*4)
		binary.LittleEndian.PutUint64(b, uint64(len(vals)/3))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[8+i*4:], math.Float32bits(v))
		}
		buf = append(buf, b...)
		return off
	}

	appendU32Array := func(vals []uint32) int {
		off := len(buf)
		b := make([]byte, 8+len(vals)*4)
		binary.LittleEndian.PutUint64(b, uint64(len(vals)))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[8+i*4:], v)
		}
		buf = append(buf, b...)
		return off
	}

	appendI32Array := func(vals []int32) int {
		off := len(buf)
		b := make([]byte, 8+len(vals)*4)
		binary.LittleEndian.PutUint64(b, uint64(len(vals)))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[8+i*4:], uint32(v))
		}
		buf = append(buf, b...)
		return off
	}

	appendVec2fArray := func(vals [][2]float32) int {
		off := len(buf)
		b := make([]byte, 8+len(vals)*8)
		binary.LittleEndian.PutUint64(b, uint64(len(vals)))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[8+i*8:], math.Float32bits(v[0]))
			binary.LittleEndian.PutUint32(b[8+i*8+4:], math.Float32bits(v[1]))
		}
		buf = append(buf, b...)
		return off
	}

	mkVR := func(offset int) uint64 {
		return uint64(offset) << 8
	}
	mkInlineF32 := func(v float32) uint64 {
		return uint64(math.Float32bits(v)) << 8
	}
	mkInlineTok := func(name string) uint64 {
		return uint64(tokIdx(name)) << 8
	}

	// Triangle mesh data
	triPtsOff := appendF32Array([]float32{0, 0, 0, 1, 0, 0, 0, 1, 0})
	triIdxOff := appendU32Array([]uint32{0, 1, 2})
	triNrmOff := appendF32Array([]float32{0, 0, 1, 0, 0, 1, 0, 0, 1})
	triUVOff := appendVec2fArray([][2]float32{{0, 0}, {1, 0}, {0.5, 1}})

	// Quad mesh data
	quadPtsOff := appendF32Array([]float32{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0})
	quadIdxOff := appendU32Array([]uint32{0, 1, 2, 3})
	quadFcOff := appendI32Array([]int32{4})

	// Camera clip range
	camClipOff := appendVec2fArray([][2]float32{{0.1, 1000}})

	// Light colors
	sunColorOff := appendF32Array([]float32{1, 0.95, 0.8})
	lampColorOff := appendF32Array([]float32{1, 1, 1})
	spotColorOff := appendF32Array([]float32{0.8, 0.9, 1})

	// Build fields
	fields := []field{
		// 0: typeName=Mesh
		{tokIdx("typeName"), mkInlineTok("Mesh")},
		// 1: points (tri)
		{tokIdx("points"), mkVR(triPtsOff)},
		// 2: faceVertexIndices (tri)
		{tokIdx("faceVertexIndices"), mkVR(triIdxOff)},
		// 3: normals (tri)
		{tokIdx("normals"), mkVR(triNrmOff)},
		// 4: primvars:st (tri)
		{tokIdx("primvars:st"), mkVR(triUVOff)},
		// 5: typeName=Mesh (quad)
		{tokIdx("typeName"), mkInlineTok("Mesh")},
		// 6: points (quad)
		{tokIdx("points"), mkVR(quadPtsOff)},
		// 7: faceVertexIndices (quad)
		{tokIdx("faceVertexIndices"), mkVR(quadIdxOff)},
		// 8: faceVertexCounts (quad)
		{tokIdx("faceVertexCounts"), mkVR(quadFcOff)},
		// 9: typeName=Camera
		{tokIdx("typeName"), mkInlineTok("Camera")},
		// 10: focalLength=35
		{tokIdx("focalLength"), mkInlineF32(35)},
		// 11: horizontalAperture=36
		{tokIdx("horizontalAperture"), mkInlineF32(36)},
		// 12: verticalAperture=24
		{tokIdx("verticalAperture"), mkInlineF32(24)},
		// 13: clippingRange
		{tokIdx("clippingRange"), mkVR(camClipOff)},
		// 14: typeName=DistantLight
		{tokIdx("typeName"), mkInlineTok("DistantLight")},
		// 15: inputs:intensity=5
		{tokIdx("inputs:intensity"), mkInlineF32(5)},
		// 16: inputs:color (sun)
		{tokIdx("inputs:color"), mkVR(sunColorOff)},
		// 17: typeName=SphereLight
		{tokIdx("typeName"), mkInlineTok("SphereLight")},
		// 18: inputs:intensity=100
		{tokIdx("inputs:intensity"), mkInlineF32(100)},
		// 19: inputs:color (lamp)
		{tokIdx("inputs:color"), mkVR(lampColorOff)},
		// 20: typeName=DiskLight
		{tokIdx("typeName"), mkInlineTok("DiskLight")},
		// 21: inputs:intensity=200
		{tokIdx("inputs:intensity"), mkInlineF32(200)},
		// 22: inputs:color (spot)
		{tokIdx("inputs:color"), mkVR(spotColorOff)},
		// 23: inputs:shaping:cone:angle=30
		{tokIdx("inputs:shaping:cone:angle"), mkInlineF32(30)},
		// 24: typeName=Xform
		{tokIdx("typeName"), mkInlineTok("Xform")},
	}

	// Field sets: groups of field indices terminated by -1
	fieldSets := []int32{
		0, 1, 2, 3, 4, -1, // tri mesh (fields 0-4)
		5, 6, 7, 8, -1, // quad mesh (fields 5-8)
		9, 10, 11, 12, 13, -1, // camera (fields 9-13)
		14, 15, 16, -1, // distant light (fields 14-16)
		17, 18, 19, -1, // sphere light (fields 17-19)
		20, 21, 22, 23, -1, // disk light (fields 20-23)
		24, -1, // xform (field 24)
	}

	type path struct {
		tokenIdx, parentIdx, childIdx, siblingIdx int32
	}
	paths := []path{
		{int32(tokIdx("Triangle")), 6, -1, -1},
		{int32(tokIdx("Quad")), -1, -1, -1},
		{int32(tokIdx("MainCam")), -1, -1, -1},
		{int32(tokIdx("Sun")), -1, -1, -1},
		{int32(tokIdx("Lamp")), -1, -1, -1},
		{int32(tokIdx("Spot")), -1, -1, -1},
		{int32(tokIdx("World")), -1, 0, -1},
	}

	type spec struct {
		pathIdx, fieldSetIdx, specType int32
	}
	specs := []spec{
		{0, 0, 1},  // Triangle → field set starting at 0
		{1, 6, 1},  // Quad → field set starting at 6
		{2, 11, 1}, // MainCam → field set starting at 11
		{3, 17, 1}, // Sun → field set starting at 17
		{4, 21, 1}, // Lamp → field set starting at 21
		{5, 25, 1}, // Spot → field set starting at 25
		{6, 30, 1}, // World → field set starting at 30
	}

	// Write sections
	writeSection := func(name string, data []byte) (uint64, uint64) {
		off := uint64(len(buf))
		buf = append(buf, data...)
		return off, uint64(len(data))
	}

	// TOKENS section
	var tokData []byte
	tokB := make([]byte, 8)
	binary.LittleEndian.PutUint64(tokB, uint64(len(tokens)))
	tokData = append(tokData, tokB...)
	tokData = append(tokData, 0) // no compression
	for _, t := range tokens {
		tokData = append(tokData, []byte(t)...)
		tokData = append(tokData, 0)
	}
	tokOff, tokSize := writeSection("TOKENS", tokData)

	// STRINGS section (empty)
	strData := make([]byte, 4)
	binary.LittleEndian.PutUint32(strData, 0)
	strOff, strSize := writeSection("STRINGS", strData)

	// FIELDS section
	var fldData []byte
	fldB := make([]byte, 8)
	binary.LittleEndian.PutUint64(fldB, uint64(len(fields)))
	fldData = append(fldData, fldB...)
	fldData = append(fldData, 0) // no compression
	for _, f := range fields {
		b := make([]byte, 12)
		binary.LittleEndian.PutUint32(b, f.tokenIdx)
		binary.LittleEndian.PutUint64(b[4:], f.valueRep)
		fldData = append(fldData, b...)
	}
	fldOff, fldSize := writeSection("FIELDS", fldData)

	// FIELDSETS section
	var fsData []byte
	fsB := make([]byte, 8)
	binary.LittleEndian.PutUint64(fsB, uint64(len(fieldSets)))
	fsData = append(fsData, fsB...)
	for _, v := range fieldSets {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(v))
		fsData = append(fsData, b...)
	}
	fsOff, fsSize := writeSection("FIELDSETS", fsData)

	// PATHS section
	var pathData []byte
	pathB := make([]byte, 8)
	binary.LittleEndian.PutUint64(pathB, uint64(len(paths)))
	pathData = append(pathData, pathB...)
	for _, p := range paths {
		b := make([]byte, 16)
		binary.LittleEndian.PutUint32(b, uint32(p.tokenIdx))
		binary.LittleEndian.PutUint32(b[4:], uint32(p.parentIdx))
		binary.LittleEndian.PutUint32(b[8:], uint32(p.childIdx))
		binary.LittleEndian.PutUint32(b[12:], uint32(p.siblingIdx))
		pathData = append(pathData, b...)
	}
	pathOff, pathSize := writeSection("PATHS", pathData)

	// SPECS section
	var specData []byte
	specB := make([]byte, 8)
	binary.LittleEndian.PutUint64(specB, uint64(len(specs)))
	specData = append(specData, specB...)
	for _, s := range specs {
		b := make([]byte, 12)
		binary.LittleEndian.PutUint32(b, uint32(s.pathIdx))
		binary.LittleEndian.PutUint32(b[4:], uint32(s.fieldSetIdx))
		binary.LittleEndian.PutUint32(b[8:], uint32(s.specType))
		specData = append(specData, b...)
	}
	specOff, specSize := writeSection("SPECS", specData)

	// Write TOC
	tocOff := len(buf)
	binary.LittleEndian.PutUint64(buf[16:], uint64(tocOff))

	sections := []struct {
		name      string
		off, size uint64
	}{
		{"TOKENS", tokOff, tokSize},
		{"STRINGS", strOff, strSize},
		{"FIELDS", fldOff, fldSize},
		{"FIELDSETS", fsOff, fsSize},
		{"PATHS", pathOff, pathSize},
		{"SPECS", specOff, specSize},
	}

	numSec := make([]byte, 8)
	binary.LittleEndian.PutUint64(numSec, uint64(len(sections)))
	buf = append(buf, numSec...)

	for _, s := range sections {
		entry := make([]byte, 32)
		copy(entry, s.name)
		binary.LittleEndian.PutUint64(entry[16:], s.off)
		binary.LittleEndian.PutUint64(entry[24:], s.size)
		buf = append(buf, entry...)
	}

	os.WriteFile("comprehensive.usdc", buf, 0644)
}

func genTestUSDZ() {
	usda := []byte("#usda 1.0\ndef Mesh \"ZipMesh\" {\n    point3f[] points = [(0, 0, 0), (1, 0, 0), (0, 1, 0)]\n    int[] faceVertexIndices = [0, 1, 2]\n}\n")

	// Minimal 1x1 red PNG (67 bytes)
	png1x1 := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x36, 0x28, 0x19,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}

	f, _ := os.Create("test.usdz")
	defer f.Close()

	w := zip.NewWriter(f)
	fw, _ := w.Create("scene.usda")
	fw.Write(usda)
	tw, _ := w.Create("textures/albedo.png")
	tw.Write(png1x1)
	w.Close()
}

