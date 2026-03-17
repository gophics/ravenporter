//nolint:mnd // Meshopt bitstream values are fixed by the format.
package gltf

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/internal/pool"
	"github.com/gophics/ravenporter/ir"
	"github.com/valyala/fastjson"
)

const (
	meshoptModeAttributes = "ATTRIBUTES"
	meshoptModeTriangles  = "TRIANGLES"
	meshoptModeIndices    = "INDICES"

	meshoptFilterNone        = "NONE"
	meshoptFilterOctahedral  = "OCTAHEDRAL"
	meshoptFilterQuaternion  = "QUATERNION"
	meshoptFilterExponential = "EXPONENTIAL"

	meshoptVertexHeader   = 0xa0
	meshoptIndexHeader    = 0xe0
	meshoptSequenceHeader = 0xd0

	meshoptDecodeVersion = 1

	meshoptVertexBlockBytes = 8192
	meshoptVertexBlockMax   = 256
	meshoptByteGroupSize    = 16

	meshoptVertexTailMinV0 = 32
	meshoptVertexTailMinV1 = 24

	meshoptTriangleTailSize = 16
	meshoptSequenceTailSize = 4
	meshoptMaxVarintBytes   = 5
	meshoptMaxByteStride    = 256

	meshoptDecodedNoteKey   = "gltf.meshopt"
	meshoptDecodedNoteValue = "decoded"
)

var meshoptVertexHeaderModesV0 = [4]int{0, 2, 4, 8}
var meshoptVertexHeaderModesV1Zero = [4]int{0, 1, 2, 4}
var meshoptVertexHeaderModesV1Delta = [4]int{1, 2, 4, 8}

type meshoptBufferView struct {
	sourceBuffer int
	sourceOffset int
	sourceLength int
	byteStride   int
	count        int
	mode         string
	filter       string
}

func parseMeshoptBufferView(ext *fastjson.Value) *meshoptBufferView {
	if ext == nil {
		return nil
	}
	view := &meshoptBufferView{
		sourceBuffer: ext.GetInt(keyBuffer),
		sourceOffset: ext.GetInt(keyByteOffset),
		sourceLength: ext.GetInt(keyByteLength),
		byteStride:   ext.GetInt(keyByteStride),
		count:        ext.GetInt(keyCount),
		mode:         decutil.Bstr(ext.GetStringBytes(keyMode)),
		filter:       decutil.Bstr(ext.GetStringBytes(keyFilter)),
	}
	if view.filter == "" {
		view.filter = meshoptFilterNone
	}
	return view
}

func (d *doc) resolveMeshoptBufferViews() error {
	decodedAny := false
	for i := range d.bufs.views {
		view := d.bufs.views[i]
		if view.meshopt == nil {
			continue
		}
		if err := validateMeshoptBufferView(i, view); err != nil {
			return err
		}
		src, err := d.bufs.slice(view.meshopt.sourceBuffer, view.meshopt.sourceOffset, view.meshopt.sourceLength)
		if err != nil {
			return meshoptBufferViewError(i, err.Error(), err)
		}
		dst := make([]byte, view.meshopt.count*view.meshopt.byteStride)
		if err := decodeMeshoptBuffer(
			dst,
			src,
			view.meshopt.count,
			view.meshopt.byteStride,
			view.meshopt.mode,
			view.meshopt.filter,
		); err != nil {
			return meshoptBufferViewError(i, err.Error(), err)
		}
		d.bufs.buffers = append(d.bufs.buffers, dst)
		d.bufs.views[i].buffer = len(d.bufs.buffers) - 1
		d.bufs.views[i].byteOffset = 0
		d.bufs.views[i].byteLength = len(dst)
		d.bufs.views[i].byteStride = view.meshopt.byteStride
		d.bufs.views[i].meshopt = nil
		decodedAny = true
	}
	if decodedAny && d.opts.Reporter != nil {
		d.opts.Reporter.AddProvenanceNote(meshoptDecodedNoteKey, meshoptDecodedNoteValue)
	}
	return nil
}

func validateMeshoptBufferView(index int, view bufferView) error {
	ext := view.meshopt
	if ext == nil {
		return nil
	}
	if ext.sourceBuffer < 0 {
		return meshoptBufferViewError(index, "invalid compressed buffer index", nil)
	}
	if ext.sourceOffset < 0 || ext.sourceLength <= 0 {
		return meshoptBufferViewError(index, "invalid compressed byte range", nil)
	}
	if ext.byteStride <= 0 {
		return meshoptBufferViewError(index, "invalid byteStride", nil)
	}
	if ext.count < 0 {
		return meshoptBufferViewError(index, "invalid count", nil)
	}
	if view.byteStride != 0 && view.byteStride != ext.byteStride {
		return meshoptBufferViewError(index, "bufferView byteStride does not match EXT_meshopt_compression byteStride", nil)
	}
	if view.byteLength != ext.count*ext.byteStride {
		return meshoptBufferViewError(index, "bufferView byteLength does not match EXT_meshopt_compression count*byteStride", nil)
	}
	switch ext.mode {
	case meshoptModeAttributes:
		if ext.byteStride > meshoptMaxByteStride || ext.byteStride%4 != 0 {
			return meshoptBufferViewError(index, "ATTRIBUTES byteStride must be divisible by 4 and <= 256", nil)
		}
	case meshoptModeTriangles:
		if ext.count%3 != 0 {
			return meshoptBufferViewError(index, "TRIANGLES count must be divisible by 3", nil)
		}
		if ext.byteStride != 2 && ext.byteStride != 4 {
			return meshoptBufferViewError(index, "TRIANGLES byteStride must be 2 or 4", nil)
		}
		if ext.filter != meshoptFilterNone {
			return meshoptBufferViewError(index, "TRIANGLES filter must be NONE", nil)
		}
	case meshoptModeIndices:
		if ext.byteStride != 2 && ext.byteStride != 4 {
			return meshoptBufferViewError(index, "INDICES byteStride must be 2 or 4", nil)
		}
		if ext.filter != meshoptFilterNone {
			return meshoptBufferViewError(index, "INDICES filter must be NONE", nil)
		}
	default:
		return meshoptBufferViewError(index, "unsupported meshopt mode "+ext.mode, nil)
	}
	switch ext.filter {
	case meshoptFilterNone:
	case meshoptFilterOctahedral:
		if ext.byteStride != 4 && ext.byteStride != 8 {
			return meshoptBufferViewError(index, "OCTAHEDRAL filter requires byteStride 4 or 8", nil)
		}
	case meshoptFilterQuaternion:
		if ext.byteStride != 8 {
			return meshoptBufferViewError(index, "QUATERNION filter requires byteStride 8", nil)
		}
	case meshoptFilterExponential:
		if ext.byteStride%4 != 0 {
			return meshoptBufferViewError(index, "EXPONENTIAL filter requires byteStride divisible by 4", nil)
		}
	default:
		return meshoptBufferViewError(index, "unsupported meshopt filter "+ext.filter, nil)
	}
	return nil
}

func meshoptBufferViewError(index int, detail string, err error) error {
	return decutil.DecodeErr(ir.FormatID(gltfName), fmt.Sprintf("%s bufferView %d: %s", extMeshopt, index, detail), err)
}

func (bs *bufferSet) slice(bufferIndex, byteOffset, byteLength int) ([]byte, error) {
	if bufferIndex < 0 || bufferIndex >= len(bs.buffers) {
		return nil, fmt.Errorf("buffer index %d out of bounds", bufferIndex)
	}
	buffer := bs.buffers[bufferIndex]
	if byteOffset < 0 || byteLength < 0 || byteOffset > len(buffer) || byteOffset+byteLength > len(buffer) {
		return nil, fmt.Errorf("byte range [%d:%d] out of bounds for buffer %d", byteOffset, byteOffset+byteLength, bufferIndex)
	}
	return buffer[byteOffset : byteOffset+byteLength], nil
}

func decodeMeshoptBuffer(dst, src []byte, count, byteStride int, mode, filter string) error {
	switch mode {
	case meshoptModeAttributes:
		if err := decodeMeshoptVertexBuffer(dst, src, count, byteStride); err != nil {
			return err
		}
		return applyMeshoptFilter(dst, count, byteStride, filter)
	case meshoptModeTriangles:
		return decodeMeshoptTriangleBuffer(dst, src, count, byteStride)
	case meshoptModeIndices:
		return decodeMeshoptIndexSequence(dst, src, count, byteStride)
	default:
		return fmt.Errorf("unsupported meshopt mode %s", mode)
	}
}

//nolint:funlen,gocyclo // Meshopt attribute decoding is a format-defined state machine.
func decodeMeshoptVertexBuffer(dst, src []byte, count, byteStride int) error {
	if len(dst) != count*byteStride {
		return fmt.Errorf("unexpected decoded vertex size")
	}
	if len(src) < 1 {
		return fmt.Errorf("meshopt vertex buffer is truncated")
	}
	header := src[0]
	if header&0xf0 != meshoptVertexHeader {
		return fmt.Errorf("invalid meshopt vertex header")
	}
	version := int(header & 0x0f)
	if version > meshoptDecodeVersion {
		return fmt.Errorf("unsupported meshopt vertex version %d", version)
	}
	tailSize := byteStride
	tailPad := meshoptVertexTailMinV0
	if version != 0 {
		tailSize += byteStride / 4
		tailPad = meshoptVertexTailMinV1
	}
	if tailPad < tailSize {
		tailPad = tailSize
	}
	if len(src) < 1+tailPad {
		return fmt.Errorf("meshopt vertex buffer is truncated")
	}
	dataLimit := len(src) - tailPad
	tail := src[len(src)-tailSize:]

	var last [meshoptMaxByteStride]byte
	copy(last[:byteStride], tail[:byteStride])

	var channels []byte
	if version != 0 {
		channels = tail[byteStride:]
	}

	maxBlockElements := meshoptVertexBlockSize(byteStride)
	deltas := pool.GetBuffer(maxBlockElements * byteStride)
	defer pool.PutBuffer(deltas)

	offset := 1
	for base := 0; base < count; base += maxBlockElements {
		blockCount := count - base
		if blockCount > maxBlockElements {
			blockCount = maxBlockElements
		}
		alignedCount := (blockCount + meshoptByteGroupSize - 1) &^ (meshoptByteGroupSize - 1)
		deltaBlock := deltas[:alignedCount*byteStride]
		clear(deltaBlock)

		controlOffset := offset
		if version != 0 {
			controlSize := byteStride / 4
			if offset+controlSize > dataLimit {
				return fmt.Errorf("meshopt vertex control data is truncated")
			}
			offset += controlSize
		}

		groupCount := alignedCount / meshoptByteGroupSize
		headerBytes := (groupCount + 3) / 4

		for byteIndex := 0; byteIndex < byteStride; byteIndex++ {
			controlMode := 0
			if version != 0 {
				controlMode = int((src[controlOffset+(byteIndex>>2)] >> ((byteIndex & 3) << 1)) & 0x03)
			}
			deltaBase := byteIndex * alignedCount

			switch controlMode {
			case 2:
				continue
			case 3:
				if offset+blockCount > dataLimit {
					return fmt.Errorf("meshopt vertex literal data is truncated")
				}
				copy(deltaBlock[deltaBase:deltaBase+blockCount], src[offset:offset+blockCount])
				offset += blockCount
				continue
			}

			if offset+headerBytes > dataLimit {
				return fmt.Errorf("meshopt vertex headers are truncated")
			}
			headers := src[offset : offset+headerBytes]
			offset += headerBytes

			for group := 0; group < groupCount; group++ {
				headerMode := int((headers[group>>2] >> ((group & 3) << 1)) & 0x03)
				modeBits := meshoptVertexModeBits(version, controlMode, headerMode)
				consumed, err := decodeMeshoptVertexGroup(deltaBlock[deltaBase+group*meshoptByteGroupSize:], src[offset:dataLimit], modeBits)
				if err != nil {
					return err
				}
				offset += consumed
			}
		}

		for element := 0; element < blockCount; element++ {
			dstOffset := (base + element) * byteStride
			for byteGroup := 0; byteGroup < byteStride; byteGroup += 4 {
				channelMode := 0
				rotateBits := 0
				if version != 0 {
					channel := channels[byteGroup>>2]
					channelMode = int(channel & 0x03)
					rotateBits = int(channel >> 4)
					if channelMode == 3 {
						return fmt.Errorf("invalid meshopt vertex channel mode")
					}
				}

				switch channelMode {
				case 0:
					for byteIndex := byteGroup; byteIndex < byteGroup+4; byteIndex++ {
						delta := meshoptDezigzag8(deltaBlock[byteIndex*alignedCount+element])
						value := last[byteIndex] + delta
						dst[dstOffset+byteIndex] = value
						last[byteIndex] = value
					}
				case 1:
					for byteIndex := byteGroup; byteIndex < byteGroup+4; byteIndex += 2 {
						delta := uint16(deltaBlock[byteIndex*alignedCount+element]) | uint16(deltaBlock[(byteIndex+1)*alignedCount+element])<<8
						value := uint16(last[byteIndex]) | uint16(last[byteIndex+1])<<8
						value += meshoptDezigzag16(delta)
						binary.LittleEndian.PutUint16(dst[dstOffset+byteIndex:], value)
						last[byteIndex] = byte(value)
						last[byteIndex+1] = byte(value >> 8)
					}
				case 2:
					delta := uint32(deltaBlock[byteGroup*alignedCount+element]) |
						uint32(deltaBlock[(byteGroup+1)*alignedCount+element])<<8 |
						uint32(deltaBlock[(byteGroup+2)*alignedCount+element])<<16 |
						uint32(deltaBlock[(byteGroup+3)*alignedCount+element])<<24
					value := uint32(last[byteGroup]) |
						uint32(last[byteGroup+1])<<8 |
						uint32(last[byteGroup+2])<<16 |
						uint32(last[byteGroup+3])<<24
					value ^= bits.RotateLeft32(delta, -rotateBits)
					binary.LittleEndian.PutUint32(dst[dstOffset+byteGroup:], value)
					last[byteGroup] = byte(value)
					last[byteGroup+1] = byte(value >> 8)
					last[byteGroup+2] = byte(value >> 16)
					last[byteGroup+3] = byte(value >> 24)
				}
			}
		}
	}

	if offset != dataLimit {
		return fmt.Errorf("meshopt vertex stream has trailing data")
	}
	return nil
}

func meshoptVertexModeBits(version, controlMode, headerMode int) int {
	if version == 0 {
		return meshoptVertexHeaderModesV0[headerMode]
	}
	if controlMode == 0 {
		return meshoptVertexHeaderModesV1Zero[headerMode]
	}
	return meshoptVertexHeaderModesV1Delta[headerMode]
}

func meshoptVertexBlockSize(byteStride int) int {
	blockSize := (meshoptVertexBlockBytes / byteStride) &^ (meshoptByteGroupSize - 1)
	if blockSize > meshoptVertexBlockMax {
		return meshoptVertexBlockMax
	}
	return blockSize
}

//nolint:funlen // The group decoder maps directly to the fixed sentinel encodings in the bitstream.
func decodeMeshoptVertexGroup(dst, src []byte, bitsPerValue int) (int, error) {
	switch bitsPerValue {
	case 0:
		clear(dst[:meshoptByteGroupSize])
		return 0, nil
	case 1:
		if len(src) < 2 {
			return 0, fmt.Errorf("meshopt vertex group is truncated")
		}
		offset := 2
		for i := 0; i < meshoptByteGroupSize; i++ {
			value := (src[i>>3] >> (i & 7)) & 0x01
			if value == 1 {
				if offset >= len(src) {
					return 0, fmt.Errorf("meshopt vertex sentinel data is truncated")
				}
				value = src[offset]
				offset++
			}
			dst[i] = value
		}
		return offset, nil
	case 2:
		if len(src) < 4 {
			return 0, fmt.Errorf("meshopt vertex group is truncated")
		}
		offset := 4
		for i := 0; i < meshoptByteGroupSize; i++ {
			shift := 6 - ((i & 3) << 1)
			value := (src[i>>2] >> shift) & 0x03
			if value == 0x03 {
				if offset >= len(src) {
					return 0, fmt.Errorf("meshopt vertex sentinel data is truncated")
				}
				value = src[offset]
				offset++
			}
			dst[i] = value
		}
		return offset, nil
	case 4:
		if len(src) < 8 {
			return 0, fmt.Errorf("meshopt vertex group is truncated")
		}
		offset := 8
		for i := 0; i < meshoptByteGroupSize; i++ {
			shift := 4 - ((i & 1) << 2)
			value := (src[i>>1] >> shift) & 0x0f
			if value == 0x0f {
				if offset >= len(src) {
					return 0, fmt.Errorf("meshopt vertex sentinel data is truncated")
				}
				value = src[offset]
				offset++
			}
			dst[i] = value
		}
		return offset, nil
	case 8:
		if len(src) < meshoptByteGroupSize {
			return 0, fmt.Errorf("meshopt vertex group is truncated")
		}
		copy(dst[:meshoptByteGroupSize], src[:meshoptByteGroupSize])
		return meshoptByteGroupSize, nil
	default:
		return 0, fmt.Errorf("invalid meshopt vertex group mode")
	}
}

func applyMeshoptFilter(dst []byte, count, byteStride int, filter string) error {
	switch filter {
	case meshoptFilterNone:
		return nil
	case meshoptFilterOctahedral:
		return applyMeshoptOctahedralFilter(dst, count, byteStride)
	case meshoptFilterQuaternion:
		return applyMeshoptQuaternionFilter(dst, count)
	case meshoptFilterExponential:
		return applyMeshoptExponentialFilter(dst, count, byteStride)
	default:
		return fmt.Errorf("unsupported meshopt filter %s", filter)
	}
}

func applyMeshoptOctahedralFilter(dst []byte, count, byteStride int) error {
	for i := 0; i < count; i++ {
		offset := i * byteStride
		if byteStride == 4 {
			x := float64(int8(dst[offset+0]))
			y := float64(int8(dst[offset+1]))
			one := float64(int8(dst[offset+2]))
			x /= one
			y /= one
			z := 1 - math.Abs(x) - math.Abs(y)
			t := math.Max(-z, 0)
			if x >= 0 {
				x -= t
			} else {
				x += t
			}
			if y >= 0 {
				y -= t
			} else {
				y += t
			}
			scale := 127.0 / math.Sqrt(x*x+y*y+z*z)
			dst[offset+0] = byte(int8(math.Round(x * scale)))
			dst[offset+1] = byte(int8(math.Round(y * scale)))
			dst[offset+2] = byte(int8(math.Round(z * scale)))
			continue
		}

		x := float64(readInt16LE(dst[offset+0:]))
		y := float64(readInt16LE(dst[offset+2:]))
		one := float64(readInt16LE(dst[offset+4:]))
		x /= one
		y /= one
		z := 1 - math.Abs(x) - math.Abs(y)
		t := math.Max(-z, 0)
		if x >= 0 {
			x -= t
		} else {
			x += t
		}
		if y >= 0 {
			y -= t
		} else {
			y += t
		}
		scale := 32767.0 / math.Sqrt(x*x+y*y+z*z)
		writeInt16LE(dst[offset+0:], int16(math.Round(x*scale)))
		writeInt16LE(dst[offset+2:], int16(math.Round(y*scale)))
		writeInt16LE(dst[offset+4:], int16(math.Round(z*scale)))
	}
	return nil
}

func applyMeshoptQuaternionFilter(dst []byte, count int) error {
	for i := 0; i < count; i++ {
		offset := i * 8
		inputW := int(readInt16LE(dst[offset+6:]))
		maxComponent := inputW & 0x03
		scale := (1 / math.Sqrt2) / float64(inputW|0x03)
		x := float64(readInt16LE(dst[offset+0:])) * scale
		y := float64(readInt16LE(dst[offset+2:])) * scale
		z := float64(readInt16LE(dst[offset+4:])) * scale
		w := math.Sqrt(math.Max(0, 1-x*x-y*y-z*z))

		values := [4]int16{}
		values[(maxComponent+1)%4] = int16(math.Round(x * 32767))
		values[(maxComponent+2)%4] = int16(math.Round(y * 32767))
		values[(maxComponent+3)%4] = int16(math.Round(z * 32767))
		values[maxComponent] = int16(math.Round(w * 32767))

		writeInt16LE(dst[offset+0:], values[0])
		writeInt16LE(dst[offset+2:], values[1])
		writeInt16LE(dst[offset+4:], values[2])
		writeInt16LE(dst[offset+6:], values[3])
	}
	return nil
}

func applyMeshoptExponentialFilter(dst []byte, count, byteStride int) error {
	wordCount := (count * byteStride) / 4
	for i := 0; i < wordCount; i++ {
		offset := i * 4
		raw := binary.LittleEndian.Uint32(dst[offset:])
		exp := int(raw >> 24)
		if exp >= 0x80 {
			exp -= 0x100
		}
		mantissa := int64(raw & 0x00ffffff)
		if mantissa&0x00800000 != 0 {
			mantissa -= 1 << 24
		}
		f := float32(math.Ldexp(float64(mantissa), exp))
		binary.LittleEndian.PutUint32(dst[offset:], math.Float32bits(f))
	}
	return nil
}

//nolint:funlen,gocyclo // Meshopt triangle decoding is a format-defined state machine.
func decodeMeshoptTriangleBuffer(dst, src []byte, count, byteStride int) error {
	if len(dst) != count*byteStride {
		return fmt.Errorf("unexpected decoded triangle size")
	}
	if len(src) < 1+count/3+meshoptTriangleTailSize {
		return fmt.Errorf("meshopt triangle buffer is truncated")
	}
	header := src[0]
	if header&0xf0 != meshoptIndexHeader {
		return fmt.Errorf("invalid meshopt triangle header")
	}
	version := int(header & 0x0f)
	if version > meshoptDecodeVersion {
		return fmt.Errorf("unsupported meshopt triangle version %d", version)
	}

	fecmax := 15
	if version >= 1 {
		fecmax = 13
	}

	codeOffset := 1
	dataOffset := codeOffset + count/3
	dataSafeEnd := len(src) - meshoptTriangleTailSize
	codeauxTable := src[dataSafeEnd:]

	var edgeFIFO [16][2]uint32
	var vertexFIFO [16]uint32
	for i := range edgeFIFO {
		edgeFIFO[i][0] = ^uint32(0)
		edgeFIFO[i][1] = ^uint32(0)
	}
	for i := range vertexFIFO {
		vertexFIFO[i] = ^uint32(0)
	}

	edgeOffset := 0
	vertexOffset := 0
	var next uint32
	var last uint32

	for i := 0; i < count; i += 3 {
		if dataOffset > dataSafeEnd {
			return fmt.Errorf("meshopt triangle data is truncated")
		}

		codeTri := src[codeOffset]
		codeOffset++

		if codeTri < 0xf0 {
			fe := int(codeTri >> 4)
			edgeIndex := (edgeOffset - 1 - fe) & 15
			a := edgeFIFO[edgeIndex][0]
			b := edgeFIFO[edgeIndex][1]

			fec := int(codeTri & 0x0f)
			var c uint32
			if fec < fecmax {
				fifoIndex := (vertexOffset - 1 - fec) & 15
				if fec == 0 {
					c = next
					next++
					pushMeshoptVertex(&vertexFIFO, c, &vertexOffset, true)
				} else {
					c = vertexFIFO[fifoIndex]
				}
			} else {
				if fec != 15 {
					if fec == 13 {
						last--
					} else {
						last++
					}
					c = last
				} else {
					value, err := meshoptDecodeIndexDelta(src, &dataOffset, dataSafeEnd, last)
					if err != nil {
						return err
					}
					last = value
					c = value
				}
				pushMeshoptVertex(&vertexFIFO, c, &vertexOffset, true)
			}

			pushMeshoptEdge(&edgeFIFO, c, b, &edgeOffset)
			pushMeshoptEdge(&edgeFIFO, a, c, &edgeOffset)
			if err := writeMeshoptIndexTriangle(dst, i, byteStride, a, b, c); err != nil {
				return err
			}
			continue
		}

		if codeTri < 0xfe {
			codeAux := codeauxTable[int(codeTri&0x0f)]
			feb := int(codeAux >> 4)
			fec := int(codeAux & 0x0f)

			a := next
			next++

			var b uint32
			if feb == 0 {
				b = next
				next++
			} else {
				b = vertexFIFO[(vertexOffset-feb)&15]
			}

			var c uint32
			if fec == 0 {
				c = next
				next++
			} else {
				c = vertexFIFO[(vertexOffset-fec)&15]
			}

			if err := writeMeshoptIndexTriangle(dst, i, byteStride, a, b, c); err != nil {
				return err
			}
			pushMeshoptVertex(&vertexFIFO, a, &vertexOffset, true)
			pushMeshoptVertex(&vertexFIFO, b, &vertexOffset, feb == 0)
			pushMeshoptVertex(&vertexFIFO, c, &vertexOffset, fec == 0)
			pushMeshoptEdge(&edgeFIFO, b, a, &edgeOffset)
			pushMeshoptEdge(&edgeFIFO, c, b, &edgeOffset)
			pushMeshoptEdge(&edgeFIFO, a, c, &edgeOffset)
			continue
		}

		if dataOffset >= dataSafeEnd {
			return fmt.Errorf("meshopt triangle codeaux data is truncated")
		}
		codeAux := src[dataOffset]
		dataOffset++

		fea := 15
		if codeTri == 0xfe {
			fea = 0
		}
		feb := int(codeAux >> 4)
		fec := int(codeAux & 0x0f)

		if codeAux == 0 {
			next = 0
		}

		var a uint32
		if fea == 0 {
			a = next
			next++
		}

		var b uint32
		if feb == 0 {
			b = next
			next++
		} else {
			b = vertexFIFO[(vertexOffset-feb)&15]
		}

		var c uint32
		if fec == 0 {
			c = next
			next++
		} else {
			c = vertexFIFO[(vertexOffset-fec)&15]
		}

		if fea == 15 {
			value, err := meshoptDecodeIndexDelta(src, &dataOffset, dataSafeEnd, last)
			if err != nil {
				return err
			}
			last = value
			a = value
		}
		if feb == 15 {
			value, err := meshoptDecodeIndexDelta(src, &dataOffset, dataSafeEnd, last)
			if err != nil {
				return err
			}
			last = value
			b = value
		}
		if fec == 15 {
			value, err := meshoptDecodeIndexDelta(src, &dataOffset, dataSafeEnd, last)
			if err != nil {
				return err
			}
			last = value
			c = value
		}

		if err := writeMeshoptIndexTriangle(dst, i, byteStride, a, b, c); err != nil {
			return err
		}
		pushMeshoptVertex(&vertexFIFO, a, &vertexOffset, true)
		pushMeshoptVertex(&vertexFIFO, b, &vertexOffset, feb == 0 || feb == 15)
		pushMeshoptVertex(&vertexFIFO, c, &vertexOffset, fec == 0 || fec == 15)
		pushMeshoptEdge(&edgeFIFO, b, a, &edgeOffset)
		pushMeshoptEdge(&edgeFIFO, c, b, &edgeOffset)
		pushMeshoptEdge(&edgeFIFO, a, c, &edgeOffset)
	}

	if dataOffset != dataSafeEnd {
		return fmt.Errorf("meshopt triangle stream has trailing data")
	}
	return nil
}

func decodeMeshoptIndexSequence(dst, src []byte, count, byteStride int) error {
	if len(dst) != count*byteStride {
		return fmt.Errorf("unexpected decoded index sequence size")
	}
	if len(src) < 1+count+meshoptSequenceTailSize {
		return fmt.Errorf("meshopt index sequence is truncated")
	}
	header := src[0]
	if header&0xf0 != meshoptSequenceHeader {
		return fmt.Errorf("invalid meshopt index sequence header")
	}
	version := int(header & 0x0f)
	if version > meshoptDecodeVersion {
		return fmt.Errorf("unsupported meshopt index sequence version %d", version)
	}

	dataOffset := 1
	dataSafeEnd := len(src) - meshoptSequenceTailSize
	var last [2]uint32

	for i := 0; i < count; i++ {
		value, err := meshoptDecodeVarint(src, &dataOffset, dataSafeEnd)
		if err != nil {
			return err
		}
		current := value & 0x01
		value >>= 1
		index := last[current] + meshoptDezigzag32(value)
		last[current] = index
		if err := writeMeshoptIndex(dst, i, byteStride, index); err != nil {
			return err
		}
	}

	if dataOffset != dataSafeEnd {
		return fmt.Errorf("meshopt index sequence has trailing data")
	}
	return nil
}

func meshoptDecodeVarint(src []byte, offset *int, limit int) (uint32, error) {
	var result uint32
	var shift uint
	for i := 0; i < meshoptMaxVarintBytes; i++ {
		if *offset >= limit {
			return 0, fmt.Errorf("meshopt varint is truncated")
		}
		group := src[*offset]
		*offset++
		result |= uint32(group&0x7f) << shift
		if group < 0x80 {
			return result, nil
		}
		shift += 7
	}
	return 0, fmt.Errorf("meshopt varint is malformed")
}

func meshoptDecodeIndexDelta(src []byte, offset *int, limit int, last uint32) (uint32, error) {
	value, err := meshoptDecodeVarint(src, offset, limit)
	if err != nil {
		return 0, err
	}
	return last + meshoptDezigzag32(value), nil
}

func pushMeshoptVertex(fifo *[16]uint32, value uint32, offset *int, advance bool) {
	fifo[*offset] = value
	if advance {
		*offset = (*offset + 1) & 15
	}
}

func pushMeshoptEdge(fifo *[16][2]uint32, a, b uint32, offset *int) {
	fifo[*offset][0] = a
	fifo[*offset][1] = b
	*offset = (*offset + 1) & 15
}

func writeMeshoptIndexTriangle(dst []byte, base, byteStride int, a, b, c uint32) error {
	if err := writeMeshoptIndex(dst, base+0, byteStride, a); err != nil {
		return err
	}
	if err := writeMeshoptIndex(dst, base+1, byteStride, b); err != nil {
		return err
	}
	return writeMeshoptIndex(dst, base+2, byteStride, c)
}

func writeMeshoptIndex(dst []byte, index, byteStride int, value uint32) error {
	offset := index * byteStride
	if byteStride == 2 {
		if value > 0xffff {
			return fmt.Errorf("meshopt index exceeds 16-bit range")
		}
		dst[offset+0] = byte(value & 0xff)
		dst[offset+1] = byte((value >> 8) & 0xff)
		return nil
	}
	binary.LittleEndian.PutUint32(dst[offset:], value)
	return nil
}

func meshoptDezigzag8(value uint8) uint8 {
	return (value >> 1) ^ (uint8(0) - (value & 0x01))
}

func meshoptDezigzag16(value uint16) uint16 {
	return (value >> 1) ^ (uint16(0) - (value & 0x0001))
}

func meshoptDezigzag32(value uint32) uint32 {
	return (value >> 1) ^ (uint32(0) - (value & 0x00000001))
}

func readInt16LE(data []byte) int16 {
	return int16(int8(data[1]))<<8 | int16(data[0])
}

func writeInt16LE(data []byte, value int16) {
	raw := int32(value)
	data[0] = byte(raw & 0xff)
	data[1] = byte((raw >> 8) & 0xff)
}
