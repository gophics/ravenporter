package tds

import (
	"math"

	"github.com/gophics/ravenporter/internal/binread"
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

const (
	chunkKeyframer      = 0xB000
	chunkMeshTrackNode  = 0xB002
	chunkCamTrackNode   = 0xB003
	chunkLightTrackNode = 0xB006
	chunkSpotTrackNode  = 0xB007
	chunkTrackObjName   = 0xB010
	chunkTrackPivot     = 0xB013
	chunkPosTrack       = 0xB020
	chunkRotTrack       = 0xB021
	chunkScaleTrack     = 0xB022
	chunkKeyframeHdr    = 0xB008

	trackHdrSize    = 10
	trackMinBody    = 12
	keyHdrSize      = 6
	posKeySize      = 12
	scaleKeySize    = 12
	rotKeySize      = 16
	defaultFPS      = 30.0
	splineBits      = 5
	halfAngleMul    = 0.5
	defaultAnimName = "default"
	nodeHdrMinSize  = 6
	hierarchyOffset = 4
)

type nodeEntry struct {
	name      string
	hierarchy int
}

func parseKeyframer(data []byte, ctx *parseCtx) {
	var entries []nodeEntry
	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			break
		}
		body := data[chunkHdrSize:size]

		switch id {
		case chunkMeshTrackNode:
			e := parseTrackNode(body, ctx)
			entries = append(entries, e)
		case chunkCamTrackNode, chunkLightTrackNode, chunkSpotTrackNode:
			e := parseTrackNodeHierarchy(body)
			entries = append(entries, e)
		}

		data = data[size:]
	}
	if len(entries) > 0 {
		wireHierarchy(entries, ctx)
	}
}

func parseTrackNode(data []byte, ctx *parseCtx) nodeEntry {
	var entry nodeEntry
	var posTimes []float32
	var posValues [][3]float32
	var rotTimes []float32
	var rotValues [][4]float32
	var scaleTimes []float32
	var scaleValues [][3]float32

	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			break
		}
		body := data[chunkHdrSize:size]

		switch id {
		case chunkTrackObjName:
			entry.name = binread.CString(body)
			nameLen := binread.CStringLen(body)
			if len(body) >= nameLen+nodeHdrMinSize {
				entry.hierarchy = int(int16(binread.ReadU16LE(body[nameLen+hierarchyOffset:]))) + 1 //nolint:gosec // spec value
			}
		case chunkPosTrack:
			posTimes, posValues = parsePosTrack(body)
		case chunkRotTrack:
			rotTimes, rotValues = parseRotTrack(body)
		case chunkScaleTrack:
			scaleTimes, scaleValues = parseScaleTrack(body)
		}

		data = data[size:]
	}

	nodeIdx := ir.NoIndex
	for i := range ctx.asset.Nodes {
		if ctx.asset.Nodes[i].Name == entry.name {
			nodeIdx = i
			break
		}
	}
	if nodeIdx == ir.NoIndex {
		return entry
	}

	anim := ensureAnimation(ctx)

	if len(posTimes) > 0 {
		anim.Channels = append(anim.Channels, ir.AnimationChannel{
			NodeIndex:     nodeIdx,
			Target:        ir.TargetTranslation,
			Interpolation: ir.InterpolationLinear,
			Times:         posTimes,
			Translations:  posValues,
		})
	}
	if len(rotTimes) > 0 {
		anim.Channels = append(anim.Channels, ir.AnimationChannel{
			NodeIndex:     nodeIdx,
			Target:        ir.TargetRotation,
			Interpolation: ir.InterpolationLinear,
			Times:         rotTimes,
			Rotations:     rotValues,
		})
	}
	if len(scaleTimes) > 0 {
		anim.Channels = append(anim.Channels, ir.AnimationChannel{
			NodeIndex:     nodeIdx,
			Target:        ir.TargetScale,
			Interpolation: ir.InterpolationLinear,
			Times:         scaleTimes,
			Scales:        scaleValues,
		})
	}
	return entry
}

func parseTrackNodeHierarchy(data []byte) nodeEntry {
	var entry nodeEntry
	for len(data) >= chunkHdrSize {
		id := binread.ReadU16LE(data[:2])
		size := binread.ClampChunkSize(len(data), binread.ReadU32LE(data[2:6]))
		if size < chunkHdrSize {
			break
		}
		body := data[chunkHdrSize:size]
		if id == chunkTrackObjName {
			entry.name = binread.CString(body)
			nameLen := binread.CStringLen(body)
			if len(body) >= nameLen+nodeHdrMinSize {
				entry.hierarchy = int(int16(binread.ReadU16LE(body[nameLen+hierarchyOffset:]))) + 1 //nolint:gosec // spec value
			}
			break
		}
		data = data[size:]
	}
	return entry
}

type stackEntry struct {
	nodeIdx   int
	hierarchy int
}

func wireHierarchy(entries []nodeEntry, ctx *parseCtx) {
	nodeMap := make(map[string]int, len(ctx.asset.Nodes))
	for i := range ctx.asset.Nodes {
		nodeMap[ctx.asset.Nodes[i].Name] = i
	}

	var stack []stackEntry
	children := make(map[int][]int)

	for _, e := range entries {
		idx, ok := nodeMap[e.name]
		if !ok {
			continue
		}

		for len(stack) > 0 && stack[len(stack)-1].hierarchy >= e.hierarchy {
			stack = stack[:len(stack)-1]
		}

		if len(stack) > 0 {
			parentIdx := stack[len(stack)-1].nodeIdx
			children[parentIdx] = append(children[parentIdx], idx)
		}

		stack = append(stack, stackEntry{nodeIdx: idx, hierarchy: e.hierarchy})
	}

	if len(children) == 0 {
		return
	}

	childSet := make(map[int]bool)
	for _, kids := range children {
		for _, c := range kids {
			childSet[c] = true
		}
	}

	for parentIdx, kids := range children {
		ctx.asset.Nodes[parentIdx].Children = kids
	}

	roots := ctx.asset.RootNodes[:0]
	for _, r := range ctx.asset.RootNodes {
		if !childSet[r] {
			roots = append(roots, r)
		}
	}
	ctx.asset.RootNodes = roots
}

func parsePosTrack(data []byte) (times []float32, values [][3]float32) {
	return parseVec3Track(data, posKeySize)
}

func parseScaleTrack(data []byte) (times []float32, values [][3]float32) {
	return parseVec3Track(data, scaleKeySize)
}

func parseVec3Track(data []byte, keySize int) (times []float32, values [][3]float32) {
	if len(data) < trackMinBody {
		return nil, nil
	}
	data = data[trackHdrSize:]

	nKeys := int(binread.ReadU16LE(data[:2]))
	data = data[2:]

	times = make([]float32, 0, nKeys)
	values = make([][3]float32, 0, nKeys)

	for range nKeys {
		if len(data) < keyHdrSize {
			break
		}
		frame := binread.ReadU32LE(data[:4])
		flags := binread.ReadU16LE(data[4:6])
		data = data[keyHdrSize:]
		splineSkip := splineDataSize(flags)
		if splineSkip > len(data) {
			break
		}
		data = data[splineSkip:]

		if len(data) < keySize {
			break
		}
		x := binread.ReadF32LE(data[0:])
		y := binread.ReadF32LE(data[4:])
		z := binread.ReadF32LE(data[8:])

		times = append(times, float32(frame)/defaultFPS)
		values = append(values, [3]float32{x, y, z})
		data = data[keySize:]
	}
	return times, values
}

func parseRotTrack(data []byte) (times []float32, values [][4]float32) {
	if len(data) < trackMinBody {
		return nil, nil
	}
	data = data[trackHdrSize:]

	nKeys := int(binread.ReadU16LE(data[:2]))
	data = data[2:]

	times = make([]float32, 0, nKeys)
	values = make([][4]float32, 0, nKeys)

	var accQ [4]float32
	accQ[3] = 1

	for range nKeys {
		if len(data) < keyHdrSize {
			break
		}
		frame := binread.ReadU32LE(data[:4])
		flags := binread.ReadU16LE(data[4:6])
		data = data[keyHdrSize:]
		splineSkip := splineDataSize(flags)
		if splineSkip > len(data) {
			break
		}
		data = data[splineSkip:]

		if len(data) < rotKeySize {
			break
		}
		angle := binread.ReadF32LE(data[0:])
		ax := binread.ReadF32LE(data[4:])
		ay := binread.ReadF32LE(data[8:])
		az := binread.ReadF32LE(data[12:])

		halfAngle := float64(angle) * halfAngleMul
		s := float32(math.Sin(halfAngle))
		c := float32(math.Cos(halfAngle))
		dq := [4]float32{ax * s, ay * s, az * s, c}
		accQ = mathx.QuatMulArr(accQ, dq)

		times = append(times, float32(frame)/defaultFPS)
		values = append(values, accQ)
		data = data[rotKeySize:]
	}
	return times, values
}

func splineDataSize(flags uint16) int {
	n := 0
	for i := range splineBits {
		if flags&(1<<i) != 0 {
			n += u32Size
		}
	}
	return n
}

func ensureAnimation(ctx *parseCtx) *ir.Animation {
	if len(ctx.asset.Animations) > 0 {
		return ctx.asset.Animations[0]
	}
	anim := &ir.Animation{Name: defaultAnimName}
	ctx.asset.Animations = append(ctx.asset.Animations, anim)
	return anim
}
