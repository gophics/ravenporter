package dae

import (
	"context"
	"strings"

	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

const (
	semanticInterpolation = "INTERPOLATION"
	defaultAnimName       = "default"

	interpBezier   = "BEZIER"
	interpHermite  = "HERMITE"
	interpCardinal = "CARDINAL"
	interpStep     = "STEP"

	targetTranslat  = "translat"
	targetLocation  = "location"
	targetRotat     = "rotat"
	targetScal      = "scal"
	targetTransform = "transform"
	targetMatrix    = "matrix"

	minTargetParts = 2
)

func convertAnimations(sysCtx context.Context, anims []xmlAnimation, asset *ir.Asset) []*ir.Animation {
	if len(anims) == 0 {
		return nil
	}

	flat := flattenAnimations(anims)
	if len(flat) == 0 {
		return nil
	}

	anim := &ir.Animation{Name: defaultAnimName}
	for i := range flat {
		channels := convertSingleAnimation(sysCtx, &flat[i], asset)
		anim.Channels = append(anim.Channels, channels...)
	}

	if len(anim.Channels) == 0 {
		return nil
	}

	for i := range anim.Channels {
		if n := len(anim.Channels[i].Times); n > 0 {
			if t := float64(anim.Channels[i].Times[n-1]); t > anim.Duration {
				anim.Duration = t
			}
		}
	}

	return []*ir.Animation{anim}
}

func flattenAnimations(anims []xmlAnimation) []xmlAnimation {
	var out []xmlAnimation
	for i := range anims {
		if len(anims[i].Channels) > 0 {
			out = append(out, anims[i])
		}
		out = append(out, flattenAnimations(anims[i].Children)...)
	}
	return out
}

func convertSingleAnimation(sysCtx context.Context, xa *xmlAnimation, asset *ir.Asset) []ir.AnimationChannel {
	if len(xa.Channels) == 0 || len(xa.Samplers) == 0 {
		return nil
	}

	srcMap := buildSourceMap(sysCtx, xa.Sources)
	sampler := xa.Samplers[0]
	channel := xa.Channels[0]

	var inputSrc, outputSrc, interpSrc string
	for _, inp := range sampler.Inputs {
		switch inp.Semantic {
		case semanticInput:
			inputSrc = inp.Source
		case semanticOutput:
			outputSrc = inp.Source
		case semanticInterpolation:
			interpSrc = inp.Source
		}
	}

	times := srcMap[inputSrc]
	values := srcMap[outputSrc]
	if len(times) == 0 || len(values) == 0 {
		return nil
	}

	interp := resolveInterpolation(xa.Sources, interpSrc)
	nodeIdx := resolveAnimNodeIndex(channel.Target, asset)

	if isMatrixAnimation(channel.Target, len(times), len(values)) {
		return convertMatrixAnimation(times, values, nodeIdx)
	}

	target := resolveAnimTarget(channel.Target)
	ch := ir.AnimationChannel{
		NodeIndex:     nodeIdx,
		Target:        target,
		Interpolation: interp,
		Times:         times,
	}

	switch target {
	case ir.TargetTranslation, ir.TargetScale:
		ch.Translations = mathx.FloatsToVec3s(values)
	case ir.TargetRotation:
		ch.Rotations = mathx.FloatsToVec4s(values)
	}

	return []ir.AnimationChannel{ch}
}

func resolveInterpolation(sources []source, ref string) ir.Interpolation {
	if ref == "" {
		return ir.InterpolationLinear
	}
	names := parseNameArray(sources, ref)
	if len(names) == 0 {
		return ir.InterpolationLinear
	}
	switch strings.ToUpper(names[0]) {
	case interpBezier, interpHermite, interpCardinal:
		return ir.InterpolationCubicSpline
	case interpStep:
		return ir.InterpolationStep
	default:
		return ir.InterpolationLinear
	}
}

func resolveAnimTarget(target string) ir.ChannelTarget {
	parts := strings.Split(target, "/")
	if len(parts) < minTargetParts {
		return ir.TargetTranslation
	}
	prop := strings.ToLower(parts[len(parts)-1])
	switch {
	case strings.Contains(prop, targetTranslat) || strings.Contains(prop, targetLocation):
		return ir.TargetTranslation
	case strings.Contains(prop, targetRotat):
		return ir.TargetRotation
	case strings.Contains(prop, targetScal):
		return ir.TargetScale
	default:
		return ir.TargetTranslation
	}
}

func resolveAnimNodeIndex(target string, asset *ir.Asset) int {
	parts := strings.Split(target, "/")
	if len(parts) == 0 {
		return ir.NoIndex
	}
	nodeID := parts[0]
	for i := range asset.Nodes {
		if asset.Nodes[i].Name == nodeID {
			return i
		}
	}
	return ir.NoIndex
}

const matStride = 16 // 4x4 matrix

func isMatrixAnimation(target string, nTimes, nValues int) bool {
	parts := strings.Split(target, "/")
	if len(parts) >= minTargetParts {
		prop := strings.ToLower(parts[len(parts)-1])
		if strings.Contains(prop, targetTransform) || strings.Contains(prop, targetMatrix) {
			return true
		}
	}
	return nTimes > 0 && nValues == nTimes*matStride
}

func convertMatrixAnimation(times, values []float32, nodeIdx int) []ir.AnimationChannel {
	n := len(times)
	if n == 0 || len(values) < n*matStride {
		return nil
	}

	translations := make([][3]float32, n)
	rotations := make([][4]float32, n)
	scales := make([][3]float32, n)

	for i := range n {
		var m [matStride]float32
		copy(m[:], values[i*matStride:(i+1)*matStride])
		translations[i], rotations[i], scales[i] = decomposeMatrix(m)
	}

	return []ir.AnimationChannel{
		{
			NodeIndex: nodeIdx, Target: ir.TargetTranslation,
			Interpolation: ir.InterpolationLinear, Times: times, Translations: translations,
		},
		{
			NodeIndex: nodeIdx, Target: ir.TargetRotation,
			Interpolation: ir.InterpolationLinear, Times: times, Rotations: rotations,
		},
		{
			NodeIndex: nodeIdx, Target: ir.TargetScale,
			Interpolation: ir.InterpolationLinear, Times: times, Translations: scales,
		},
	}
}

func decomposeMatrix(m [16]float32) (t [3]float32, r [4]float32, s [3]float32) {
	t = [3]float32{m[3], m[7], m[11]}

	sx := mathx.VecLen3(m[0], m[4], m[8])
	sy := mathx.VecLen3(m[1], m[5], m[9])
	sz := mathx.VecLen3(m[2], m[6], m[10])
	s = [3]float32{sx, sy, sz}

	if sx > 0 {
		sx = 1.0 / sx
	}
	if sy > 0 {
		sy = 1.0 / sy
	}
	if sz > 0 {
		sz = 1.0 / sz
	}

	r00, r10, r20 := m[0]*sx, m[4]*sx, m[8]*sx
	r01, r11, r21 := m[1]*sy, m[5]*sy, m[9]*sy
	r02, r12, r22 := m[2]*sz, m[6]*sz, m[10]*sz

	r = mathx.MatToQuat(r00, r01, r02, r10, r11, r12, r20, r21, r22)
	return t, r, s
}
