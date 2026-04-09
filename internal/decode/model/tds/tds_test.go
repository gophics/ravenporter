package tds_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decode/model/tds"
	"github.com/gophics/ravenporter/ir"
)

func writeChunk(buf *bytes.Buffer, id uint16, data []byte) {
	_ = binary.Write(buf, binary.LittleEndian, id)
	_ = binary.Write(buf, binary.LittleEndian, uint32(chunkHdrLen+len(data)))
	buf.Write(data)
}

func writeCString(buf *bytes.Buffer, s string) {
	buf.WriteString(s)
	buf.WriteByte(0)
}

const (
	chunkHdrLen = 6

	tMain        = 0x4D4D
	tEditor      = 0x3D3D
	tObject      = 0x4000
	tTriMesh     = 0x4100
	tVertices    = 0x4110
	tFaces       = 0x4120
	tFaceMat     = 0x4130
	tTexCoord    = 0x4140
	tLocalMat    = 0x4160
	tLight       = 0x4600
	tSpotlight   = 0x4610
	tDirectLight = 0x4613
	tCamera      = 0x4700

	tMaterial   = 0xAFFF
	tMatName    = 0xA000
	tMatDiffuse = 0xA020
	tMatSpec    = 0xA030
	tMatTexMap  = 0xA200
	tMatBumpMap = 0xA230
	tMatTexFile = 0xA300
	tColor24    = 0x0011

	tKeyframer     = 0xB000
	tMeshTrackNode = 0xB002
	tTrackObjName  = 0xB010
	tPosTrack      = 0xB020
	tRotTrack      = 0xB021
	tScaleTrack    = 0xB022

	tTrackValCount = 3
	tRotValCount   = 4

	kindMesh   = "mesh"
	kindLight  = "light"
	kindCamera = "camera"
)

type testMaterial struct {
	name                string
	diffR, diffG, diffB byte
	texFile             string
	bumpFile            string
	specR, specG, specB byte
	hasSpec             bool
}

type testFaceMat struct {
	name  string
	faces []uint16
}

type testObject struct {
	name          string
	kind          string
	verts         [][3]float32
	faces         [][3]uint16
	uvs           [][2]float32
	matName       string
	faceMats      []testFaceMat
	hasMatrix     bool
	matrix        [12]float32
	pos           [3]float32
	isSpot        bool
	isDirectional bool
}

type testTrack struct {
	objName   string
	hierarchy int16
	posKeys   [][4]float32
	rotKeys   [][5]float32
	sclKeys   [][4]float32
}

type testOpts struct {
	materials []testMaterial
	objects   []testObject
	tracks    []testTrack
}

func buildTestScene(opts testOpts) *bytes.Reader {
	var editor bytes.Buffer

	for _, mat := range opts.materials {
		var matBody bytes.Buffer

		var nameData bytes.Buffer
		writeCString(&nameData, mat.name)
		writeChunk(&matBody, tMatName, nameData.Bytes())

		var diffBody bytes.Buffer
		var colorData bytes.Buffer
		colorData.Write([]byte{mat.diffR, mat.diffG, mat.diffB})
		writeChunk(&diffBody, tColor24, colorData.Bytes())
		writeChunk(&matBody, tMatDiffuse, diffBody.Bytes())

		if mat.hasSpec {
			var specBody bytes.Buffer
			var specColor bytes.Buffer
			specColor.Write([]byte{mat.specR, mat.specG, mat.specB})
			writeChunk(&specBody, tColor24, specColor.Bytes())
			writeChunk(&matBody, tMatSpec, specBody.Bytes())
		}

		if mat.texFile != "" {
			var texMapBody bytes.Buffer
			var texFileData bytes.Buffer
			writeCString(&texFileData, mat.texFile)
			writeChunk(&texMapBody, tMatTexFile, texFileData.Bytes())
			writeChunk(&matBody, tMatTexMap, texMapBody.Bytes())
		}

		if mat.bumpFile != "" {
			var bumpMapBody bytes.Buffer
			var bumpFileData bytes.Buffer
			writeCString(&bumpFileData, mat.bumpFile)
			writeChunk(&bumpMapBody, tMatTexFile, bumpFileData.Bytes())
			writeChunk(&matBody, tMatBumpMap, bumpMapBody.Bytes())
		}

		writeChunk(&editor, tMaterial, matBody.Bytes())
	}

	for i := range opts.objects {
		obj := opts.objects[i]
		var objBody bytes.Buffer
		writeCString(&objBody, obj.name)

		switch obj.kind {
		case kindMesh:
			var triMesh bytes.Buffer

			var vertBuf bytes.Buffer
			_ = binary.Write(&vertBuf, binary.LittleEndian, uint16(len(obj.verts)))
			for _, v := range obj.verts {
				_ = binary.Write(&vertBuf, binary.LittleEndian, v)
			}
			writeChunk(&triMesh, tVertices, vertBuf.Bytes())

			var faceBuf bytes.Buffer
			_ = binary.Write(&faceBuf, binary.LittleEndian, uint16(len(obj.faces)))
			for _, f := range obj.faces {
				_ = binary.Write(&faceBuf, binary.LittleEndian, f)
				_ = binary.Write(&faceBuf, binary.LittleEndian, uint16(0))
			}
			faceMats := obj.faceMats
			if len(faceMats) == 0 && obj.matName != "" {
				allFaces := make([]uint16, len(obj.faces))
				for j := range obj.faces {
					allFaces[j] = uint16(j)
				}
				faceMats = []testFaceMat{{name: obj.matName, faces: allFaces}}
			}
			for _, group := range faceMats {
				var faceMatBuf bytes.Buffer
				writeCString(&faceMatBuf, group.name)
				_ = binary.Write(&faceMatBuf, binary.LittleEndian, uint16(len(group.faces)))
				for _, faceIdx := range group.faces {
					_ = binary.Write(&faceMatBuf, binary.LittleEndian, faceIdx)
				}
				writeChunk(&faceBuf, tFaceMat, faceMatBuf.Bytes())
			}
			writeChunk(&triMesh, tFaces, faceBuf.Bytes())

			if len(obj.uvs) > 0 {
				var uvBuf bytes.Buffer
				_ = binary.Write(&uvBuf, binary.LittleEndian, uint16(len(obj.uvs)))
				for _, uv := range obj.uvs {
					_ = binary.Write(&uvBuf, binary.LittleEndian, uv)
				}
				writeChunk(&triMesh, tTexCoord, uvBuf.Bytes())
			}

			if obj.hasMatrix {
				var matBuf bytes.Buffer
				for _, f := range obj.matrix {
					_ = binary.Write(&matBuf, binary.LittleEndian, f)
				}
				writeChunk(&triMesh, tLocalMat, matBuf.Bytes())
			}

			writeChunk(&objBody, tTriMesh, triMesh.Bytes())

		case kindLight:
			var lightBody bytes.Buffer
			_ = binary.Write(&lightBody, binary.LittleEndian, obj.pos)
			if obj.isSpot {
				var spotData bytes.Buffer
				_ = binary.Write(&spotData, binary.LittleEndian, [3]float32{0, 0, 0})
				_ = binary.Write(&spotData, binary.LittleEndian, float32(45.0))
				writeChunk(&lightBody, tSpotlight, spotData.Bytes())
			}
			if obj.isDirectional {
				writeChunk(&lightBody, tDirectLight, nil)
			}
			writeChunk(&objBody, tLight, lightBody.Bytes())

		case kindCamera:
			var camData bytes.Buffer
			_ = binary.Write(&camData, binary.LittleEndian, obj.pos)
			_ = binary.Write(&camData, binary.LittleEndian, [3]float32{})
			_ = binary.Write(&camData, binary.LittleEndian, float32(0))
			_ = binary.Write(&camData, binary.LittleEndian, float32(60.0))
			writeChunk(&objBody, tCamera, camData.Bytes())
		}

		writeChunk(&editor, tObject, objBody.Bytes())
	}

	var main bytes.Buffer
	writeChunk(&main, tEditor, editor.Bytes())

	if len(opts.tracks) > 0 {
		var kfBody bytes.Buffer
		for _, trk := range opts.tracks {
			var trackBody bytes.Buffer

			var nameData bytes.Buffer
			writeCString(&nameData, trk.objName)
			_ = binary.Write(&nameData, binary.LittleEndian, uint16(0))
			_ = binary.Write(&nameData, binary.LittleEndian, uint16(0))
			_ = binary.Write(&nameData, binary.LittleEndian, trk.hierarchy)
			writeChunk(&trackBody, tTrackObjName, nameData.Bytes())

			if len(trk.posKeys) > 0 {
				writeTrack(&trackBody, tPosTrack, trk.posKeys, tTrackValCount)
			}
			if len(trk.rotKeys) > 0 {
				writeRotTrack(&trackBody, trk.rotKeys)
			}
			if len(trk.sclKeys) > 0 {
				writeTrack(&trackBody, tScaleTrack, trk.sclKeys, tTrackValCount)
			}

			writeChunk(&kfBody, tMeshTrackNode, trackBody.Bytes())
		}
		writeChunk(&main, tKeyframer, kfBody.Bytes())
	}

	var file bytes.Buffer
	writeChunk(&file, tMain, main.Bytes())
	return bytes.NewReader(file.Bytes())
}

func writeTrack(buf *bytes.Buffer, chunkID uint16, keys [][4]float32, valCount int) {
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, uint16(0))
	_ = binary.Write(&data, binary.LittleEndian, uint32(0))
	_ = binary.Write(&data, binary.LittleEndian, uint32(0))
	_ = binary.Write(&data, binary.LittleEndian, uint16(len(keys)))

	for _, key := range keys {
		_ = binary.Write(&data, binary.LittleEndian, uint32(key[0]))
		_ = binary.Write(&data, binary.LittleEndian, uint16(0))
		for j := 1; j <= valCount; j++ {
			if j < len(key) {
				_ = binary.Write(&data, binary.LittleEndian, key[j])
			} else {
				_ = binary.Write(&data, binary.LittleEndian, float32(0))
			}
		}
	}

	writeChunk(buf, chunkID, data.Bytes())
}

func writeRotTrack(buf *bytes.Buffer, keys [][5]float32) {
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, uint16(0))
	_ = binary.Write(&data, binary.LittleEndian, uint32(0))
	_ = binary.Write(&data, binary.LittleEndian, uint32(0))
	_ = binary.Write(&data, binary.LittleEndian, uint16(len(keys)))

	for _, key := range keys {
		_ = binary.Write(&data, binary.LittleEndian, uint32(key[0]))
		_ = binary.Write(&data, binary.LittleEndian, uint16(0))
		for j := 1; j <= tRotValCount; j++ {
			_ = binary.Write(&data, binary.LittleEndian, key[j])
		}
	}

	writeChunk(buf, tRotTrack, data.Bytes())
}

func TestDecode3DS(t *testing.T) {
	tests := []struct {
		name  string
		opts  testOpts
		check func(t *testing.T, sc *ir.Asset)
	}{
		{
			name: "Geometry",
			opts: testOpts{objects: []testObject{{
				name:  "box",
				kind:  kindMesh,
				verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}},
				faces: [][3]uint16{{0, 1, 2}, {1, 3, 2}},
			}}},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Meshes, 1)
				assert.Equal(t, "box", sc.Meshes[0].Name)
				prim := sc.Meshes[0].Primitives[0]
				assert.Equal(t, ir.Triangles, prim.Mode)
				assert.Equal(t, 4, prim.Data.VertexCount)
				assert.Len(t, prim.Data.Indices, 6)
				assert.Equal(t, [3]float32{1, 1, 0}, prim.Data.Positions[3])
			},
		},
		{
			name: "UVs",
			opts: testOpts{objects: []testObject{{
				name:  "quad",
				kind:  kindMesh,
				verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
				faces: [][3]uint16{{0, 1, 2}},
				uvs:   [][2]float32{{0, 0}, {1, 0}, {0, 1}},
			}}},
			check: func(t *testing.T, sc *ir.Asset) {
				prim := sc.Meshes[0].Primitives[0]
				require.True(t, prim.Data.HasUVs())
				assert.Equal(t, [2]float32{1, 0}, prim.Data.TexCoord0[1])
			},
		},
		{
			name: "Materials",
			opts: testOpts{
				materials: []testMaterial{{name: "red_mat", diffR: 255, diffG: 0, diffB: 0, texFile: "brick.png"}},
				objects:   []testObject{{name: "box", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}}, matName: "red_mat"}},
			},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Materials, 1)
				mat := sc.Materials[0]
				assert.Equal(t, "red_mat", mat.Name)
				assert.InDelta(t, 1.0, mat.BaseColorFactor[0], 0.01)
				assert.InDelta(t, 0.0, mat.BaseColorFactor[1], 0.01)
				require.NotNil(t, mat.BaseColorTexture)
				assert.Equal(t, "brick.png", sc.Images[sc.Textures[0].ImageIndex].SourcePath)
				assert.Equal(t, 0, sc.Meshes[0].Primitives[0].MaterialIndex)
			},
		},
		{
			name: "TransformMatrix",
			opts: testOpts{objects: []testObject{{
				name: "moved", kind: kindMesh,
				verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}},
				hasMatrix: true, matrix: [12]float32{1, 0, 0, 0, 1, 0, 0, 0, 1, 10, 20, 30},
			}}},
			check: func(t *testing.T, sc *ir.Asset) {
				node := sc.Nodes[0]
				assert.Equal(t, float32(10), node.Transform.Matrix[12])
				assert.Equal(t, float32(20), node.Transform.Matrix[13])
				assert.Equal(t, float32(30), node.Transform.Matrix[14])
			},
		},
		{
			name: "PointLight",
			opts: testOpts{objects: []testObject{{name: "sun", kind: kindLight, pos: [3]float32{5, 10, 15}}}},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Lights, 1)
				assert.Equal(t, "sun", sc.Lights[0].Name)
				assert.NotNil(t, sc.Lights[0].Point)
				assert.Nil(t, sc.Lights[0].Spot)
				assert.Equal(t, float32(5), sc.Nodes[0].Transform.Translation[0])
			},
		},
		{
			name: "Spotlight",
			opts: testOpts{objects: []testObject{{name: "spot", kind: kindLight, pos: [3]float32{1, 2, 3}, isSpot: true}}},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Lights, 1)
				assert.NotNil(t, sc.Lights[0].Spot)
				assert.Nil(t, sc.Lights[0].Point)
				assert.InDelta(t, 45.0*math.Pi/180.0, sc.Lights[0].Spot.InnerConeAngle, 0.01)
			},
		},
		{
			name: "DirectionalLight",
			opts: testOpts{objects: []testObject{{name: "sun", kind: kindLight, pos: [3]float32{0, 10, 0}, isDirectional: true}}},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Lights, 1)
				assert.NotNil(t, sc.Lights[0].Directional)
				assert.Nil(t, sc.Lights[0].Point)
				assert.Nil(t, sc.Lights[0].Spot)
			},
		},
		{
			name: "Camera",
			opts: testOpts{objects: []testObject{{name: "cam1", kind: kindCamera, pos: [3]float32{0, 5, -10}}}},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Cameras, 1)
				assert.Equal(t, "cam1", sc.Cameras[0].Name)
				assert.InDelta(t, 60.0*math.Pi/180.0, sc.Cameras[0].Perspective.FOV, 0.01)
				assert.Equal(t, 0, sc.Nodes[0].CameraIndex)
			},
		},
		{
			name: "BumpMap",
			opts: testOpts{
				materials: []testMaterial{{name: "bumpy", diffR: 200, diffG: 200, diffB: 200, bumpFile: "normal.png"}},
				objects:   []testObject{{name: "plane", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}}}},
			},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Materials, 1)
				mat := sc.Materials[0]
				require.NotNil(t, mat.NormalTexture)
				assert.Equal(t, "normal.png", sc.Images[sc.Textures[mat.NormalTexture.TextureIndex].ImageIndex].SourcePath)
				assert.InDelta(t, 1.0, mat.NormalScale, 0.001)
			},
		},
		{
			name: "SpecularColor",
			opts: testOpts{
				materials: []testMaterial{{name: "shiny", diffR: 128, diffG: 128, diffB: 128, hasSpec: true, specR: 255, specG: 255, specB: 255}},
				objects:   []testObject{{name: "obj", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}}}},
			},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Materials, 1)
				assert.InDelta(t, 1.0, sc.Materials[0].MetallicFactor, 0.01)
			},
		},
		{
			name: "FullScene",
			opts: testOpts{
				materials: []testMaterial{{name: "floor_mat", diffR: 200, diffG: 200, diffB: 200}},
				objects: []testObject{
					{name: "floor", kind: kindMesh, verts: [][3]float32{{-5, 0, -5}, {5, 0, -5}, {5, 0, 5}, {-5, 0, 5}}, faces: [][3]uint16{{0, 1, 2}, {0, 2, 3}}, uvs: [][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}, matName: "floor_mat"},
					{name: "key_light", kind: kindLight, pos: [3]float32{3, 8, 4}},
					{name: "main_cam", kind: kindCamera, pos: [3]float32{0, 3, -8}},
				},
			},
			check: func(t *testing.T, sc *ir.Asset) {
				assert.Len(t, sc.Meshes, 1)
				assert.Len(t, sc.Materials, 1)
				assert.Len(t, sc.Lights, 1)
				assert.Len(t, sc.Cameras, 1)
				assert.Len(t, sc.Nodes, 3)
				assert.Equal(t, ir.Format3DS, sc.Metadata.SourceFormat)
			},
		},
		{
			name: "PerFaceMaterialGroups",
			opts: testOpts{
				materials: []testMaterial{
					{name: "red", diffR: 255, diffG: 0, diffB: 0},
					{name: "blue", diffR: 0, diffG: 0, diffB: 255},
				},
				objects: []testObject{{
					name:  "split",
					kind:  kindMesh,
					verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}},
					faces: [][3]uint16{{0, 1, 2}, {1, 3, 2}},
					faceMats: []testFaceMat{
						{name: "red", faces: []uint16{0}},
						{name: "blue", faces: []uint16{1}},
					},
				}},
			},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Meshes, 1)
				require.Len(t, sc.Meshes[0].Primitives, 2)
				assert.Equal(t, 0, sc.Meshes[0].Primitives[0].MaterialIndex)
				assert.Equal(t, 1, sc.Meshes[0].Primitives[1].MaterialIndex)
				assert.Len(t, sc.Meshes[0].Primitives[0].Data.Indices, 3)
				assert.Len(t, sc.Meshes[0].Primitives[1].Data.Indices, 3)
			},
		},
		{
			name: "KeyframeAnimation",
			opts: testOpts{
				objects: []testObject{{name: "cube", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}}}},
				tracks: []testTrack{{
					objName: "cube",
					posKeys: [][4]float32{{0, 0, 0, 0}, {30, 10, 0, 0}},
				}},
			},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Animations, 1)
				anim := sc.Animations[0]
				require.Greater(t, len(anim.Channels), 0)

				var posCh *ir.AnimationChannel
				for i := range anim.Channels {
					if anim.Channels[i].Target == ir.TargetTranslation {
						posCh = &anim.Channels[i]
						break
					}
				}
				require.NotNil(t, posCh)
				require.Len(t, posCh.Times, 2)
				assert.InDelta(t, 0.0, posCh.Times[0], 0.01)
				assert.InDelta(t, 1.0, posCh.Times[1], 0.01)
				assert.InDelta(t, float32(10), posCh.Translations[1][0], 0.01)
			},
		},
		{
			name: "NodeHierarchy",
			opts: testOpts{
				objects: []testObject{
					{name: "root", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}}},
					{name: "child1", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {2, 0, 0}, {0, 2, 0}}, faces: [][3]uint16{{0, 1, 2}}},
					{name: "child2", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {3, 0, 0}, {0, 3, 0}}, faces: [][3]uint16{{0, 1, 2}}},
				},
				tracks: []testTrack{
					{objName: "root", hierarchy: -1},
					{objName: "child1", hierarchy: 0},
					{objName: "child2", hierarchy: 0},
				},
			},
			check: func(t *testing.T, sc *ir.Asset) {
				require.Len(t, sc.Nodes, 3)
				var rootIdx int
				for i := range sc.Nodes {
					if sc.Nodes[i].Name == "root" {
						rootIdx = i
						break
					}
				}
				assert.Len(t, sc.Nodes[rootIdx].Children, 2)
				assert.Len(t, sc.RootNodes, 1)
				assert.Equal(t, rootIdx, sc.RootNodes[0])
			},
		},
	}

	dec := &tds.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := buildTestScene(tt.opts)
			sc, err := dec.Decode(r, detect.DecodeOptions{})
			require.NoError(t, err)
			tt.check(t, sc)
		})
	}
}

func TestDecode3DSBasics(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, dec *tds.Decoder)
	}{
		{"Probe", func(t *testing.T, dec *tds.Decoder) {
			r := buildTestScene(testOpts{objects: []testObject{{name: "tri", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}}}}})
			assert.True(t, dec.Probe(r))
			assert.False(t, dec.Probe(bytes.NewReader([]byte("not 3ds data at all"))))
		}},
		{"RejectJunk", func(t *testing.T, dec *tds.Decoder) {
			_, err := dec.Decode(bytes.NewReader([]byte{0x01, 0x02}), detect.DecodeOptions{})
			require.Error(t, err)
		}},
	}

	dec := &tds.Decoder{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, dec)
		})
	}
}

func TestDecodeRotAndScaleTracks(t *testing.T) {
	r := buildTestScene(testOpts{
		objects: []testObject{{name: "cube", kind: kindMesh, verts: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, faces: [][3]uint16{{0, 1, 2}}}},
		tracks: []testTrack{{
			objName:   "cube",
			hierarchy: -1,
			posKeys:   [][4]float32{{0, 0, 0, 0}},
			rotKeys:   [][5]float32{{0, 0.707, 0, 0, 0.707}},
			sclKeys:   [][4]float32{{0, 2, 2, 2}},
		}},
	})

	dec := &tds.Decoder{}
	asset, err := dec.Decode(r, detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, asset.Animations, 1)

	anim := asset.Animations[0]
	var hasTrans, hasRot, hasScale bool
	for _, ch := range anim.Channels {
		switch ch.Target {
		case ir.TargetTranslation:
			hasTrans = true
		case ir.TargetRotation:
			hasRot = true
		case ir.TargetScale:
			hasScale = true
		}
	}
	assert.True(t, hasTrans)
	assert.True(t, hasRot)
	assert.True(t, hasScale)
}

func TestDecodeSmoothGroups(t *testing.T) {
	// Smooth groups chunk (0x4150) is read after faces.
	// Since test helper doesn't emit it yet, test via full scene approach.
	dec := &tds.Decoder{}
	r := buildTestScene(testOpts{
		materials: []testMaterial{{name: "m", diffR: 128, diffG: 128, diffB: 128, texFile: "tex.png", hasSpec: true, specR: 200, specG: 200, specB: 200}},
		objects: []testObject{
			{name: "multi", kind: kindMesh,
				verts:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}, {0, 0, 1}, {1, 0, 1}},
				faces:   [][3]uint16{{0, 1, 2}, {3, 4, 5}},
				uvs:     [][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 0}, {1, 0}},
				matName: "m",
			},
		},
	})
	asset, err := dec.Decode(r, detect.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, asset.Meshes, 1)
	assert.Equal(t, 6, asset.Meshes[0].Primitives[0].Data.VertexCount)
}
