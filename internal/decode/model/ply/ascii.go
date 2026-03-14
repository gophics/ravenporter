package ply

import (
	"strconv"

	"github.com/gophics/ravenporter/internal/decutil"
)

const (
	maxFaceVertsASCII = 8
	maxStackProps     = 16
	indicesPerEdge    = 2
	minEdgeFields     = 2
)

//nolint:funlen // flat architecture to avoid sub-allocations in hotpath
func readASCIIVertices(sc *decutil.LineScanner, hdr *header, pos, norm [][3]float32, col [][4]float32, uv [][2]float32) error {
	nProps := len(hdr.props)
	var stackVals [maxStackProps]float64
	var vals []float64
	if nProps <= maxStackProps {
		vals = stackVals[:nProps]
	} else {
		vals = make([]float64, nProps)
	}
	var fields []string

	for i := range hdr.vertexCount {
		line := sc.Next()
		if line == nil {
			return decodeErr(errBadVertex.Error() + " " + strconv.Itoa(i))
		}
		fields = decutil.SplitFields(decutil.Bstr(line), fields)
		if len(fields) < nProps {
			return decodeErr(errBadVertex.Error() + " " + strconv.Itoa(i) + ": field count mismatch")
		}

		for j := range nProps {
			v, err := strconv.ParseFloat(fields[j], 64)
			if err != nil {
				return decodeErrCause(errBadVertex.Error(), err)
			}
			vals[j] = v
		}

		if hdr.xIdx >= 0 {
			var vx, vy, vz float32
			vx = float32(vals[hdr.xIdx])
			if hdr.yIdx >= 0 {
				vy = float32(vals[hdr.yIdx])
			}
			if hdr.zIdx >= 0 {
				vz = float32(vals[hdr.zIdx])
			}
			pos[i] = [3]float32{vx, vy, vz}
		}
		if norm != nil {
			var nx, ny, nz float32
			if hdr.nxIdx >= 0 {
				nx = float32(vals[hdr.nxIdx])
			}
			if hdr.nyIdx >= 0 {
				ny = float32(vals[hdr.nyIdx])
			}
			if hdr.nzIdx >= 0 {
				nz = float32(vals[hdr.nzIdx])
			}
			norm[i] = [3]float32{nx, ny, nz}
		}
		if col != nil {
			var cr, cg, cb, ca float32
			ca = 1.0
			if hdr.rIdx >= 0 {
				cr = float32(vals[hdr.rIdx])
			}
			if hdr.gIdx >= 0 {
				cg = float32(vals[hdr.gIdx])
			}
			if hdr.bIdx >= 0 {
				cb = float32(vals[hdr.bIdx])
			}
			if hdr.aIdx >= 0 {
				ca = float32(vals[hdr.aIdx])
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
				s = float32(vals[hdr.sIdx])
			}
			if hdr.tIdx >= 0 {
				t = float32(vals[hdr.tIdx])
			}
			uv[i] = [2]float32{s, t}
		}
	}
	return nil
}

func readASCIIFaces(sc *decutil.LineScanner, hdr *header) ([]uint32, error) {
	if hdr.faceCount == 0 {
		return nil, nil
	}
	indices := make([]uint32, 0, hdr.faceCount*minFaceVerts)
	var fields []string
	var stackFace [maxFaceVertsASCII]uint32

	for i := range hdr.faceCount {
		line := sc.Next()
		if line == nil {
			return nil, decodeErr(errBadFace.Error() + " " + strconv.Itoa(i))
		}
		fields = decutil.SplitFields(decutil.Bstr(line), fields)
		if len(fields) < 1 {
			return nil, decodeErr(errBadFace.Error() + " " + strconv.Itoa(i))
		}

		n, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, decodeErrCause(errBadFace.Error(), err)
		}
		if len(fields) < n+1 || n < minFaceVerts {
			continue
		}

		var faceVerts []uint32
		if n <= maxFaceVertsASCII {
			faceVerts = stackFace[:n]
		} else {
			faceVerts = make([]uint32, n)
		}
		for j := range n {
			v, err := strconv.Atoi(fields[j+1])
			if err != nil {
				return nil, decodeErrCause(errBadFace.Error(), err)
			}
			faceVerts[j] = uint32(v) //nolint:gosec // validated by Atoi
		}
		indices = fanFromSlice(indices, faceVerts)
	}
	return indices, nil
}

func readASCIIEdges(sc *decutil.LineScanner, hdr *header) ([]uint32, error) {
	if hdr.edgeCount == 0 {
		return nil, nil
	}
	edges := make([]uint32, 0, hdr.edgeCount*indicesPerEdge)
	var fields []string

	for i := range hdr.edgeCount {
		line := sc.Next()
		if line == nil {
			return nil, decodeErr(errBadEdge.Error() + " " + strconv.Itoa(i))
		}
		fields = decutil.SplitFields(decutil.Bstr(line), fields)
		if len(fields) < minEdgeFields {
			return nil, decodeErr(errBadEdge.Error() + " " + strconv.Itoa(i))
		}

		v1, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, decodeErrCause(errBadEdge.Error(), err)
		}
		v2, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, decodeErrCause(errBadEdge.Error(), err)
		}
		edges = append(edges, uint32(v1), uint32(v2)) //nolint:gosec // validated by Atoi
	}
	return edges, nil
}
