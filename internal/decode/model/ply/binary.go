package ply

import (
	"io"
	"strconv"

	"github.com/gophics/ravenporter/internal/binread"
)

const (
	maxFaceVertsBinary = 8
	maxStrideBuf       = 128
)

func readBinaryVertices(r io.Reader, hdr *header, le bool, pos, norm [][3]float32, col [][4]float32, uv [][2]float32) error {
	var stackBuf [maxStrideBuf]byte
	var buf []byte
	if hdr.stride <= maxStrideBuf {
		buf = stackBuf[:hdr.stride]
	} else {
		buf = make([]byte, hdr.stride)
	}
	for i := range hdr.vertexCount {
		if _, err := io.ReadFull(r, buf); err != nil {
			return decodeErrCause(errBadVertex.Error()+" "+strconv.Itoa(i), err)
		}

		if hdr.xIdx >= 0 {
			var vx, vy, vz float32
			vx = readPropFloat(buf, hdr.offsets[hdr.xIdx], hdr.props[hdr.xIdx].typ, le)
			if hdr.yIdx >= 0 {
				vy = readPropFloat(buf, hdr.offsets[hdr.yIdx], hdr.props[hdr.yIdx].typ, le)
			}
			if hdr.zIdx >= 0 {
				vz = readPropFloat(buf, hdr.offsets[hdr.zIdx], hdr.props[hdr.zIdx].typ, le)
			}
			pos[i] = [3]float32{vx, vy, vz}
		}
		if norm != nil {
			var nx, ny, nz float32
			if hdr.nxIdx >= 0 {
				nx = readPropFloat(buf, hdr.offsets[hdr.nxIdx], hdr.props[hdr.nxIdx].typ, le)
			}
			if hdr.nyIdx >= 0 {
				ny = readPropFloat(buf, hdr.offsets[hdr.nyIdx], hdr.props[hdr.nyIdx].typ, le)
			}
			if hdr.nzIdx >= 0 {
				nz = readPropFloat(buf, hdr.offsets[hdr.nzIdx], hdr.props[hdr.nzIdx].typ, le)
			}
			norm[i] = [3]float32{nx, ny, nz}
		}
		if col != nil {
			var cr, cg, cb, ca float32
			ca = 1.0
			if hdr.rIdx >= 0 {
				cr = readPropFloat(buf, hdr.offsets[hdr.rIdx], hdr.props[hdr.rIdx].typ, le)
			}
			if hdr.gIdx >= 0 {
				cg = readPropFloat(buf, hdr.offsets[hdr.gIdx], hdr.props[hdr.gIdx].typ, le)
			}
			if hdr.bIdx >= 0 {
				cb = readPropFloat(buf, hdr.offsets[hdr.bIdx], hdr.props[hdr.bIdx].typ, le)
			}
			if hdr.aIdx >= 0 {
				ca = readPropFloat(buf, hdr.offsets[hdr.aIdx], hdr.props[hdr.aIdx].typ, le)
			}
			if hdr.rIdx >= 0 && hdr.props[hdr.rIdx].typ == propUchar {
				cr *= inv255
				cg *= inv255
				cb *= inv255
				if hdr.aIdx >= 0 {
					ca *= inv255
				}
			}
			col[i] = [4]float32{cr, cg, cb, ca}
		}
		if uv != nil {
			var s, t float32
			if hdr.sIdx >= 0 {
				s = readPropFloat(buf, hdr.offsets[hdr.sIdx], hdr.props[hdr.sIdx].typ, le)
			}
			if hdr.tIdx >= 0 {
				t = readPropFloat(buf, hdr.offsets[hdr.tIdx], hdr.props[hdr.tIdx].typ, le)
			}
			uv[i] = [2]float32{s, t}
		}
	}
	return nil
}

func readBinaryFaces(r io.Reader, hdr *header, le bool) ([]uint32, error) {
	if hdr.faceCount == 0 {
		return nil, nil
	}

	indices := make([]uint32, 0, hdr.faceCount*minFaceVerts)
	countSize := propSize(hdr.faceListType)
	indexSize := propSize(hdr.faceIndexType)
	countBuf := make([]byte, countSize)
	faceBuf := make([]byte, indexSize*maxFaceVertsBinary)

	for i := range hdr.faceCount {
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, decodeErrCause(errBadFace.Error()+" "+strconv.Itoa(i), err)
		}
		n := int(readPropUint(countBuf, 0, hdr.faceListType, le))
		if n < minFaceVerts {
			continue
		}

		needed := n * indexSize
		if needed > len(faceBuf) {
			faceBuf = make([]byte, needed)
		}

		if _, err := io.ReadFull(r, faceBuf[:needed]); err != nil {
			return nil, decodeErrCause(errBadFace.Error()+" "+strconv.Itoa(i), err)
		}

		var stackBuf [maxFaceVertsBinary]uint32
		var faceVerts []uint32
		if n <= maxFaceVertsBinary {
			faceVerts = stackBuf[:n]
		} else {
			faceVerts = make([]uint32, n)
		}
		for j := range n {
			faceVerts[j] = readPropUint(faceBuf, j*indexSize, hdr.faceIndexType, le)
		}
		indices = fanFromSlice(indices, faceVerts)
	}
	return indices, nil
}

func readPropFloat(buf []byte, offset int, t propType, le bool) float32 {
	b := buf[offset:]
	switch t {
	case propFloat:
		if le {
			return binread.ReadF32LE(b)
		}
		return binread.ReadF32BE(b)
	case propDouble:
		if le {
			return float32(binread.ReadF64LE(b))
		}
		return float32(binread.ReadF64BE(b))
	case propUchar:
		return float32(b[0])
	case propChar:
		return float32(int8(b[0]))
	case propInt:
		if le {
			return float32(binread.ReadI32LE(b))
		}
		return float32(int32(binread.ReadU32BE(b))) //nolint:gosec // intentional
	case propShort:
		if le {
			return float32(binread.ReadI16LE(b))
		}
		return float32(binread.ReadI16BE(b))
	case propUint:
		if le {
			return float32(binread.ReadU32LE(b))
		}
		return float32(binread.ReadU32BE(b))
	case propUshort:
		if le {
			return float32(binread.ReadU16LE(b))
		}
		return float32(binread.ReadU16BE(b))
	default:
		return 0
	}
}

func readPropUint(buf []byte, offset int, t propType, le bool) uint32 {
	b := buf[offset:]
	switch t {
	case propUchar:
		return uint32(b[0])
	case propChar:
		return uint32(int8(b[0]))
	case propUshort:
		if le {
			return uint32(binread.ReadU16LE(b))
		}
		return uint32(binread.ReadU16BE(b))
	case propInt, propUint:
		if le {
			return binread.ReadU32LE(b)
		}
		return binread.ReadU32BE(b)
	default:
		return 0
	}
}

func readBinaryEdges(r io.Reader, hdr *header, le bool) ([]uint32, error) {
	if hdr.edgeCount == 0 {
		return nil, nil
	}
	v1Size := propSize(hdr.edgeVertex1Type)
	v2Size := propSize(hdr.edgeVertex2Type)
	edgeStride := v1Size + v2Size
	buf := make([]byte, edgeStride)
	edges := make([]uint32, 0, hdr.edgeCount*indicesPerEdge)

	for i := range hdr.edgeCount {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, decodeErrCause(errBadEdge.Error()+" "+strconv.Itoa(i), err)
		}
		v1 := readPropUint(buf, 0, hdr.edgeVertex1Type, le)
		v2 := readPropUint(buf, v1Size, hdr.edgeVertex2Type, le)
		edges = append(edges, v1, v2)
	}
	return edges, nil
}
