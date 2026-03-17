package gltf

import (
	"encoding/binary"
	"errors"
	"unsafe"

	"github.com/gophics/ravenporter/internal/binread"
)

const (
	compByte   = 5120
	compUByte  = 5121
	compShort  = 5122
	compUShort = 5123
	compUInt   = 5125
	compFloat  = 5126
)

const (
	sizeCompByte  = 1
	sizeCompShort = 2
	sizeCompInt   = 4
	sizeCompFloat = 4

	elemVec2 = 2
	elemVec3 = 3
	elemVec4 = 4
	elemMat3 = 9
	elemMat4 = 16

	normUbyteMax  = 255.0
	normByteMax   = 127.0
	normUshortMax = 65535.0
)

func typeElemCount(t string) int {
	switch t {
	case "SCALAR":
		return 1
	case "VEC2":
		return elemVec2
	case "VEC3":
		return elemVec3
	case "VEC4":
		return elemVec4
	case "MAT2":
		return elemVec4
	case "MAT3":
		return elemMat3
	case "MAT4":
		return elemMat4
	default:
		return 0
	}
}

var errAccessorOOB = errors.New("gltf: accessor out of bounds")

type accessor struct {
	bufferView    int
	byteOffset    int
	componentType int
	count         int
	elemCount     int
	sparseCount   int
	sparseIdxBV   int
	sparseIdxOff  int
	sparseIdxComp int
	sparseValBV   int
	sparseValOff  int
}

type bufferView struct {
	buffer     int
	byteOffset int
	byteLength int
	byteStride int
	meshopt    *meshoptBufferView
}

type bufferSet struct {
	buffers [][]byte
	views   []bufferView
}

func componentSize(comp int) int {
	switch comp {
	case compByte, compUByte:
		return sizeCompByte
	case compShort, compUShort:
		return sizeCompShort
	case compUInt:
		return sizeCompInt
	case compFloat:
		return sizeCompFloat
	default:
		return 0
	}
}

//nolint:gocritic // unnamedResult: returns are assigned in body
func (bs *bufferSet) resolveBytes(a accessor) ([]byte, int, error) {
	if a.bufferView < 0 || a.bufferView >= len(bs.views) {
		return nil, 0, errAccessorOOB
	}
	bv := bs.views[a.bufferView]
	if bv.buffer < 0 || bv.buffer >= len(bs.buffers) {
		return nil, 0, errAccessorOOB
	}
	buf := bs.buffers[bv.buffer]
	start := bv.byteOffset + a.byteOffset
	if start >= len(buf) {
		return nil, 0, errAccessorOOB
	}
	data := buf[start:]

	stride := bv.byteStride
	if stride == 0 {
		stride = componentSize(a.componentType) * a.elemCount
	}
	return data, stride, nil
}

func (bs *bufferSet) readFloat32s(a accessor) []float32 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	out := make([]float32, a.count)
	cs := componentSize(a.componentType)
	for i := range a.count {
		off := i * stride
		if off+cs > len(data) {
			break
		}
		out[i] = readComponent(data[off:], a.componentType)
	}
	return out
}

func (bs *bufferSet) readVec2s(a accessor) [][2]float32 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	out := make([][2]float32, a.count)
	cs := componentSize(a.componentType)
	for i := range a.count {
		off := i * stride
		end := off + cs*elemVec2
		if end > len(data) {
			break
		}
		out[i][0] = readComponent(data[off:], a.componentType)
		out[i][1] = readComponent(data[off+cs:], a.componentType)
	}
	return out
}

func (bs *bufferSet) readVec3s(a accessor) [][3]float32 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	// Fast path: tightly-packed float32 VEC3.
	if a.componentType == compFloat && stride == vec3Float32Stride && a.sparseCount == 0 {
		return castVec3Slice(data, a.count)
	}

	out := make([][3]float32, a.count)
	cs := componentSize(a.componentType)
	for i := range a.count {
		off := i * stride
		end := off + cs*elemVec3
		if end > len(data) {
			break
		}
		out[i][0] = readComponent(data[off:], a.componentType)
		out[i][1] = readComponent(data[off+cs:], a.componentType)
		out[i][2] = readComponent(data[off+cs*2:], a.componentType)
	}
	if a.sparseCount > 0 {
		bs.applySparseVec3s(a, out)
	}
	return out
}

func (bs *bufferSet) applySparseVec3s(a accessor, out [][3]float32) {
	if a.sparseIdxBV < 0 || a.sparseIdxBV >= len(bs.views) ||
		a.sparseValBV < 0 || a.sparseValBV >= len(bs.views) {

		return
	}

	idxView := bs.views[a.sparseIdxBV]
	if idxView.buffer < 0 || idxView.buffer >= len(bs.buffers) {
		return
	}
	idxData := bs.buffers[idxView.buffer][idxView.byteOffset+a.sparseIdxOff:]
	idxCS := componentSize(a.sparseIdxComp)

	valView := bs.views[a.sparseValBV]
	if valView.buffer < 0 || valView.buffer >= len(bs.buffers) {
		return
	}
	valData := bs.buffers[valView.buffer][valView.byteOffset+a.sparseValOff:]
	valCS := componentSize(a.componentType)

	for i := range a.sparseCount {
		idxOff := i * idxCS
		if idxOff+idxCS > len(idxData) {
			break
		}
		var idx int
		switch a.sparseIdxComp {
		case compUByte:
			idx = int(idxData[idxOff])
		case compUShort:
			idx = int(binary.LittleEndian.Uint16(idxData[idxOff:]))
		case compUInt:
			idx = int(binary.LittleEndian.Uint32(idxData[idxOff:]))
		}
		if idx >= len(out) {
			continue
		}
		vOff := i * valCS * elemVec3
		if vOff+valCS*elemVec3 > len(valData) {
			break
		}
		out[idx][0] = readComponent(valData[vOff:], a.componentType)
		out[idx][1] = readComponent(valData[vOff+valCS:], a.componentType)
		out[idx][2] = readComponent(valData[vOff+valCS*2:], a.componentType)
	}
}

func (bs *bufferSet) readVec4s(a accessor) [][4]float32 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	// Fast path: tightly-packed float32 VEC4.
	if a.componentType == compFloat && stride == vec4Float32Stride {
		return castVec4Slice(data, a.count)
	}

	out := make([][4]float32, a.count)
	cs := componentSize(a.componentType)
	for i := range a.count {
		off := i * stride
		end := off + cs*elemVec4
		if end > len(data) {
			break
		}
		out[i][0] = readComponent(data[off:], a.componentType)
		out[i][1] = readComponent(data[off+cs:], a.componentType)
		out[i][2] = readComponent(data[off+cs*2:], a.componentType)
		out[i][3] = readComponent(data[off+cs*3:], a.componentType)
	}
	return out
}

func (bs *bufferSet) readMat4s(a accessor) [][16]float32 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	out := make([][16]float32, a.count)
	cs := componentSize(a.componentType)
	for i := range a.count {
		off := i * stride
		for j := range elemMat4 {
			p := off + j*cs
			if p+cs > len(data) {
				break
			}
			out[i][j] = readComponent(data[p:], a.componentType)
		}
	}
	return out
}

func (bs *bufferSet) readIndices(a accessor) []uint32 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	out := make([]uint32, a.count)
	cs := componentSize(a.componentType)
	if stride == 0 {
		stride = cs
	}
	for i := range a.count {
		off := i * stride
		if off+cs > len(data) {
			break
		}
		switch a.componentType {
		case compUByte:
			out[i] = uint32(data[off])
		case compUShort:
			out[i] = uint32(binary.LittleEndian.Uint16(data[off:]))
		case compUInt:
			out[i] = binary.LittleEndian.Uint32(data[off:])
		}
	}
	return out
}

func (bs *bufferSet) readJoints(a accessor) [][4]uint16 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	out := make([][4]uint16, a.count)
	cs := componentSize(a.componentType)
	for i := range a.count {
		off := i * stride
		for j := range elemVec4 {
			p := off + j*cs
			if p+cs > len(data) {
				break
			}
			switch a.componentType {
			case compUByte:
				out[i][j] = uint16(data[p])
			case compUShort:
				out[i][j] = binary.LittleEndian.Uint16(data[p:])
			}
		}
	}
	return out
}

func (bs *bufferSet) readColors(a accessor) [][4]float32 {
	data, stride, err := bs.resolveBytes(a)
	if err != nil || a.count == 0 {
		return nil
	}

	out := make([][4]float32, a.count)
	cs := componentSize(a.componentType)
	for i := range a.count {
		off := i * stride
		for j := range a.elemCount {
			p := off + j*cs
			if p+cs > len(data) {
				break
			}
			out[i][j] = readComponentNorm(data[p:], a.componentType)
		}
		if a.elemCount == elemVec3 {
			out[i][3] = 1.0
		}
	}
	return out
}

func readComponent(data []byte, comp int) float32 {
	if len(data) == 0 {
		return 0
	}
	switch comp {
	case compFloat:
		return binread.ReadF32LE(data)
	case compByte:
		return float32(int8(data[0]))
	case compUByte:
		return float32(data[0])
	case compShort:
		return float32(binread.ReadI16LE(data))
	case compUShort:
		return float32(binary.LittleEndian.Uint16(data))
	case compUInt:
		return float32(binary.LittleEndian.Uint32(data))
	default:
		return 0
	}
}

func readComponentNorm(data []byte, comp int) float32 {
	if len(data) == 0 {
		return 0
	}
	switch comp {
	case compFloat:
		return binread.ReadF32LE(data)
	case compUByte:
		return float32(data[0]) / normUbyteMax
	case compByte:
		return float32(int8(data[0])) / normByteMax
	case compUShort:
		return float32(binary.LittleEndian.Uint16(data)) / normUshortMax
	default:
		return readComponent(data, comp)
	}
}

const (
	vec3Float32Stride = 12
	vec4Float32Stride = 16
)

func castVec3Slice(data []byte, count int) [][3]float32 {
	need := count * vec3Float32Stride
	if len(data) < need {
		return nil
	}
	return unsafe.Slice((*[3]float32)(unsafe.Pointer(&data[0])), count) //nolint:gosec // bounds checked
}

func castVec4Slice(data []byte, count int) [][4]float32 {
	need := count * vec4Float32Stride
	if len(data) < need {
		return nil
	}
	return unsafe.Slice((*[4]float32)(unsafe.Pointer(&data[0])), count) //nolint:gosec // bounds checked
}
