package fbx

import (
	"context"
	"errors"
	"unsafe"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pool"
)

const (
	nodeHeaderSize32 = 13
	nodeHeaderSize64 = 25
	nullNodeSize32   = 13

	propBool    = 'C'
	propInt16   = 'Y'
	propInt32   = 'I'
	propInt64   = 'L'
	propFloat32 = 'F'
	propFloat64 = 'D'
	propString  = 'S'
	propRaw     = 'R'

	propArrBool    = 'b'
	propArrInt32   = 'i'
	propArrInt64   = 'l'
	propArrFloat32 = 'f'
	propArrFloat64 = 'd'

	arrHeaderSize  = 12
	encodingDeflat = 1

	sizeBool   = 1
	sizeInt16  = 2
	sizeInt32  = 4
	sizeInt64  = 8
	sizeFloat  = 4
	sizeDouble = 8
	sizeLenPfx = 4
)

var (
	errTooShort    = errors.New("file too short")
	errBadNode     = errors.New("invalid node record")
	errBadProperty = errors.New("invalid property record")
	errBadArray    = errors.New("invalid array property")
)

var zlibPool pool.ZlibReader

type fbxNode struct {
	name       string
	properties []fbxProp
	children   []fbxNode
}

type fbxProp struct {
	typecode byte
	boolVal  bool
	intVal   int64
	floatVal float64
	strVal   string
	rawVal   []byte
	arrI32   []int32
	arrI64   []int64
	arrF32   []float32
	arrF64   []float64
}

const maxNodeDepth = 64

var errMaxDepth = errors.New("maximum node nesting depth exceeded")

type parseCtx struct {
	nodes pool.Arena[fbxNode]
	props pool.Arena[fbxProp]
}

func parseNodesAt(sysCtx context.Context, data []byte, off int, version uint32, depth int, ctx *parseCtx) ([]fbxNode, error) {
	if depth > maxNodeDepth {
		return nil, errMaxDepth
	}
	nodes := ctx.nodes.Alloc(8)[:0] //nolint:mnd // capacity hint
	for {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		node, nextOff, err := parseNodeAt(sysCtx, data, off, version, depth, ctx)
		if err != nil {
			return nil, err
		}
		if nextOff == off {
			break
		}
		off = nextOff
		if node.name != "" {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func parseNodeAt(sysCtx context.Context, data []byte, off int, version uint32, depth int, ctx *parseCtx) (fbxNode, int, error) {
	is64 := version >= v7500
	hdrSize := nodeHeaderSize32
	if is64 {
		hdrSize = nodeHeaderSize64
	}

	if off+hdrSize > len(data) {
		return fbxNode{}, off, nil
	}

	var endOff, propCount, propListLen uint64

	if is64 {
		endOff = binread.ReadU64LE(data[off:])
		propCount = binread.ReadU64LE(data[off+8:])
		propListLen = binread.ReadU64LE(data[off+16:])
	} else {
		endOff = uint64(binread.ReadU32LE(data[off:]))
		propCount = uint64(binread.ReadU32LE(data[off+4:]))
		propListLen = uint64(binread.ReadU32LE(data[off+8:]))
	}

	if endOff == 0 && propCount == 0 && propListLen == 0 {
		return fbxNode{}, off, nil
	}

	nameOff := off + hdrSize
	if nameOff >= len(data) {
		return fbxNode{}, 0, errBadNode
	}

	nameLen := int(data[nameOff-1])
	if nameOff+nameLen > len(data) {
		return fbxNode{}, 0, errBadNode
	}
	name := decutil.Bstr(data[nameOff : nameOff+nameLen])

	propStart := nameOff + nameLen
	props, propBytes, err := parseProperties(data, propStart, int(propCount), &ctx.props) //nolint:gosec // FBX file bounded
	if err != nil {
		return fbxNode{}, 0, err
	}
	_ = propListLen
	_ = propBytes

	node := fbxNode{
		name:       name,
		properties: props,
	}

	childStart := propStart + propBytes
	childEnd := int(endOff) //nolint:gosec // FBX file bounded
	if childEnd > childStart && childEnd <= len(data) {
		node.children, err = parseNodesAt(sysCtx, data, childStart, version, depth+1, ctx)
		if err != nil {
			return fbxNode{}, 0, err
		}
	}

	if childEnd <= off {
		return fbxNode{}, off + hdrSize, nil
	}
	return node, childEnd, nil
}

func parseProperties(data []byte, off, count int, pa *pool.Arena[fbxProp]) ([]fbxProp, int, error) {
	if count > 10000 || count < 0 {
		return nil, 0, errBadProperty
	}
	props := pa.Alloc(count)

	pos := off
	for i := range count {
		if pos >= len(data) {
			return nil, 0, errBadProperty
		}
		tc := data[pos]
		pos++

		props[i].typecode = tc
		n, err := parseSingleProp(data, pos, tc, &props[i])
		if err != nil {
			return nil, 0, err
		}
		pos += n
	}
	return props, pos - off, nil
}

func parseSingleProp(data []byte, pos int, tc byte, prop *fbxProp) (int, error) {
	switch tc {
	case propBool:
		if pos >= len(data) {
			return 0, errBadProperty
		}
		prop.boolVal = data[pos] != 0
		return sizeBool, nil
	case propInt16:
		if pos+sizeInt16 > len(data) {
			return 0, errBadProperty
		}
		prop.intVal = int64(binread.ReadI16LE(data[pos:]))
		return sizeInt16, nil
	case propInt32:
		if pos+sizeInt32 > len(data) {
			return 0, errBadProperty
		}
		prop.intVal = int64(binread.ReadI32LE(data[pos:]))
		return sizeInt32, nil
	case propInt64:
		if pos+sizeInt64 > len(data) {
			return 0, errBadProperty
		}
		prop.intVal = int64(binread.ReadU64LE(data[pos:])) //nolint:gosec // intentional
		return sizeInt64, nil
	case propFloat32:
		if pos+sizeFloat > len(data) {
			return 0, errBadProperty
		}
		prop.floatVal = float64(binread.ReadF32LE(data[pos:]))
		return sizeFloat, nil
	case propFloat64:
		if pos+sizeDouble > len(data) {
			return 0, errBadProperty
		}
		prop.floatVal = binread.ReadF64LE(data[pos:])
		return sizeDouble, nil
	case propString, propRaw:
		return parseStringOrRaw(data, pos, tc, prop)
	case propArrBool, propArrInt32, propArrInt64, propArrFloat32, propArrFloat64:
		return readArrayProp(data, pos, tc, prop)
	default:
		return 0, errBadProperty
	}
}

func parseStringOrRaw(data []byte, pos int, tc byte, prop *fbxProp) (int, error) {
	if pos+sizeLenPfx > len(data) {
		return 0, errBadProperty
	}
	slen := int(binread.ReadU32LE(data[pos:]))
	pos += sizeLenPfx
	if pos+slen > len(data) {
		return 0, errBadProperty
	}
	if tc == propString {
		prop.strVal = decutil.Bstr(data[pos : pos+slen])
	} else {
		prop.rawVal = data[pos : pos+slen]
	}
	return sizeLenPfx + slen, nil
}

func readArrayProp(data []byte, pos int, tc byte, prop *fbxProp) (int, error) {
	if pos+arrHeaderSize > len(data) {
		return 0, errBadArray
	}
	count := int(binread.ReadU32LE(data[pos:]))
	encoding := binread.ReadU32LE(data[pos+4:])
	compLen := int(binread.ReadU32LE(data[pos+8:]))
	pos += arrHeaderSize

	if pos+compLen > len(data) {
		return 0, errBadArray
	}

	raw := data[pos : pos+compLen]
	if encoding == encodingDeflat {
		exactSize := 0
		switch tc {
		case propArrBool:
			exactSize = count * sizeBool
		case propArrInt32:
			exactSize = count * sizeInt32
		case propArrInt64:
			exactSize = count * sizeInt64
		case propArrFloat32:
			exactSize = count * sizeFloat
		case propArrFloat64:
			exactSize = count * sizeDouble
		}
		decompressed, err := zlibDecompressSized(raw, exactSize)
		if err != nil {
			return 0, errBadArray
		}
		raw = decompressed
	}

	switch tc {
	case propArrBool:
		prop.rawVal = raw
	case propArrInt32:
		prop.arrI32 = castI32Slice(raw, count)
	case propArrInt64:
		prop.arrI64 = castI64Slice(raw, count)
	case propArrFloat32:
		prop.arrF32 = castF32Slice(raw, count)
	case propArrFloat64:
		prop.arrF64 = castF64Slice(raw, count)
	}

	return arrHeaderSize + compLen, nil
}

func zlibDecompressSized(compressed []byte, exactSize int) ([]byte, error) {
	if exactSize <= 0 {
		return zlibPool.Decompress(compressed)
	}
	dst := make([]byte, exactSize)
	if err := zlibPool.DecompressInto(dst, compressed); err != nil {
		return nil, err
	}
	return dst, nil
}

const (
	sizeI32 = int(unsafe.Sizeof(int32(0)))
	sizeI64 = int(unsafe.Sizeof(int64(0)))
	sizeF32 = int(unsafe.Sizeof(float32(0)))
	sizeF64 = int(unsafe.Sizeof(float64(0)))
)

func castI32Slice(data []byte, count int) []int32 {
	need := count * sizeI32
	if len(data) < need {
		return readI32SliceSafe(data, count)
	}
	return unsafe.Slice((*int32)(unsafe.Pointer(&data[0])), count) //nolint:gosec // bounds checked above
}

func castI64Slice(data []byte, count int) []int64 {
	need := count * sizeI64
	if len(data) < need {
		return readI64SliceSafe(data, count)
	}
	return unsafe.Slice((*int64)(unsafe.Pointer(&data[0])), count) //nolint:gosec // bounds checked above
}

func castF32Slice(data []byte, count int) []float32 {
	need := count * sizeF32
	if len(data) < need {
		return readF32SliceSafe(data, count)
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(&data[0])), count) //nolint:gosec // bounds checked above
}

func castF64Slice(data []byte, count int) []float64 {
	need := count * sizeF64
	if len(data) < need {
		return readF64SliceSafe(data, count)
	}
	return unsafe.Slice((*float64)(unsafe.Pointer(&data[0])), count) //nolint:gosec // bounds checked above
}

func readI32SliceSafe(data []byte, count int) []int32 {
	out := make([]int32, count)
	for i := range count {
		off := i * sizeI32
		if off+sizeI32 > len(data) {
			break
		}
		out[i] = binread.ReadI32LE(data[off:])
	}
	return out
}

func readI64SliceSafe(data []byte, count int) []int64 {
	out := make([]int64, count)
	for i := range count {
		off := i * sizeI64
		if off+sizeI64 > len(data) {
			break
		}
		out[i] = int64(binread.ReadU64LE(data[off:])) //nolint:gosec // intentional
	}
	return out
}

func readF32SliceSafe(data []byte, count int) []float32 {
	out := make([]float32, count)
	for i := range count {
		off := i * sizeF32
		if off+sizeF32 > len(data) {
			break
		}
		out[i] = binread.ReadF32LE(data[off:])
	}
	return out
}

func readF64SliceSafe(data []byte, count int) []float64 {
	out := make([]float64, count)
	for i := range count {
		off := i * sizeF64
		if off+sizeF64 > len(data) {
			break
		}
		out[i] = binread.ReadF64LE(data[off:])
	}
	return out
}
