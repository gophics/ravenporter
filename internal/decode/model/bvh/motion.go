package bvh

import (
	"bytes"
	"errors"
	"strconv"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

var (
	errNoMotion   = errors.New("missing MOTION section")
	errNoFrames   = errors.New("missing Frames line")
	errFrameBound = errors.New("frame count exceeds file size bounds")

	chanXposB = []byte(channelXpos)
	chanYposB = []byte(channelYpos)
	chanZposB = []byte(channelZpos)
	chanXrotB = []byte(channelXrot)
	chanYrotB = []byte(channelYrot)
	chanZrotB = []byte(channelZrot)
)

func parseMotion(s *decutil.LineScanner, joints []joint) (*ir.Animation, error) {
	line := s.Next()
	if !bytes.HasPrefix(line, kwFramesB) {
		return nil, errNoMotion
	}

	frameCount, frameTime, err := parseMotionHeader(s, line)
	if err != nil {
		return nil, err
	}
	if frameCount < 0 || frameCount > len(s.Data) {
		return nil, decutil.DecodeErr(formatName, "frame bounds", errFrameBound)
	}

	channelMap := buildChannelMap(joints)
	channels := make([]ir.AnimationChannel, len(channelMap))
	for i, cm := range channelMap {
		channels[i] = ir.AnimationChannel{
			NodeIndex:     cm.jointIdx,
			Target:        cm.target,
			Interpolation: ir.InterpolationLinear,
			Times:         make([]float32, frameCount),
		}
		switch cm.target {
		case ir.TargetTranslation:
			channels[i].Translations = make([][3]float32, frameCount)
		case ir.TargetRotation:
			channels[i].Rotations = make([][4]float32, frameCount)
		}
	}

	totalChannels := 0
	for _, j := range joints {
		totalChannels += len(j.channels)
	}

	fieldsBuf := make([][]byte, 0, totalChannels)
	for f := range frameCount {
		line = s.Next()
		fieldsBuf = decutil.SplitByteFields(line, fieldsBuf)
		t := float32(f) * float32(frameTime)

		globalIdx := 0
		for ci := range channelMap {
			channels[ci].Times[f] = t
			cm := &channelMap[ci]

			switch cm.target {
			case ir.TargetTranslation:
				channels[ci].Translations[f] = readVec3(fieldsBuf, globalIdx)
				globalIdx += axisCount
			case ir.TargetRotation:
				channels[ci].Rotations[f] = eulerToQuat(fieldsBuf, globalIdx, cm.rotOrder)
				globalIdx += axisCount
			}
		}
	}

	duration := float64(frameCount) * frameTime

	return &ir.Animation{
		Name:     formatName,
		Channels: channels,
		Duration: duration,
	}, nil
}

func parseMotionHeader(s *decutil.LineScanner, firstLine []byte) (frameCount int, frameTime float64, err error) {
	frameLine := firstLine
	if !bytes.HasPrefix(frameLine, kwFramesB) {
		frameLine = s.Next()
	}
	if !bytes.HasPrefix(frameLine, kwFramesB) {
		return 0, 0, errNoFrames
	}

	countStr := string(bytes.TrimSpace(bytes.TrimPrefix(frameLine, kwFramesB)))
	frameCount, err = strconv.Atoi(countStr)
	if err != nil {
		return 0, 0, errNoFrames
	}

	ftLine := s.Next()
	parts := decutil.SplitByteFields(ftLine, make([][]byte, 0, frameTimeFields))
	frameTime = defaultFrameTime
	if len(parts) >= frameTimeFields {
		frameTime, _ = strconv.ParseFloat(string(parts[2]), 64) //nolint:errcheck
	}

	return frameCount, frameTime, nil
}

type channelEntry struct {
	jointIdx int
	target   ir.ChannelTarget
	rotOrder [axisCount]int
}

func buildChannelMap(joints []joint) []channelEntry {
	entries := make([]channelEntry, 0, len(joints)*2) //nolint:mnd // translation + rotation per joint

	for i, j := range joints {
		if len(j.channels) == 0 {
			continue
		}

		hasPos := false
		hasRot := false
		var rotOrder [axisCount]int

		for _, ch := range j.channels {
			switch {
			case bytes.Equal(ch, chanXposB) || bytes.Equal(ch, chanYposB) || bytes.Equal(ch, chanZposB):
				hasPos = true
			case bytes.Equal(ch, chanXrotB) || bytes.Equal(ch, chanYrotB) || bytes.Equal(ch, chanZrotB):
				hasRot = true
			}
		}

		if hasRot {
			rotOrder = parseRotOrder(j.channels)
		}

		if hasPos {
			entries = append(entries, channelEntry{jointIdx: i, target: ir.TargetTranslation})
		}
		if hasRot {
			entries = append(entries, channelEntry{jointIdx: i, target: ir.TargetRotation, rotOrder: rotOrder})
		}
	}

	return entries
}

func parseRotOrder(channels [][]byte) [axisCount]int {
	order := [axisCount]int{0, 1, 2}
	idx := 0
	for _, ch := range channels {
		switch {
		case bytes.Equal(ch, chanXrotB):
			order[idx] = 0
			idx++
		case bytes.Equal(ch, chanYrotB):
			order[idx] = 1
			idx++
		case bytes.Equal(ch, chanZrotB):
			order[idx] = 2
			idx++
		}
		if idx >= axisCount {
			break
		}
	}
	return order
}
