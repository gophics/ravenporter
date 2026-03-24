package usda

import (
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const quatComponents = 4

type skelAnimData struct {
	transTimes  []float32
	transFrames [][][3]float32
	rotTimes    []float32
	rotFrames   [][][4]float32
	scaleTimes  []float32
	scaleFrames [][][3]float32
}

func (p *usdaParser) parseSkelAnimPrim(defLine string) {
	name := extractQuotedName(defLine)
	var joints []string
	var ad skelAnimData

	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch {
		case line == "{":
			depth++
		case line == "}":
			depth--
			if depth == 0 {
				if len(joints) == 0 {
					return
				}
				anim := buildSkelAnim(name, joints, &ad, p.asset)
				if anim != nil {
					p.asset.Animations = append(p.asset.Animations, anim)
				}
				return
			}
		case strings.Contains(line, usdaSkelJoints):
			joints = parseTokenArray(p.collectArray(line))
		case strings.Contains(line, usdaAnimTranslations):
			if strings.Contains(line, usdaTimeSamples) {
				ad.transTimes, ad.transFrames = p.parseTimeSampledVec3()
			} else {
				ad.transFrames = [][][3]float32{parseVec3Array(p.collectArray(line))}
				ad.transTimes = []float32{0}
			}
		case strings.Contains(line, usdaAnimRotations):
			if strings.Contains(line, usdaTimeSamples) {
				ad.rotTimes, ad.rotFrames = p.parseTimeSampledQuat()
			} else {
				ad.rotFrames = [][][4]float32{parseQuatArray(p.collectArray(line))}
				ad.rotTimes = []float32{0}
			}
		case strings.Contains(line, usdaAnimScales):
			if strings.Contains(line, usdaTimeSamples) {
				ad.scaleTimes, ad.scaleFrames = p.parseTimeSampledVec3()
			} else {
				ad.scaleFrames = [][][3]float32{parseVec3Array(p.collectArray(line))}
				ad.scaleTimes = []float32{0}
			}
		}
	}
}

func buildSkelAnim(
	name string,
	joints []string,
	ad *skelAnimData,
	asset *ir.Asset,
) *ir.Animation {
	if len(joints) == 0 {
		return nil
	}

	nodeMap := make(map[string]int, len(asset.Nodes))
	for i := range asset.Nodes {
		nodeMap[asset.Nodes[i].Name] = i
	}

	var channels []ir.AnimationChannel
	for ji, jPath := range joints {
		leaf := jPath
		if idx := strings.LastIndex(jPath, "/"); idx >= 0 {
			leaf = jPath[idx+1:]
		}
		ni, ok := nodeMap[leaf]
		if !ok {
			continue
		}
		if len(ad.transFrames) > 0 {
			ch := ir.AnimationChannel{
				NodeIndex:     ni,
				Target:        ir.TargetTranslation,
				Interpolation: ir.InterpolationLinear,
				Times:         ad.transTimes,
				Translations:  make([][3]float32, len(ad.transFrames)),
			}
			for fi, frame := range ad.transFrames {
				if ji < len(frame) {
					ch.Translations[fi] = frame[ji]
				}
			}
			channels = append(channels, ch)
		}
		if len(ad.rotFrames) > 0 {
			ch := ir.AnimationChannel{
				NodeIndex:     ni,
				Target:        ir.TargetRotation,
				Interpolation: ir.InterpolationLinear,
				Times:         ad.rotTimes,
				Rotations:     make([][4]float32, len(ad.rotFrames)),
			}
			for fi, frame := range ad.rotFrames {
				if ji < len(frame) {
					ch.Rotations[fi] = frame[ji]
				}
			}
			channels = append(channels, ch)
		}
		if len(ad.scaleFrames) > 0 {
			ch := ir.AnimationChannel{
				NodeIndex:     ni,
				Target:        ir.TargetScale,
				Interpolation: ir.InterpolationLinear,
				Times:         ad.scaleTimes,
				Scales:        make([][3]float32, len(ad.scaleFrames)),
			}
			for fi, frame := range ad.scaleFrames {
				if ji < len(frame) {
					ch.Scales[fi] = frame[ji]
				}
			}
			channels = append(channels, ch)
		}
	}
	if len(channels) == 0 {
		return nil
	}
	return &ir.Animation{Name: name, Channels: channels, Duration: maxAnimTime(ad)}
}

func maxAnimTime(ad *skelAnimData) float64 {
	var m float64
	for _, t := range ad.transTimes {
		if float64(t) > m {
			m = float64(t)
		}
	}
	for _, t := range ad.rotTimes {
		if float64(t) > m {
			m = float64(t)
		}
	}
	for _, t := range ad.scaleTimes {
		if float64(t) > m {
			m = float64(t)
		}
	}
	return m
}

func (p *usdaParser) parseTimeSampledVec3() (times []float32, frames [][][3]float32) {
	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch line {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				return times, frames
			}
		default:
			t, vals := parseTimeSampleLine(line)
			if vals == "" {
				continue
			}
			times = append(times, t)
			frames = append(frames, parseVec3Array(vals))
		}
	}
	return times, frames
}

func (p *usdaParser) parseTimeSampledQuat() (times []float32, frames [][][4]float32) {
	depth := 1
	for b := p.ls.Next(); b != nil; b = p.ls.Next() {
		line := decutil.Bstr(b)
		switch line {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				return times, frames
			}
		default:
			t, vals := parseTimeSampleLine(line)
			if vals == "" {
				continue
			}
			times = append(times, t)
			frames = append(frames, parseQuatArray(vals))
		}
	}
	return times, frames
}

func parseTimeSampleLine(line string) (time float32, values string) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return 0, ""
	}
	t := parseOneFloat(strings.TrimSpace(line[:colon]))
	rest := strings.TrimSpace(line[colon+1:])
	bracketStart := strings.IndexByte(rest, '[')
	if bracketStart < 0 {
		return t, ""
	}
	bracketEnd := strings.LastIndexByte(rest, ']')
	if bracketEnd < bracketStart {
		return t, ""
	}
	return t, rest[bracketStart+1 : bracketEnd]
}

func parseQuatArray(data string) [][4]float32 {
	var result [][4]float32
	for data != "" {
		start := strings.IndexByte(data, '(')
		if start < 0 {
			break
		}
		end := strings.IndexByte(data[start:], ')')
		if end < 0 {
			break
		}
		inner := data[start+1 : start+end]
		data = data[start+end+1:]

		parts := strings.Split(inner, ",")
		if len(parts) < quatComponents {
			continue
		}
		w := parseOneFloat(strings.TrimSpace(parts[0]))
		x := parseOneFloat(strings.TrimSpace(parts[1]))
		y := parseOneFloat(strings.TrimSpace(parts[2]))
		z := parseOneFloat(strings.TrimSpace(parts[3]))
		result = append(result, [4]float32{x, y, z, w})
	}
	return result
}

func parseOneFloat(s string) float32 {
	v, _ := strconv.ParseFloat(s, 32) //nolint:errcheck // best-effort
	return float32(v)
}
