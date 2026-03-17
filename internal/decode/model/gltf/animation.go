package gltf

import (
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/valyala/fastjson"
)

const (
	pathTranslation = "translation"
	pathRotation    = "rotation"
	pathScale       = "scale"
	pathWeights     = "weights"

	interpLinear      = "LINEAR"
	interpStep        = "STEP"
	interpCubicSpline = "CUBICSPLINE"

	ptrPrefixMaterials = "/materials/"
	ptrPrefixNodes     = "/nodes/"
	ptrPrefixCameras   = "/cameras/"
	ptrPrefixLights    = "/extensions/KHR_lights_punctual/lights/"

	ptrSuffixBaseColorFactor  = "pbrMetallicRoughness/baseColorFactor"
	ptrSuffixMetallicFactor   = "pbrMetallicRoughness/metallicFactor"
	ptrSuffixRoughnessFactor  = "pbrMetallicRoughness/roughnessFactor"
	ptrSuffixEmissiveFactor   = "emissiveFactor"
	ptrSuffixAlphaCutoff      = "alphaCutoff"
	ptrSuffixEmissiveStrength = "extensions/KHR_materials_emissive_strength/emissiveStrength"
	ptrSuffixTransmission     = "extensions/KHR_materials_transmission/transmissionFactor"
	ptrSuffixIOR              = "extensions/KHR_materials_ior/ior"
	ptrSuffixClearcoat        = "extensions/KHR_materials_clearcoat/clearcoatFactor"
	ptrSuffixClearcoatRough   = "extensions/KHR_materials_clearcoat/clearcoatRoughnessFactor"
	ptrSuffixSheenColor       = "extensions/KHR_materials_sheen/sheenColorFactor"
	ptrSuffixSheenRoughness   = "extensions/KHR_materials_sheen/sheenRoughnessFactor"
	ptrSuffixThickness        = "extensions/KHR_materials_volume/thicknessFactor"
	ptrSuffixAttenuation      = "extensions/KHR_materials_volume/attenuationDistance"

	ptrSuffixTranslation = "translation"
	ptrSuffixRotation    = "rotation"
	ptrSuffixScale       = "scale"
	ptrSuffixWeights     = "weights"

	ptrSuffixColor     = "color"
	ptrSuffixIntensity = "intensity"

	ptrSuffixYfov = "perspective/yfov"
	ptrSuffixXmag = "orthographic/xmag"
	ptrSuffixYmag = "orthographic/ymag"
)

func (d *doc) convertAnimations() []*ir.Animation {
	arr := d.root.GetArray(keyAnimations)
	if len(arr) == 0 {
		return nil
	}
	bulk := make([]ir.Animation, len(arr))
	out := make([]*ir.Animation, len(arr))
	for i, a := range arr {
		d.convertAnimation(a, &bulk[i])
		out[i] = &bulk[i]
	}
	return out
}

func (d *doc) convertAnimation(a *fastjson.Value, anim *ir.Animation) {
	samplers := a.GetArray(keySamplers)
	channels := a.GetArray(keyChannels)
	irChannels := make([]ir.AnimationChannel, 0, len(channels))
	var maxTime float64

	for _, ch := range channels {
		samplerIdx := ch.GetInt(keySampler)
		if samplerIdx < 0 || samplerIdx >= len(samplers) {
			continue
		}
		irCh, dur := d.convertAnimChannel(ch, samplers[samplerIdx])
		irChannels = append(irChannels, irCh)
		if dur > maxTime {
			maxTime = dur
		}
	}

	anim.Name = string(a.GetStringBytes(keyName))
	anim.Channels = irChannels
	anim.Duration = maxTime
}

//nolint:gocritic // unnamedResult: returns are assigned in body
func (d *doc) convertAnimChannel(ch, sampler *fastjson.Value) (ir.AnimationChannel, float64) {
	inputAcr := d.getAccessor(sampler.GetInt(keyInput))
	outputAcr := d.getAccessor(sampler.GetInt(keyOutput))
	times := d.bufs.readFloat32s(inputAcr)

	target := ch.Get(keyTarget)
	path := string(target.GetStringBytes(keyPath))

	irCh := ir.AnimationChannel{
		NodeIndex:     target.GetInt(keyNode),
		Interpolation: interpolation(decutil.Bstr(sampler.GetStringBytes(keyInterpolation))),
		Times:         times,
	}

	if path == pathPointer {
		d.resolvePointerChannel(ch, outputAcr, &irCh)
	} else {
		irCh.Target = channelTarget(path)
		switch path {
		case pathTranslation:
			irCh.Translations = d.bufs.readVec3s(outputAcr)
		case pathRotation:
			irCh.Rotations = d.bufs.readVec4s(outputAcr)
		case pathScale:
			irCh.Scales = d.bufs.readVec3s(outputAcr)
		case pathWeights:
			irCh.Weights = d.bufs.readFloat32s(outputAcr)
		}
	}

	var dur float64
	if len(times) > 0 {
		dur = float64(times[len(times)-1])
	}
	return irCh, dur
}

func (d *doc) resolvePointerChannel(ch *fastjson.Value, outputAcr accessor, irCh *ir.AnimationChannel) {
	ext := ch.Get(keyTarget, keyExtensions, keyKHRAnimationPointer)
	if ext == nil {
		return
	}
	ptr := string(ext.GetStringBytes(keyPointer))
	if ptr == "" {
		return
	}
	irCh.Pointer = ptr

	switch {
	case strings.HasPrefix(ptr, ptrPrefixMaterials):
		d.resolveMaterialPointer(ptr[len(ptrPrefixMaterials):], outputAcr, irCh)
	case strings.HasPrefix(ptr, ptrPrefixNodes):
		d.resolveNodePointer(ptr[len(ptrPrefixNodes):], outputAcr, irCh)
	case strings.HasPrefix(ptr, ptrPrefixCameras):
		d.resolveCameraPointer(ptr[len(ptrPrefixCameras):], outputAcr, irCh)
	case strings.HasPrefix(ptr, ptrPrefixLights):
		d.resolveLightPointer(ptr[len(ptrPrefixLights):], outputAcr, irCh)
	default:
		irCh.Target = ir.TargetPointer
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	}
}

func (d *doc) resolveMaterialPointer(rest string, outputAcr accessor, irCh *ir.AnimationChannel) {
	idx, suffix := splitIndexSuffix(rest)
	if idx < 0 {
		return
	}
	irCh.MaterialIndex = idx

	switch suffix {
	case ptrSuffixBaseColorFactor, ptrSuffixEmissiveFactor, ptrSuffixSheenColor:
		irCh.Target = ir.TargetMaterialColor
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	case ptrSuffixMetallicFactor, ptrSuffixRoughnessFactor, ptrSuffixAlphaCutoff,
		ptrSuffixEmissiveStrength, ptrSuffixTransmission, ptrSuffixIOR,
		ptrSuffixClearcoat, ptrSuffixClearcoatRough, ptrSuffixSheenRoughness,
		ptrSuffixThickness, ptrSuffixAttenuation:
		irCh.Target = ir.TargetMaterialScalar
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	default:
		irCh.Target = ir.TargetPointer
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	}
}

func (d *doc) resolveNodePointer(rest string, outputAcr accessor, irCh *ir.AnimationChannel) {
	idx, suffix := splitIndexSuffix(rest)
	if idx < 0 {
		return
	}
	irCh.NodeIndex = idx

	switch suffix {
	case ptrSuffixTranslation:
		irCh.Target = ir.TargetTranslation
		irCh.Translations = d.bufs.readVec3s(outputAcr)
	case ptrSuffixRotation:
		irCh.Target = ir.TargetRotation
		irCh.Rotations = d.bufs.readVec4s(outputAcr)
	case ptrSuffixScale:
		irCh.Target = ir.TargetScale
		irCh.Scales = d.bufs.readVec3s(outputAcr)
	case ptrSuffixWeights:
		irCh.Target = ir.TargetMorphWeights
		irCh.Weights = d.bufs.readFloat32s(outputAcr)
	default:
		irCh.Target = ir.TargetPointer
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	}
}

func (d *doc) resolveLightPointer(rest string, outputAcr accessor, irCh *ir.AnimationChannel) {
	_, suffix := splitIndexSuffix(rest)

	switch suffix {
	case ptrSuffixColor:
		irCh.Target = ir.TargetLightColor
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	case ptrSuffixIntensity:
		irCh.Target = ir.TargetLightIntensity
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	default:
		irCh.Target = ir.TargetPointer
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	}
}

func (d *doc) resolveCameraPointer(rest string, outputAcr accessor, irCh *ir.AnimationChannel) {
	_, suffix := splitIndexSuffix(rest)

	switch suffix {
	case ptrSuffixYfov, ptrSuffixXmag, ptrSuffixYmag:
		irCh.Target = ir.TargetCameraFOV
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	default:
		irCh.Target = ir.TargetPointer
		irCh.Values = d.bufs.readFloat32s(outputAcr)
	}
}

func splitIndexSuffix(s string) (idx int, suffix string) {
	slashPos := strings.IndexByte(s, '/')
	var err error
	if slashPos < 0 {
		idx, err = strconv.Atoi(s)
		if err != nil {
			return -1, ""
		}
		return idx, ""
	}
	idx, err = strconv.Atoi(s[:slashPos])
	if err != nil {
		return -1, ""
	}
	return idx, s[slashPos+1:]
}

func channelTarget(path string) ir.ChannelTarget {
	switch path {
	case pathTranslation:
		return ir.TargetTranslation
	case pathRotation:
		return ir.TargetRotation
	case pathScale:
		return ir.TargetScale
	case pathWeights:
		return ir.TargetMorphWeights
	default:
		return ir.TargetTranslation
	}
}

func interpolation(s string) ir.Interpolation {
	switch s {
	case interpLinear:
		return ir.InterpolationLinear
	case interpStep:
		return ir.InterpolationStep
	case interpCubicSpline:
		return ir.InterpolationCubicSpline
	default:
		return ir.InterpolationLinear
	}
}
