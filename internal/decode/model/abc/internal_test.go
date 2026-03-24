package abc

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/gophics/ravenporter/ir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers to build synthetic ogawa binary data ---

func putU64LE(buf []byte, off int, v uint64) {
	binary.LittleEndian.PutUint64(buf[off:], v)
}

func putU32LE(buf []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(buf[off:], v)
}

func putF32LE(buf []byte, off int, v float32) {
	binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
}

func putF64LE(buf []byte, off int, v float64) {
	binary.LittleEndian.PutUint64(buf[off:], math.Float64bits(v))
}

// makeDataChild creates a data child reference to the given position.
func makeDataChild(pos int) uint64 { return ogawaDataFlag | uint64(pos) }

// buildDataNode builds a raw data node at the given offset in buf.
// Returns new offset after the node.
func buildDataNode(buf []byte, offset int, payload []byte) int { //nolint:unparam // layout structure
	putU64LE(buf, offset, uint64(len(payload)))
	copy(buf[offset+ogawaU64Size:], payload)
	return offset + ogawaU64Size + len(payload)
}

// --- triangulateAlembicFaces ---

func TestTriangulateAlembicFaces(t *testing.T) {
	tests := []struct {
		name       string
		indices    []int32
		counts     []int32
		wantLen    int
		wantFirst3 []uint32
	}{
		{"Empty", nil, nil, 0, nil},
		{"EmptyIndices", []int32{}, nil, 0, nil},
		{"NoFaceCounts_Passthrough", []int32{0, 1, 2, 3}, nil, 4, []uint32{0, 1, 2, 3}},
		{"SingleTriangle", []int32{0, 1, 2}, []int32{3}, 3, []uint32{0, 1, 2}},
		{"Quad", []int32{0, 1, 2, 3}, []int32{4}, 6, []uint32{0, 1, 2}},
		{"TwoTriangles", []int32{0, 1, 2, 3, 4, 5}, []int32{3, 3}, 6, []uint32{0, 1, 2}},
		{"DegenerateFace", []int32{0, 1}, []int32{2}, 0, nil},
		{"NegativeIndex_Passthrough", []int32{0, -1, 2}, nil, 2, nil},
		{"Pentagon", []int32{0, 1, 2, 3, 4}, []int32{5}, 9, []uint32{0, 1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := triangulateAlembicFaces(tt.indices, tt.counts)
			assert.Len(t, result, tt.wantLen)
			if tt.wantFirst3 != nil && len(result) >= 3 {
				assert.Equal(t, tt.wantFirst3[:3], result[:3])
			}
		})
	}
}

// --- looksLikeVertexData ---

func TestLooksLikeVertexData(t *testing.T) {
	makeFloatBytes := func(vals ...float32) []byte {
		buf := make([]byte, len(vals)*4)
		for i, v := range vals {
			putF32LE(buf, i*4, v)
		}
		return buf
	}

	tests := []struct {
		name   string
		data   []byte
		offset int
		count  int
		want   bool
	}{
		{"ValidVertices", makeFloatBytes(1.0, 2.0, -3.0), 0, 3, true},
		{"ContainsNaN", makeFloatBytes(float32(math.NaN()), 1.0, 2.0), 0, 3, false},
		{"ContainsInf", makeFloatBytes(float32(math.Inf(1)), 1.0, 2.0), 0, 3, false},
		{"OutOfRange", makeFloatBytes(2e10, 1.0, 2.0), 0, 3, false},
		{"NegativeInf", makeFloatBytes(float32(math.Inf(-1)), 1.0, 2.0), 0, 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, looksLikeVertexData(tt.data, tt.offset, tt.count))
		})
	}
}

// --- buildXformAnimation ---

func TestBuildXformAnimationChannels(t *testing.T) {
	identity := [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	matrices := [][16]float32{identity, identity}
	anim := buildXformAnimation(0, matrices, 0, 1.0/24)
	assert.Equal(t, xformAnimName, anim.Name)
	assert.Len(t, anim.Channels, 3)
	assert.Equal(t, ir.TargetTranslation, anim.Channels[0].Target)
	assert.Equal(t, ir.TargetRotation, anim.Channels[1].Target)
	assert.Equal(t, ir.TargetScale, anim.Channels[2].Target)
	assert.Len(t, anim.Channels[0].Times, 2)
}

// --- childAddr, isGroup, isData ---

func TestChildAddressHelpers(t *testing.T) {
	assert.True(t, isData(ogawaDataFlag|42))
	assert.False(t, isGroup(ogawaDataFlag|42))
	assert.True(t, isGroup(42))
	assert.False(t, isData(42))

	assert.Equal(t, 42, childAddr(42))
	assert.Equal(t, 42, childAddr(ogawaDataFlag|42))
	assert.Equal(t, 0, childAddr(0))
}

// --- versionString ---

func TestVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		want    string
	}{
		{"1.0", abcVersionDiv, "1.0"},
		{"1.5", abcVersionDiv + 5, "1.5"},
		{"0.0", 0, "0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, versionString(tt.version))
		})
	}
}

// --- parseMetadata ---

func TestParseMetadata(t *testing.T) {
	p := &ogawaParser{asset: &ir.Asset{}}
	p.parseMetadata("upAxis=Z;other=val")
	assert.Equal(t, ir.ZUp, p.asset.UpAxis)

	p2 := &ogawaParser{asset: &ir.Asset{}}
	p2.parseMetadata("upAxis=Y")
	assert.NotEqual(t, ir.ZUp, p2.asset.UpAxis)

	p3 := &ogawaParser{asset: &ir.Asset{}}
	p3.parseMetadata("noequals")
	assert.NotEqual(t, ir.ZUp, p3.asset.UpAxis)
}

// --- readDataPayload ---

func TestReadDataPayload(t *testing.T) {
	buf := make([]byte, 64)
	// Build data node at offset 16: size=4, followed by [0xDE, 0xAD, 0xBE, 0xEF]
	putU64LE(buf, 16, 4)
	buf[24] = 0xDE
	buf[25] = 0xAD
	buf[26] = 0xBE
	buf[27] = 0xEF

	p := &ogawaParser{data: buf}

	t.Run("ValidData", func(t *testing.T) {
		result := p.readDataPayload(makeDataChild(16))
		assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, result)
	})

	t.Run("InvalidPos", func(t *testing.T) {
		assert.Nil(t, p.readDataPayload(makeDataChild(0)))
	})

	t.Run("ZeroSize", func(t *testing.T) {
		putU64LE(buf, 32, 0) // size=0
		assert.Nil(t, p.readDataPayload(makeDataChild(32)))
	})
}

// --- readPropertyData ---

func TestReadPropertyData(t *testing.T) {
	buf := make([]byte, 128)

	// Data node at offset 16: size=3, payload "abc"
	putU64LE(buf, 16, 3)
	copy(buf[24:], "abc")

	p := &ogawaParser{data: buf}

	t.Run("DataChild", func(t *testing.T) {
		result := p.readPropertyData(makeDataChild(16))
		assert.Equal(t, []byte("abc"), result)
	})

	t.Run("GroupChild_WithSubData", func(t *testing.T) {
		// Group at offset 40: 2 children
		putU64LE(buf, 40, 2)
		// child[0] = group (skip)
		putU64LE(buf, 48, 0)
		// child[1] = data ref to offset 16
		putU64LE(buf, 56, makeDataChild(16))

		result := p.readPropertyData(40) // group child
		assert.Equal(t, []byte("abc"), result)
	})

	t.Run("GroupChild_TooFewSubs", func(t *testing.T) {
		// Group at offset 80: 1 child only
		putU64LE(buf, 80, 1)
		putU64LE(buf, 88, 0)

		result := p.readPropertyData(80)
		assert.Nil(t, result)
	})
}

// --- readSingleMatrix ---

func TestReadSingleMatrix(t *testing.T) {
	buf := make([]byte, 256)
	// Build data node at offset 16: 128 bytes payload (16 * 8 = 128 for 16 float64s)
	putU64LE(buf, 16, xformByteSize)
	for i := range xformF64Count {
		putF64LE(buf, 24+i*f64Size, float64(i+1))
	}

	p := &ogawaParser{data: buf}

	mat, ok := p.readSingleMatrix(makeDataChild(16))
	assert.True(t, ok)
	assert.Equal(t, float32(1), mat[0])
	assert.Equal(t, float32(16), mat[15])

	// Too short
	putU64LE(buf, 200, 8) // only 8 bytes, need 128
	mat2, ok2 := p.readSingleMatrix(makeDataChild(200))
	assert.False(t, ok2)
	assert.Equal(t, [16]float32{}, mat2)
}

// --- readF64Property ---

func TestReadF64Property(t *testing.T) {
	buf := make([]byte, 128)
	// Data node at offset 16: 8 bytes payload = one float64
	putU64LE(buf, 16, f64Size)
	putF64LE(buf, 24, 3.14)

	p := &ogawaParser{data: buf}
	children := []uint64{0, makeDataChild(16)} // index 0 is unused, index 1 = data

	t.Run("Found", func(t *testing.T) {
		props := map[string]int{"focal": 0}
		v := p.readF64Property(children, props, "focal")
		assert.InDelta(t, 3.14, v, 0.001)
	})

	t.Run("NotFound", func(t *testing.T) {
		props := map[string]int{}
		v := p.readF64Property(children, props, "missing")
		assert.Equal(t, 0.0, v)
	})

	t.Run("OutOfBounds", func(t *testing.T) {
		props := map[string]int{"big": 99}
		v := p.readF64Property(children, props, "big")
		assert.Equal(t, 0.0, v)
	})
}

// --- readBinaryArray (via readVec3Array, readInt32Array, etc.) ---

func TestReadVec3Array(t *testing.T) {
	buf := make([]byte, 128)
	// Data node at offset 16: 3 floats = 1 vec3
	putU64LE(buf, 16, vec3Stride)
	putF32LE(buf, 24, 1.0)
	putF32LE(buf, 28, 2.0)
	putF32LE(buf, 32, 3.0)

	p := &ogawaParser{data: buf}
	result := p.readVec3Array(makeDataChild(16))
	require.Len(t, result, 1)
	assert.Equal(t, [3]float32{1.0, 2.0, 3.0}, result[0])
}

func TestReadVec2Array(t *testing.T) {
	buf := make([]byte, 64)
	putU64LE(buf, 16, vec2Stride)
	putF32LE(buf, 24, 0.5)
	putF32LE(buf, 28, 0.75)

	p := &ogawaParser{data: buf}
	result := p.readVec2Array(makeDataChild(16))
	require.Len(t, result, 1)
	assert.Equal(t, [2]float32{0.5, 0.75}, result[0])
}

func TestReadVec4Array(t *testing.T) {
	buf := make([]byte, 64)
	putU64LE(buf, 16, vec4Stride)
	putF32LE(buf, 24, 1.0)
	putF32LE(buf, 28, 0.5)
	putF32LE(buf, 32, 0.0)
	putF32LE(buf, 36, 1.0)

	p := &ogawaParser{data: buf}
	result := p.readVec4Array(makeDataChild(16))
	require.Len(t, result, 1)
	assert.Equal(t, [4]float32{1.0, 0.5, 0.0, 1.0}, result[0])
}

func TestReadInt32Array(t *testing.T) {
	buf := make([]byte, 48)
	putU64LE(buf, 16, i32Size*2)
	putU32LE(buf, 24, 42)
	putU32LE(buf, 28, 7)

	p := &ogawaParser{data: buf}
	result := p.readInt32Array(makeDataChild(16))
	require.Len(t, result, 2)
	assert.Equal(t, int32(42), result[0])
	assert.Equal(t, int32(7), result[1])
}

// --- readPropertyNamesFromData ---

func TestReadPropertyNamesFromData(t *testing.T) {
	buf := make([]byte, 128)

	// name "P" (1 byte) + null separator + name "N" (1 byte)
	// Each name is: u32 nameLen, then bytes, then null byte
	payload := make([]byte, 0, 32)

	// name 0: "P" (len=1)
	payload = binary.LittleEndian.AppendUint32(payload, 1)
	payload = append(payload, 'P', 0)

	// name 1: "N" (len=1)
	payload = binary.LittleEndian.AppendUint32(payload, 1)
	payload = append(payload, 'N', 0)

	buildDataNode(buf, 16, payload)

	p := &ogawaParser{data: buf}
	result := make(map[string]int)
	got := p.readPropertyNamesFromData(makeDataChild(16), result)
	assert.Equal(t, 0, got["P"])
	assert.Equal(t, 1, got["N"])
}

// --- readCompoundPropertyNames ---

func TestReadCompoundPropertyNames(t *testing.T) {
	buf := make([]byte, 128)

	payload := make([]byte, 0, 32)
	payload = binary.LittleEndian.AppendUint32(payload, 1)
	payload = append(payload, 'P', 0)

	buildDataNode(buf, 16, payload)

	p := &ogawaParser{data: buf}

	t.Run("DataChild", func(t *testing.T) {
		children := []uint64{makeDataChild(16)}
		result := p.readCompoundPropertyNames(children)
		assert.Contains(t, result, "P")
	})

	t.Run("Empty", func(t *testing.T) {
		result := p.readCompoundPropertyNames(nil)
		assert.Nil(t, result)
	})
}

// --- tryExtractMaterialRef ---

func TestTryExtractMaterialRef(t *testing.T) {
	buf := make([]byte, 128)
	// Data node at offset 16 with material name "wood\x00"
	buildDataNode(buf, 16, []byte("wood\x00"))

	p := &ogawaParser{data: buf, asset: &ir.Asset{}}
	children := []uint64{0, makeDataChild(16)}
	props := map[string]int{abcPropArbGeom: 0}

	idx := p.tryExtractMaterialRef(children, props)
	assert.Equal(t, 0, idx)
	require.Len(t, p.asset.Materials, 1)
	assert.Equal(t, "wood", p.asset.Materials[0].Name)

	t.Run("NotFound", func(t *testing.T) {
		p2 := &ogawaParser{data: buf, asset: &ir.Asset{}}
		idx := p2.tryExtractMaterialRef(children, map[string]int{})
		assert.Equal(t, ir.NoIndex, idx)
	})
}

// --- addMeshNode ---

func TestAddMeshNode(t *testing.T) {
	p := &ogawaParser{asset: &ir.Asset{}}
	positions := [][3]float32{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	normals := [][3]float32{{0, 1, 0}, {0, 1, 0}, {0, 1, 0}}
	uvs := [][2]float32{{0, 0}, {1, 0}, {0, 1}}
	colors := [][4]float32{{1, 0, 0, 1}, {0, 1, 0, 1}, {0, 0, 1, 1}}
	indices := []uint32{0, 1, 2}

	p.addMeshNode(positions, normals, uvs, colors, indices)

	require.Len(t, p.asset.Meshes, 1)
	require.Len(t, p.asset.Nodes, 1)
	assert.Equal(t, 3, p.asset.Meshes[0].Primitives[0].Data.VertexCount)
	assert.NotNil(t, p.asset.Meshes[0].Primitives[0].Data.Normals)
	assert.NotNil(t, p.asset.Meshes[0].Primitives[0].Data.TexCoord0)
	assert.NotNil(t, p.asset.Meshes[0].Primitives[0].Data.Colors0)
}

// --- readObjectName ---

func TestReadObjectName(t *testing.T) {
	buf := make([]byte, 64)
	// Data node at offset 16: name "Cube"
	namePayload := make([]byte, 0, 16)
	namePayload = binary.LittleEndian.AppendUint32(namePayload, 4)
	namePayload = append(namePayload, "Cube"...)
	buildDataNode(buf, 16, namePayload)

	p := &ogawaParser{data: buf}
	name := p.readObjectName(makeDataChild(16))
	assert.Equal(t, "Cube", name)

	t.Run("Invalid", func(t *testing.T) {
		name := p.readObjectName(makeDataChild(0))
		assert.Equal(t, defaultObjName, name)
	})
}

// --- readPropertyName ---

func TestReadPropertyName(t *testing.T) {
	buf := make([]byte, 64)
	namePayload := make([]byte, 0, 16)
	namePayload = binary.LittleEndian.AppendUint32(namePayload, 2)
	namePayload = append(namePayload, "uv"...)
	buildDataNode(buf, 16, namePayload)

	p := &ogawaParser{data: buf}
	name := p.readPropertyName(makeDataChild(16))
	assert.Equal(t, "uv", name)

	t.Run("Invalid", func(t *testing.T) {
		name := p.readPropertyName(makeDataChild(0))
		assert.Equal(t, "", name)
	})

	t.Run("TooShort", func(t *testing.T) {
		putU64LE(buf, 40, 2) // size=2, too small for i32Size name length
		name := p.readPropertyName(makeDataChild(40))
		assert.Equal(t, "", name)
	})
}

// --- readXformSamples ---

func TestReadXformSamples(t *testing.T) {
	buf := make([]byte, 512)
	// Build data node at offset 16: identity matrix (16 float64s)
	matPayload := make([]byte, xformByteSize)
	for i := range xformF64Count {
		if i%5 == 0 { // set diagonal to 1.0
			putF64LE(matPayload, i*f64Size, 1.0)
		}
	}
	buildDataNode(buf, 16, matPayload)

	p := &ogawaParser{data: buf}

	t.Run("SingleMatrixFromData", func(t *testing.T) {
		mats, ok := p.readXformSamples(makeDataChild(16))
		assert.True(t, ok)
		require.Len(t, mats, 1)
	})

	t.Run("NotGroupNotData", func(t *testing.T) {
		mats, ok := p.readXformSamples(0) // zero is degenerate
		assert.False(t, ok)
		assert.Nil(t, mats)
	})
}

func TestExtractCameraFromNamedProps(t *testing.T) {
	buf := make([]byte, 4096)

	// Build data nodes for focal length, vertical aperture, near, far
	focalOff := 100
	putU64LE(buf, focalOff, f64Size)
	putF64LE(buf, focalOff+ogawaU64Size, 50.0)

	vertApOff := 200
	putU64LE(buf, vertApOff, f64Size)
	putF64LE(buf, vertApOff+ogawaU64Size, 24.0)

	nearOff := 300
	putU64LE(buf, nearOff, f64Size)
	putF64LE(buf, nearOff+ogawaU64Size, 0.1)

	farOff := 400
	putU64LE(buf, farOff, f64Size)
	putF64LE(buf, farOff+ogawaU64Size, 1000.0)

	children := []uint64{
		0,                       // index 0: placeholder for focalLength name
		makeDataChild(focalOff), // index 1: focalLength data (props["focalLength"]=0, childIdx=0+1=1)
		0,                       // index 2: placeholder for verticalAperture name
		makeDataChild(vertApOff),
		0,
		makeDataChild(nearOff),
		0,
		makeDataChild(farOff),
	}

	props := map[string]int{
		abcPropFocalLen: 0,
		abcPropVertAp:   2,
		abcPropNearClip: 4,
		abcPropFarClip:  6,
	}

	p := &ogawaParser{data: buf, asset: &ir.Asset{}}
	p.extractCameraFromNamedProps(children, props)

	require.Len(t, p.asset.Cameras, 1)
	cam := p.asset.Cameras[0]
	assert.Equal(t, "AlembicCamera", cam.Name)
	require.NotNil(t, cam.Perspective)
	assert.True(t, cam.Perspective.FOV > 0)
	assert.True(t, cam.Perspective.Near > 0)
}

func TestExtractPolyMeshFromNamedProps(t *testing.T) {
	buf := make([]byte, 8192)

	// Build Vec3f positions at offset 100
	posOff := 100
	posPayload := make([]byte, 3*3*4) // 3 vertices × 3 components × 4 bytes
	// Vertex 0: (0,0,0)
	binary.LittleEndian.PutUint32(posPayload[0:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(posPayload[4:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(posPayload[8:], math.Float32bits(0))
	// Vertex 1: (1,0,0)
	binary.LittleEndian.PutUint32(posPayload[12:], math.Float32bits(1))
	binary.LittleEndian.PutUint32(posPayload[16:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(posPayload[20:], math.Float32bits(0))
	// Vertex 2: (0,1,0)
	binary.LittleEndian.PutUint32(posPayload[24:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(posPayload[28:], math.Float32bits(1))
	binary.LittleEndian.PutUint32(posPayload[32:], math.Float32bits(0))
	putU64LE(buf, posOff, uint64(len(posPayload)))
	copy(buf[posOff+ogawaU64Size:], posPayload)

	// Build face indices at offset 300 (3 int32s)
	idxOff := 300
	idxPayload := make([]byte, 3*4)
	binary.LittleEndian.PutUint32(idxPayload[0:], 0)
	binary.LittleEndian.PutUint32(idxPayload[4:], 1)
	binary.LittleEndian.PutUint32(idxPayload[8:], 2)
	putU64LE(buf, idxOff, uint64(len(idxPayload)))
	copy(buf[idxOff+ogawaU64Size:], idxPayload)

	// Build face counts at offset 500 (1 int32 = 3)
	faceCountOff := 500
	faceCountPayload := make([]byte, 4)
	binary.LittleEndian.PutUint32(faceCountPayload[0:], 3)
	putU64LE(buf, faceCountOff, uint64(len(faceCountPayload)))
	copy(buf[faceCountOff+ogawaU64Size:], faceCountPayload)

	children := []uint64{
		0,                           // index 0: P name
		makeDataChild(posOff),       // index 1: P data
		0,                           // index 2: .faceIndices name
		makeDataChild(idxOff),       // index 3: faceIndices data
		0,                           // index 4: .faceCounts name
		makeDataChild(faceCountOff), // index 5: faceCounts data
	}

	props := map[string]int{
		abcPropP:          0,
		abcPropFaceIdx:    2,
		abcPropFaceCounts: 4,
	}

	p := &ogawaParser{data: buf, asset: &ir.Asset{}}
	p.extractPolyMeshFromNamedProps(children, props)

	require.Len(t, p.asset.Meshes, 1)
	mesh := p.asset.Meshes[0]
	require.Len(t, mesh.Primitives, 1)
	assert.Len(t, mesh.Primitives[0].Data.Positions, 3)
}

func TestReadPropertyNamesFromGroup(t *testing.T) {
	buf := make([]byte, 4096)

	// Create a data child that holds a property name:
	// Size (8 bytes), nameLen (4 bytes), name (N bytes)
	nameOff := 100
	name := "myProp"
	nameLen := len(name)
	payloadSize := 4 + nameLen
	putU64LE(buf, nameOff, uint64(payloadSize))
	putU32LE(buf, nameOff+ogawaU64Size, uint32(nameLen))
	copy(buf[nameOff+ogawaU64Size+i32Size:], name)

	dataChild := makeDataChild(nameOff)

	// Create a group child holding the data child
	groupOff := 200
	putU64LE(buf, groupOff, 1) // write 1 child
	putU64LE(buf, groupOff+ogawaU64Size, dataChild)

	p := &ogawaParser{data: buf}
	result := make(map[string]int)

	groupToken := uint64(groupOff) // Group tokens are just offsets without data flag
	res := p.readPropertyNamesFromGroup(groupToken, result)

	assert.Len(t, res, 1)
	assert.Equal(t, 0, res["myProp"])
}

func TestTryExtractMeshFromProperties(t *testing.T) {
	buf := make([]byte, 4096)

	// Data payload representing vertices
	// 3 vertices, each 3 float32 = 36 bytes
	posOff := 100
	posPayload := make([]byte, 36)
	putU64LE(buf, posOff, 36)
	copy(buf[posOff+ogawaU64Size:], posPayload)
	dataChild := makeDataChild(posOff)

	// A group to hold the data child
	innerGroupOff := 200
	putU64LE(buf, innerGroupOff, 1)
	putU64LE(buf, innerGroupOff+ogawaU64Size, dataChild)
	innerGroupChild := uint64(innerGroupOff)

	// Outer group holding the inner group
	outerGroupOff := 300
	putU64LE(buf, outerGroupOff, 1)
	putU64LE(buf, outerGroupOff+ogawaU64Size, innerGroupChild)

	p := &ogawaParser{data: buf, asset: &ir.Asset{}}

	// tryExtractMeshFromProperties will traverse outer -> inner -> data -> tryExtractFromDataChild -> tryParseVertexArray
	p.tryExtractMeshFromProperties(outerGroupOff, 0)

	require.Len(t, p.asset.Meshes, 1)
	assert.Len(t, p.asset.Meshes[0].Primitives[0].Data.Positions, 3)
}
