package fbx

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

func parseASCIICamera(s *decutil.LineScanner, depth *int, name string) *ir.Camera {
	cam := &ir.Camera{
		Name: name,
		Perspective: &ir.PerspectiveCamera{
			FOV:  float32(defaultCamFOV * degToRad),
			Near: defaultNear,
			Far:  defaultFar,
		},
	}
	for line := s.Next(); line != nil; line = s.Next() {
		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)
		*depth += openCount - closeCount

		if bytes.HasPrefix(line, bPropLine) {
			if v, ok := asciiPropVal(line); ok {
				switch {
				case bytes.Contains(line, bPropFOV) || bytes.Contains(line, bPropFOVX):
					cam.Perspective.FOV = float32(v * degToRad)
				case bytes.Contains(line, bPropNear):
					cam.Perspective.Near = float32(v)
				case bytes.Contains(line, bPropFar):
					cam.Perspective.Far = float32(v)
				}
			}
		}

		if closeCount > 0 && *depth <= 1 {
			break
		}
	}
	return cam
}

func parseASCIILight(s *decutil.LineScanner, depth *int, name string) *ir.Light {
	light := &ir.Light{
		Name:      name,
		Color:     defaultLightColor,
		Intensity: defaultIntensity,
		Point:     &ir.PointLight{},
	}
	var lightType int
	var innerAngle, outerAngle float32
	for line := s.Next(); line != nil; line = s.Next() {
		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)
		*depth += openCount - closeCount

		if bytes.HasPrefix(line, bPropLine) {
			if v, ok := asciiPropVal(line); ok {
				switch {
				case bytes.Contains(line, bPropLightColor):
					light.Color = asciiPropColor(line)
				case bytes.Contains(line, bPropIntensity):
					light.Intensity = float32(v) / float32(fbxIntensityScale)
				case bytes.Contains(line, bPropLightType):
					lightType = int(v)
				case bytes.Contains(line, bPropInnerAngle):
					innerAngle = float32(v * degToRad)
				case bytes.Contains(line, bPropOuterAngle):
					outerAngle = float32(v * degToRad)
				}
			}
		}

		if closeCount > 0 && *depth <= 1 {
			break
		}
	}
	switch lightType {
	case 0:
		light.Point = &ir.PointLight{}
	case 1:
		light.Point = nil
		light.Directional = &ir.DirectionalLight{}
	case 2: //nolint:mnd // FBX LightType: Spot
		light.Point = nil
		light.Spot = &ir.SpotLight{
			InnerConeAngle: innerAngle,
			OuterConeAngle: outerAngle,
		}
	}
	return light
}

func asciiPropVal(line []byte) (float64, bool) {
	idx := bytes.LastIndexByte(line, ',')
	if idx < 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(decutil.Bstr(bytes.TrimSpace(line[idx+1:])), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func asciiPropColor(line []byte) [3]float32 {
	end := len(line)
	c3 := bytes.LastIndexByte(line[:end], ',')
	if c3 < 0 {
		return defaultLightColor
	}
	c2 := bytes.LastIndexByte(line[:c3], ',')
	if c2 < 0 {
		return defaultLightColor
	}
	c1 := bytes.LastIndexByte(line[:c2], ',')
	if c1 < 0 {
		return defaultLightColor
	}
	r, _ := strconv.ParseFloat(decutil.Bstr(bytes.TrimSpace(line[c1+1:c2])), 64)  //nolint:errcheck
	g, _ := strconv.ParseFloat(decutil.Bstr(bytes.TrimSpace(line[c2+1:c3])), 64)  //nolint:errcheck
	b, _ := strconv.ParseFloat(decutil.Bstr(bytes.TrimSpace(line[c3+1:end])), 64) //nolint:errcheck
	return [3]float32{float32(r), float32(g), float32(b)}
}

func parseASCIITexture(s *decutil.LineScanner, depth *int, id int64, name string) asciiTexture {
	tex := asciiTexture{id: id, name: name}

	for line := s.Next(); line != nil; line = s.Next() {
		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)

		if bytes.HasPrefix(line, bRelFilename) {
			tex.path = extractQuotedValueB(line)
		} else if bytes.HasPrefix(line, bFileName) && tex.path == "" {
			tex.path = extractQuotedValueB(line)
		}

		*depth += openCount - closeCount

		if closeCount > 0 && *depth <= 1 {
			return tex
		}
	}
	return tex
}

func extractQuotedValueB(line []byte) string {
	_, after, ok := bytes.Cut(line, bQuote)
	if !ok {
		return ""
	}
	val, _, ok := bytes.Cut(after, bQuote)
	if !ok {
		return ""
	}
	return string(val)
}

func parseASCIIGeometry(s *decutil.LineScanner, depth *int) asciiMeshData {
	var geo asciiMeshData

	for line := s.Next(); line != nil; line = s.Next() {
		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)

		if bytes.HasPrefix(line, bPropVertex) && openCount > 0 {
			geo.positions = parseASCIIFloatTriples(s, extractASCIICapacity(line))
			*depth += openCount - closeCount - 1
			continue
		}
		if bytes.HasPrefix(line, bPropIndex) && openCount > 0 {
			geo.polyIndices = parseASCIIIntArray(s, extractASCIICapacity(line))
			geo.indices = make([]uint32, len(geo.polyIndices))
			for i, v := range geo.polyIndices {
				if v < 0 {
					v = ^v
				}
				geo.indices[i] = uint32(v) //nolint:gosec // bounded
			}
			*depth += openCount - closeCount - 1
			continue
		}
		if bytes.HasPrefix(line, bPropNormals) && openCount > 0 {
			geo.normals = parseASCIIFloatTriples(s, extractASCIICapacity(line))
			*depth += openCount - closeCount - 1
			continue
		}
		if bytes.HasPrefix(line, bPropUV) && !bytes.HasPrefix(line, bUVIndex) && openCount > 0 {
			geo.uvs = parseASCIIFloatPairs(s, extractASCIICapacity(line))
			*depth += openCount - closeCount - 1
			continue
		}

		*depth += openCount - closeCount

		if closeCount > 0 && *depth <= 1 {
			return geo
		}
	}
	return geo
}

func parseASCIIDeformer(s *decutil.LineScanner, depth *int, id int64) asciiCluster {
	cl := asciiCluster{id: id}

	for line := s.Next(); line != nil; line = s.Next() {
		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)

		if bytes.HasPrefix(line, bClusterIndexes) && openCount > 0 {
			cl.idxs = parseASCIIIntArray(s, extractASCIICapacity(line))
			*depth += openCount - closeCount - 1
			continue
		}
		if bytes.HasPrefix(line, bClusterWeights) && openCount > 0 {
			cl.weights = parseASCIIFloatArray(s, extractASCIICapacity(line))
			*depth += openCount - closeCount - 1
			continue
		}
		if (bytes.HasPrefix(line, bClusterTransformLink) || bytes.HasPrefix(line, bTransform)) && openCount > 0 {
			raw := parseASCIIFloatArray(s, extractASCIICapacity(line))
			for i := range min(len(raw), 16) { //nolint:mnd // 4x4 matrix
				cl.ibm[i] = float32(raw[i])
			}
			*depth += openCount - closeCount - 1
			continue
		}

		*depth += openCount - closeCount

		if closeCount > 0 && *depth <= 1 {
			return cl
		}
	}
	return cl
}

func parseASCIIAnimCurve(s *decutil.LineScanner, depth *int, id int64) asciiAnimCurve {
	ac := asciiAnimCurve{id: id}

	for line := s.Next(); line != nil; line = s.Next() {
		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)

		if bytes.HasPrefix(line, bKeyTime) && openCount > 0 {
			raw := parseASCIIFloatArray(s, extractASCIICapacity(line))
			ac.keyTimes = raw
			*depth += openCount - closeCount - 1
			continue
		}
		if bytes.HasPrefix(line, bKeyValue) && openCount > 0 {
			ac.keyVals = parseASCIIFloatArray(s, extractASCIICapacity(line))
			*depth += openCount - closeCount - 1
			continue
		}

		*depth += openCount - closeCount

		if closeCount > 0 && *depth <= 1 {
			return ac
		}
	}
	return ac
}

func parseASCIIShape(s *decutil.LineScanner, depth *int, id int64, name string) asciiShape {
	sh := asciiShape{id: id, name: name}

	for line := s.Next(); line != nil; line = s.Next() {
		openCount := bytes.Count(line, bOpen)
		closeCount := bytes.Count(line, bClose)

		if bytes.HasPrefix(line, bVertices) && openCount > 0 {
			raw := parseASCIIFloatArray(s, extractASCIICapacity(line))
			sh.deltas = floatsToVec3(raw)
			*depth += openCount - closeCount - 1
			continue
		}

		*depth += openCount - closeCount

		if closeCount > 0 && *depth <= 1 {
			return sh
		}
	}
	return sh
}

func floatsToVec3(raw []float64) [][3]float32 {
	const stride = 3
	n := len(raw) / stride
	out := make([][3]float32, n)
	for i := range n {
		out[i] = [3]float32{float32(raw[i*stride]), float32(raw[i*stride+1]), float32(raw[i*stride+2])}
	}
	return out
}

func resolveASCIIMorphTargets(asset *ir.Asset, res asciiParseResult) {
	if len(res.shapes) == 0 || len(asset.Meshes) == 0 {
		return
	}

	for _, shape := range res.shapes {
		mt := ir.MorphTarget{
			Name:      shape.name,
			Positions: shape.deltas,
		}
		for _, mesh := range asset.Meshes {
			for pi := range mesh.Primitives {
				if mesh.Primitives[pi].Data.VertexCount == len(shape.deltas) {
					mesh.Primitives[pi].MorphTargets = append(mesh.Primitives[pi].MorphTargets, mt)
					break
				}
			}
		}
	}
}

func resolveASCIIAnimations(asset *ir.Asset, res asciiParseResult, conns []asciiConnection, models []asciiModelInfo) {
	if len(res.curves) == 0 || len(res.curveNodeIDs) == 0 {
		return
	}

	cnIDMap := make(map[int64]int, len(res.curveNodeIDs))
	for i, id := range res.curveNodeIDs {
		cnIDMap[id] = i
	}

	modelMap := make(map[int64]int, len(models))
	for i, m := range models {
		modelMap[m.id] = i
	}

	curveMap := make(map[int64]*asciiAnimCurve, len(res.curves))
	for i := range res.curves {
		curveMap[res.curves[i].id] = &res.curves[i]
	}

	cnToModel := make(map[int64]int, len(res.curveNodeIDs))
	curveToCN := make(map[int64]int64, len(res.curves))
	for _, c := range conns {
		if _, ok := cnIDMap[c.child]; ok {
			if nodeIdx, ok := modelMap[c.parent]; ok {
				cnToModel[c.child] = nodeIdx
			}
		}
		if _, ok := curveMap[c.child]; ok {
			curveToCN[c.child] = c.parent
		}
	}

	channels := make(map[int64]*ir.AnimationChannel)
	for i, cnID := range res.curveNodeIDs {
		nodeIdx, ok := cnToModel[cnID]
		if !ok {
			continue
		}

		target := resolveASCIIAnimTarget(res.curveNodeTargets[i])
		channels[cnID] = &ir.AnimationChannel{
			NodeIndex:     nodeIdx,
			Target:        target,
			Interpolation: ir.InterpolationLinear,
		}
	}

	for curveID, curve := range curveMap {
		cnID, ok := curveToCN[curveID]
		if !ok {
			continue
		}
		ch, ok := channels[cnID]
		if !ok || len(curve.keyTimes) == 0 {
			continue
		}
		ch.Times = make([]float32, len(curve.keyTimes))
		for i, kt := range curve.keyTimes {
			ch.Times[i] = float32(kt / fbxKTimeScale)
		}
		ch.Translations = make([][3]float32, len(curve.keyVals))
		for i, v := range curve.keyVals {
			ch.Translations[i] = [3]float32{float32(v), 0, 0}
		}
	}

	var anim *ir.Animation
	if len(asset.Animations) > 0 {
		anim = asset.Animations[0]
	} else {
		anim = &ir.Animation{Name: defaultAnimName}
		asset.Animations = append(asset.Animations, anim)
	}
	for _, ch := range channels {
		if len(ch.Times) > 0 {
			anim.Channels = append(anim.Channels, *ch)
		}
	}
}

func resolveASCIIAnimTarget(name string) ir.ChannelTarget {
	switch {
	case strings.HasPrefix(name, animTargetT) || strings.Contains(name, animLongT):
		return ir.TargetTranslation
	case strings.HasPrefix(name, animTargetR) || strings.Contains(name, animLongR):
		return ir.TargetRotation
	case strings.HasPrefix(name, animTargetS) || strings.Contains(name, animLongS):
		return ir.TargetScale
	default:
		return ir.TargetTranslation
	}
}
