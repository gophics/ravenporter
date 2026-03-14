package ply

import (
	"strconv"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatASCII    = "ascii"
	formatBinaryLE = "binary_little_endian"
	formatBinaryBE = "binary_big_endian"

	elemVertex = "vertex"
	elemFace   = "face"
	elemEdge   = "edge"

	hdrEndHeader = "end_header"
	hdrFormat    = "format"
	hdrElement   = "element"
	hdrProperty  = "property"
	hdrList      = "list"

	minFormatFields  = 2
	minListFields    = 5
	minElementFields = 3
	maxInitProps     = 12

	propNameX     = "x"
	propNameY     = "y"
	propNameZ     = "z"
	propNameNX    = "nx"
	propNameNY    = "ny"
	propNameNZ    = "nz"
	propNameRed   = "red"
	propNameGreen = "green"
	propNameBlue  = "blue"
	propNameAlpha = "alpha"
	propNameS     = "s"
	propNameT     = "t"
	propNameTexU  = "texture_u"
	propNameTexV  = "texture_v"

	edgePropV1 = "vertex1"
	edgePropV2 = "vertex2"

	hdrMagic = "ply"
)

type propType int

const (
	propFloat propType = iota
	propDouble
	propChar
	propUchar
	propInt
	propShort
	propUint
	propUshort
)

const (
	sizeByte   = 1
	sizeShort  = 2
	sizeInt    = 4
	sizeDouble = 8
)

type property struct {
	name string
	typ  propType
}

type header struct {
	format      string
	vertexCount int
	faceCount   int
	edgeCount   int
	props       []property
	offsets     []int
	stride      int
	hasNormals  bool
	hasColors   bool
	hasTexCoord bool

	xIdx, yIdx, zIdx       int
	nxIdx, nyIdx, nzIdx    int
	rIdx, gIdx, bIdx, aIdx int
	sIdx, tIdx             int
	faceListType           propType
	faceIndexType          propType
	edgeVertex1Type        propType
	edgeVertex2Type        propType
}

func parseHeader(raw []byte) (*header, []byte, error) {
	sc := decutil.LineScanner{Data: raw}

	first := sc.Next()
	if first == nil || decutil.Bstr(first) != hdrMagic {
		return nil, nil, decodeErr(errBadHeader.Error())
	}

	hdr := &header{
		xIdx: -1, yIdx: -1, zIdx: -1,
		nxIdx: -1, nyIdx: -1, nzIdx: -1,
		rIdx: -1, gIdx: -1, bIdx: -1, aIdx: -1,
		sIdx: -1, tIdx: -1,
		props: make([]property, 0, maxInitProps),
	}

	currentElement := ""
	var fields []string
	for {
		line := sc.Next()
		if line == nil {
			return nil, nil, decodeErr(errBadHeader.Error())
		}
		trimmed := decutil.Bstr(line)
		if trimmed == hdrEndHeader {
			break
		}

		var err error
		currentElement, err = parseHeaderLine(hdr, trimmed, currentElement, &fields)
		if err != nil {
			return nil, nil, err
		}
	}

	hdr.hasNormals = hdr.nxIdx >= 0 && hdr.nyIdx >= 0 && hdr.nzIdx >= 0
	hdr.hasColors = hdr.rIdx >= 0 && hdr.gIdx >= 0 && hdr.bIdx >= 0
	hdr.hasTexCoord = hdr.sIdx >= 0 && hdr.tIdx >= 0

	hdr.offsets = make([]int, len(hdr.props))
	off := 0
	for i, p := range hdr.props {
		hdr.offsets[i] = off
		off += propSize(p.typ)
	}
	hdr.stride = off

	body := raw[sc.Pos:]
	return hdr, body, nil
}

func parseHeaderLine(hdr *header, line, currentElement string, fields *[]string) (string, error) {
	*fields = decutil.SplitFields(line, *fields)
	f := *fields
	if len(f) == 0 {
		return currentElement, nil
	}

	switch f[0] {
	case hdrFormat:
		if len(f) < minFormatFields {
			return "", decodeErr(errBadFormat.Error())
		}
		hdr.format = f[1]
	case hdrElement:
		if len(f) < minElementFields {
			return currentElement, nil
		}
		currentElement = f[1]
		count, err := strconv.Atoi(f[2])
		if err != nil {
			return "", decodeErrCause(errBadHeader.Error(), err)
		}
		switch currentElement {
		case elemVertex:
			hdr.vertexCount = count
		case elemFace:
			hdr.faceCount = count
		case elemEdge:
			hdr.edgeCount = count
		}
	case hdrProperty:
		if currentElement == elemVertex && len(f) >= minElementFields {
			pt, err := parseType(f[1])
			if err != nil {
				return "", err
			}
			idx := len(hdr.props)
			hdr.props = append(hdr.props, property{name: f[2], typ: pt})
			mapPropertyIndex(hdr, f[2], idx)
		}
		if currentElement == elemFace && len(f) >= minListFields && f[1] == hdrList {
			lt, err := parseType(f[2])
			if err != nil {
				return "", err
			}
			it, err := parseType(f[minElementFields])
			if err != nil {
				return "", err
			}
			hdr.faceListType = lt
			hdr.faceIndexType = it
		}
		if currentElement == elemEdge && len(f) >= minElementFields {
			pt, err := parseType(f[1])
			if err != nil {
				return "", err
			}
			switch f[2] {
			case edgePropV1:
				hdr.edgeVertex1Type = pt
			case edgePropV2:
				hdr.edgeVertex2Type = pt
			}
		}
	}
	return currentElement, nil
}

func mapPropertyIndex(hdr *header, name string, idx int) {
	switch name {
	case propNameX:
		hdr.xIdx = idx
	case propNameY:
		hdr.yIdx = idx
	case propNameZ:
		hdr.zIdx = idx
	case propNameNX:
		hdr.nxIdx = idx
	case propNameNY:
		hdr.nyIdx = idx
	case propNameNZ:
		hdr.nzIdx = idx
	case propNameRed:
		hdr.rIdx = idx
	case propNameGreen:
		hdr.gIdx = idx
	case propNameBlue:
		hdr.bIdx = idx
	case propNameAlpha:
		hdr.aIdx = idx
	case propNameS, propNameTexU:
		hdr.sIdx = idx
	case propNameT, propNameTexV:
		hdr.tIdx = idx
	}
}

func parseType(s string) (propType, error) {
	switch s {
	case "float", "float32":
		return propFloat, nil
	case "double", "float64":
		return propDouble, nil
	case "char", "int8":
		return propChar, nil
	case "uchar", "uint8":
		return propUchar, nil
	case "int", "int32":
		return propInt, nil
	case "short", "int16":
		return propShort, nil
	case "uint", "uint32":
		return propUint, nil
	case "ushort", "uint16":
		return propUshort, nil
	default:
		return 0, decodeErr(errBadProperty.Error() + ": " + s)
	}
}

func propSize(t propType) int {
	switch t {
	case propChar, propUchar:
		return sizeByte
	case propShort, propUshort:
		return sizeShort
	case propFloat, propInt, propUint:
		return sizeInt
	case propDouble:
		return sizeDouble
	default:
		return 0
	}
}

func decodeErr(msg string) error {
	return decutil.DecodeErr(ir.FormatPLY, msg, nil)
}

func decodeErrCause(msg string, cause error) error {
	return decutil.DecodeErr(ir.FormatPLY, msg, cause)
}
