package gltf

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/gophics/ravenporter/detect"
)

func FuzzDecodeDracoExtension(f *testing.F) {
	valid := encodeDracoTriangle(f, false)
	f.Add(valid, 0, 0, len(valid), 3, 3, int(testDracoPositionID), modeTriangles, uint8(0))
	f.Add([]byte{1, 2, 3}, 0, 0, 3, 3, 3, int(testDracoPositionID), modeTriangles, uint8(0))
	f.Add(valid, 9, 0, len(valid), 3, 3, int(testDracoPositionID), modeTriangles, uint8(0))
	f.Add(valid, 0, 1, len(valid), 3, 3, int(testDracoPositionID), modeTriangles, uint8(3))
	f.Add(valid, 0, 0, len(valid), 3, 3, int(testDracoPositionID), modeLines, uint8(4))

	f.Fuzz(func(
		t *testing.T,
		payload []byte,
		bufferView int,
		byteOffset int,
		byteLength int,
		positionCount int,
		indexCount int,
		uniqueID int,
		mode int,
		shape uint8,
	) {
		if len(payload) > 4096 {
			t.Skip()
		}
		positionCount = boundedFuzzCount(positionCount)
		indexCount = boundedFuzzCount(indexCount)
		byteOffset = boundedFuzzRange(byteOffset, len(payload)+8)
		byteLength = boundedFuzzRange(byteLength, len(payload)+8)
		uniqueID = boundedFuzzRange(uniqueID, 1<<16)

		jsonData := fmt.Sprintf(`{
  "asset": {"version": "2.0"},
  "extensionsUsed": ["KHR_draco_mesh_compression"],
  "buffers": [{"uri": "mesh.drc", "byteLength": %d}],
  "bufferViews": [{"buffer": 0, "byteOffset": %d, "byteLength": %d}],
  "accessors": [
    {"componentType": 5126, "count": %d, "type": "VEC3"},
    {"componentType": 5125, "count": %d, "type": "SCALAR"}
  ],
  "meshes": [{"primitives": [{
    "attributes": {"POSITION": 0},
    "indices": 1,
    "mode": %d,
    "extensions": {"KHR_draco_mesh_compression": {%s}}
  }]}],
  "nodes": [{"mesh": 0}],
  "scenes": [{"nodes": [0]}],
  "scene": 0
}`, len(payload), byteOffset, byteLength, positionCount, indexCount, mode, fuzzDracoExtensionJSON(bufferView, uniqueID, shape))

		_, _ = (&Decoder{}).Decode(bytes.NewReader([]byte(jsonData)), detect.DecodeOptions{
			FS:          mapFS{"mesh.drc": payload},
			MaxVertices: 128,
		})
	})
}

func fuzzDracoExtensionJSON(bufferView, uniqueID int, shape uint8) string {
	switch shape % 6 {
	case 0:
		return fmt.Sprintf(`"bufferView": %d, "attributes": {"POSITION": %d}`, bufferView, uniqueID)
	case 1:
		return fmt.Sprintf(`"bufferView": "bad", "attributes": {"POSITION": %d}`, uniqueID)
	case 2:
		return fmt.Sprintf(`"bufferView": %d`, bufferView)
	case 3:
		return fmt.Sprintf(`"bufferView": %d, "attributes": {"POSITION": "bad"}`, bufferView)
	case 4:
		return fmt.Sprintf(`"bufferView": %d, "attributes": {"POSITION": %d, "_IGNORED": 999}`, bufferView, uniqueID)
	default:
		return fmt.Sprintf(`"bufferView": %d, "attributes": {}`, bufferView)
	}
}

func boundedFuzzCount(value int) int {
	if value < 0 {
		value = ^value
	}
	return value % 64
}

func boundedFuzzRange(value, limit int) int {
	if limit <= 0 {
		return 0
	}
	if value < 0 {
		value = ^value
	}
	return value % limit
}
