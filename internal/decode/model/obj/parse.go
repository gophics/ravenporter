package obj

import (
	"bytes"
	"context"
)

type vertexRef struct {
	pos, uv, norm int
}

type face struct {
	vertStart int
	vertCount int
}

type group struct {
	name     string
	material string
	smooth   int
	faces    []face
	vertRefs []vertexRef
	lines    []uint32
	points   []uint32
}

type objData struct {
	positions    [][3]float32
	texCoords    [][2]float32
	normals      [][3]float32
	vertexColors [][4]float32
	groups       []group
	mtlLib       string
	paramVerts   [][3]float32
	curves3D     []curve3D
	curves2D     []curve2D
	surfaces     []surface
}

const (
	defaultGroupName = "default"

	minPositionFields  = 4
	minTexCoordFields  = 3
	minFaceFields      = 4
	minFloat3Fields    = 3
	minEstCapacity     = 64
	minLineFields      = 3
	minPointFields     = 2
	minVertColorFields = 7
	homogeneousFields  = 5
	minVpFields        = 2
	minCstypeFields    = 2
	minDegFields       = 2
	minBmatFields      = 2
	minStepFields      = 2
	minCurvFields      = 4
	minCurv2Fields     = 2
	minSurfFields      = 6
	minParmFields      = 3
	minTrimFields      = 4

	bytesPerVertex = 30
)

var (
	cmdO       = []byte("o")
	cmdG       = []byte("g")
	cmdUseMtl  = []byte("usemtl")
	cmdMtlLib  = []byte("mtllib")
	smoothOff  = []byte("off")
	smoothZero = []byte("0")

	cmdVp     = []byte("vp")
	cmdCstype = []byte("cstype")
	cmdDeg    = []byte("deg")
	cmdBmat   = []byte("bmat")
	cmdStep   = []byte("step")
	cmdCurv   = []byte("curv")
	cmdCurv2  = []byte("curv2")
	cmdSurf   = []byte("surf")
	cmdParm   = []byte("parm")
	cmdTrim   = []byte("trim")
	cmdHole   = []byte("hole")
	cmdEnd    = []byte("end")

	typBezier   = []byte("bezier")
	typBspline  = []byte("bspline")
	typCardinal = []byte("cardinal")
	typTaylor   = []byte("taylor")
	typBmatrix  = []byte("bmatrix")
	typRat      = []byte("rat")
)

//nolint:gocyclo,funlen // OBJ state machine requires large switch
func parseOBJ(ctx context.Context, raw []byte) (*objData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	estVerts := len(raw) / bytesPerVertex
	if estVerts < minEstCapacity {
		estVerts = minEstCapacity
	}

	data := &objData{
		positions: make([][3]float32, 0, estVerts),
	}
	cur := group{name: defaultGroupName}
	sc := byteScanner{data: raw}
	var fs fieldSplitter

	var ffState freeformState
	var activeCurv3D *curve3D
	var activeCurv2D *curve2D
	var activeSurf *surface
	inBody := false

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line, ok := sc.nextLogicalLine()
		if !ok {
			break
		}
		fs.split(line)
		if fs.count == 0 {
			continue
		}

		cmd := fs.get(0)
		if cmd[0] == '#' {
			continue
		}

		var err error

		if inBody {
			switch {
			case bytes.Equal(cmd, cmdParm):
				err = parseParmB(activeCurv3D, activeCurv2D, activeSurf, &fs)
			case bytes.Equal(cmd, cmdTrim):
				err = parseTrimHoleB(activeSurf, &fs, false)
			case bytes.Equal(cmd, cmdHole):
				err = parseTrimHoleB(activeSurf, &fs, true)
			case bytes.Equal(cmd, cmdEnd):
				parseEndB(data, activeCurv3D, activeCurv2D, activeSurf)
				activeCurv3D = nil
				activeCurv2D = nil
				activeSurf = nil
				inBody = false
			}
			if err != nil {
				return nil, err
			}
			continue
		}

		switch {
		case len(cmd) == 1 && cmd[0] == 'v':
			err = parsePositionB(data, &fs)
		case len(cmd) == 2 && cmd[0] == 'v' && cmd[1] == 't':
			err = parseTexCoordB(data, &fs)
		case len(cmd) == 2 && cmd[0] == 'v' && cmd[1] == 'n':
			err = parseNormalB(data, &fs)
		case len(cmd) == 1 && cmd[0] == 'f':
			err = parseFaceB(&cur, &fs)
		case bytes.Equal(cmd, cmdO) || bytes.Equal(cmd, cmdG):
			cur = flushGroupB(data, cur, &fs)
		case bytes.Equal(cmd, cmdUseMtl):
			cur = handleUseMTLB(data, cur, &fs)
		case bytes.Equal(cmd, cmdMtlLib):
			parseMtlLibB(data, &fs)
		case len(cmd) == 1 && cmd[0] == 's':
			cur.smooth = parseSmoothGroup(&fs)
		case len(cmd) == 1 && cmd[0] == 'l':
			parseLineB(&cur, &fs, data)
		case len(cmd) == 1 && cmd[0] == 'p':
			parsePointB(&cur, &fs, data)
		case bytes.Equal(cmd, cmdVp):
			err = parseVpB(data, &fs)
		case bytes.Equal(cmd, cmdCstype):
			parseCstypeB(&ffState, &fs)
		case bytes.Equal(cmd, cmdDeg):
			parseDegB(&ffState, &fs)
		case bytes.Equal(cmd, cmdBmat):
			parseBmatB(&ffState, &fs)
		case bytes.Equal(cmd, cmdStep):
			parseStepB(&ffState, &fs)
		case bytes.Equal(cmd, cmdCurv):
			activeCurv3D, err = parseCurvB(ffState, &fs)
			inBody = err == nil && activeCurv3D != nil
		case bytes.Equal(cmd, cmdCurv2):
			activeCurv2D, err = parseCurv2B(ffState, &fs)
			inBody = err == nil && activeCurv2D != nil
		case bytes.Equal(cmd, cmdSurf):
			activeSurf, err = parseSurfB(ffState, &fs)
			inBody = err == nil && activeSurf != nil
		}
		if err != nil {
			return nil, err
		}
	}

	if len(cur.faces) > 0 || len(cur.lines) > 0 || len(cur.points) > 0 {
		data.groups = append(data.groups, cur)
	}
	return data, nil
}

func parseMtlLibB(data *objData, fs *fieldSplitter) {
	if f := fs.get(1); f != nil {
		data.mtlLib = string(f)
	}
}

func flushGroupB(data *objData, cur group, fs *fieldSplitter) group {
	if len(cur.faces) > 0 {
		data.groups = append(data.groups, cur)
	}
	name := defaultGroupName
	if f := fs.get(1); f != nil {
		name = string(f)
	}
	return group{name: name, material: cur.material, smooth: cur.smooth}
}

func parseSmoothGroup(fs *fieldSplitter) int {
	f := fs.get(1)
	if f == nil || bytes.Equal(f, smoothOff) || bytes.Equal(f, smoothZero) {
		return 0
	}
	v, _ := parseIntBytes(f) //nolint:errcheck // best-effort parse
	return v
}

func handleUseMTLB(data *objData, cur group, fs *fieldSplitter) group {
	f := fs.get(1)
	if f == nil {
		return cur
	}
	mat := string(f)
	if len(cur.faces) > 0 && cur.material != mat {
		data.groups = append(data.groups, cur)
		return group{name: cur.name, material: mat}
	}
	cur.material = mat
	return cur
}

func parsePositionB(data *objData, fs *fieldSplitter) error {
	if fs.count < minPositionFields {
		return decodeErr(errBadVertex.Error())
	}
	v, err := parseFloat3Bytes(fs.get(1), fs.get(2), fs.get(3)) //nolint:mnd // xyz coordinates
	if err != nil {
		return decodeErrCause(errBadVertex.Error(), err)
	}
	data.positions = append(data.positions, v)
	if fs.count == homogeneousFields {
		w, werr := parseFloatBytes(fs.get(4)) //nolint:mnd // 4th
		if werr == nil && w != 0 && w != 1 {
			idx := len(data.positions) - 1
			data.positions[idx] = [3]float32{v[0] / w, v[1] / w, v[2] / w}
		}
	}
	if fs.count >= minVertColorFields {
		c, err := parseFloat3Bytes(fs.get(4), fs.get(5), fs.get(6)) //nolint:mnd // color components
		if err == nil {
			for len(data.vertexColors) < len(data.positions)-1 {
				data.vertexColors = append(data.vertexColors, [4]float32{0, 0, 0, 1})
			}
			data.vertexColors = append(data.vertexColors, [4]float32{c[0], c[1], c[2], 1})
		}
	}
	return nil
}

func parseTexCoordB(data *objData, fs *fieldSplitter) error {
	if fs.count < minTexCoordFields {
		return decodeErr(errBadVertex.Error())
	}
	u, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return decodeErrCause(errBadVertex.Error(), err)
	}
	v, err := parseFloatBytes(fs.get(2)) //nolint:mnd // v-coord
	if err != nil {
		return decodeErrCause(errBadVertex.Error(), err)
	}
	data.texCoords = append(data.texCoords, [2]float32{u, v})
	return nil
}

func parseNormalB(data *objData, fs *fieldSplitter) error {
	if fs.count < minPositionFields {
		return decodeErr(errBadVertex.Error())
	}
	v, err := parseFloat3Bytes(fs.get(1), fs.get(2), fs.get(3)) //nolint:mnd // xyz coordinates
	if err != nil {
		return decodeErrCause(errBadVertex.Error(), err)
	}
	data.normals = append(data.normals, v)
	return nil
}

const maxStackVerts = 8

func parseFaceB(g *group, fs *fieldSplitter) error {
	if fs.count < minFaceFields {
		return decodeErr(errBadFace.Error())
	}
	n := fs.count - 1
	start := len(g.vertRefs)

	var stackBuf [maxStackVerts]vertexRef
	var refs []vertexRef
	if n <= maxStackVerts {
		refs = stackBuf[:n]
	} else {
		refs = make([]vertexRef, n)
	}

	for i := 1; i < fs.count; i++ {
		ref, err := parseFaceVertexBytes(fs.get(i))
		if err != nil {
			return err
		}
		refs[i-1] = ref
	}
	g.vertRefs = append(g.vertRefs, refs...)
	g.faces = append(g.faces, face{vertStart: start, vertCount: n})
	return nil
}

func parseLineB(g *group, fs *fieldSplitter, data *objData) {
	if fs.count < minLineFields {
		return
	}
	for i := 1; i < fs.count-1; i++ {
		ref0, err0 := parseFaceVertexBytes(fs.get(i))
		ref1, err1 := parseFaceVertexBytes(fs.get(i + 1))
		if err0 != nil || err1 != nil {
			continue
		}
		idx0 := resolveIndex(ref0.pos, len(data.positions))
		idx1 := resolveIndex(ref1.pos, len(data.positions))
		if idx0 >= 0 && idx1 >= 0 {
			g.lines = append(g.lines, uint32(idx0), uint32(idx1)) //nolint:gosec // bounded by positions
		}
	}
}

func parsePointB(g *group, fs *fieldSplitter, data *objData) {
	if fs.count < minPointFields {
		return
	}
	for i := 1; i < fs.count; i++ {
		ref, err := parseFaceVertexBytes(fs.get(i))
		if err != nil {
			continue
		}
		idx := resolveIndex(ref.pos, len(data.positions))
		if idx >= 0 {
			g.points = append(g.points, uint32(idx)) //nolint:gosec // bounded by positions
		}
	}
}

func parseVpB(data *objData, fs *fieldSplitter) error {
	if fs.count < minVpFields {
		return decodeErr(errBadVp.Error())
	}
	u, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return decodeErrCause(errBadVp.Error(), err)
	}
	var v, w float32
	if fs.count >= 3 { //nolint:mnd // optional param
		v, _ = parseFloatBytes(fs.get(2)) //nolint:errcheck,mnd // optional
	}
	if fs.count >= 4 { //nolint:mnd // optional param
		w, _ = parseFloatBytes(fs.get(3)) //nolint:errcheck,mnd // optional
	}
	if w == 0 {
		w = 1
	}
	data.paramVerts = append(data.paramVerts, [3]float32{u, v, w})
	return nil
}

func parseCstypeB(state *freeformState, fs *fieldSplitter) {
	if fs.count < minCstypeFields {
		return
	}
	state.rational = false
	idx := 1
	if bytes.Equal(fs.get(idx), typRat) {
		state.rational = true
		idx++
	}
	if idx >= fs.count {
		return
	}
	t := fs.get(idx)
	switch {
	case bytes.Equal(t, typBezier):
		state.typ = csBezier
	case bytes.Equal(t, typBspline):
		state.typ = csBSpline
	case bytes.Equal(t, typCardinal):
		state.typ = csCardinal
	case bytes.Equal(t, typTaylor):
		state.typ = csTaylor
	case bytes.Equal(t, typBmatrix):
		state.typ = csBasisMatrix
	}
}

func parseDegB(state *freeformState, fs *fieldSplitter) {
	if fs.count < minDegFields {
		return
	}
	du, err := parseIntBytes(fs.get(1))
	if err != nil {
		return
	}
	state.degU = du
	state.degV = du
	if fs.count >= 3 { //nolint:mnd // optional v degree
		if dv, err := parseIntBytes(fs.get(2)); err == nil { //nolint:mnd // var degV
			state.degV = dv
		}
	}
}

func parseBmatB(state *freeformState, fs *fieldSplitter) {
	if fs.count < minBmatFields {
		return
	}
	dir := fs.get(1)
	isV := len(dir) == 1 && dir[0] == 'v'

	vals := make([]float32, 0, fs.count-2) //nolint:mnd // index offset
	for i := 2; i < fs.count; i++ {
		v, err := parseFloatBytes(fs.get(i))
		if err != nil {
			continue
		}
		vals = append(vals, v)
	}

	if isV {
		state.bmatV = vals
	} else {
		state.bmatU = vals
	}
}

func parseStepB(state *freeformState, fs *fieldSplitter) {
	if fs.count < minStepFields {
		return
	}
	su, err := parseIntBytes(fs.get(1))
	if err != nil {
		return
	}
	state.stepU = su
	state.stepV = su
	if fs.count >= 3 { //nolint:mnd // param fields
		if sv, err := parseIntBytes(fs.get(2)); err == nil { //nolint:mnd // optional v step
			state.stepV = sv
		}
	}
}

func parseCurvB(state freeformState, fs *fieldSplitter) (*curve3D, error) {
	if fs.count < minCurvFields {
		return nil, decodeErr(errBadCurv.Error())
	}
	u0, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return nil, decodeErrCause(errBadCurv.Error(), err)
	}
	u1, err := parseFloatBytes(fs.get(2)) //nolint:mnd // domain end
	if err != nil {
		return nil, decodeErrCause(errBadCurv.Error(), err)
	}
	indices := make([]int, 0, fs.count-3) //nolint:mnd // index offset
	for i := 3; i < fs.count; i++ {
		v, err := parseIntBytes(fs.get(i))
		if err != nil {
			return nil, decodeErrCause(errBadCurvIdx.Error(), err)
		}
		indices = append(indices, v)
	}
	return &curve3D{u0: u0, u1: u1, ctrlIdx: indices, state: state}, nil
}

func parseCurv2B(state freeformState, fs *fieldSplitter) (*curve2D, error) {
	if fs.count < minCurv2Fields {
		return nil, decodeErr(errBadCurv2.Error())
	}
	indices := make([]int, 0, fs.count-1)
	for i := 1; i < fs.count; i++ {
		v, err := parseIntBytes(fs.get(i))
		if err != nil {
			return nil, decodeErrCause(errBadCurvIdx.Error(), err)
		}
		indices = append(indices, v)
	}
	return &curve2D{ctrlIdx: indices, state: state}, nil
}

func parseSurfB(state freeformState, fs *fieldSplitter) (*surface, error) {
	if fs.count < minSurfFields {
		return nil, decodeErr(errBadSurf.Error())
	}
	s0, err := parseFloatBytes(fs.get(1))
	if err != nil {
		return nil, decodeErrCause(errBadSurf.Error(), err)
	}
	s1, err := parseFloatBytes(fs.get(2)) //nolint:mnd // u-domain end
	if err != nil {
		return nil, decodeErrCause(errBadSurf.Error(), err)
	}
	t0, err := parseFloatBytes(fs.get(3)) //nolint:mnd // u-domain start
	if err != nil {
		return nil, decodeErrCause(errBadSurf.Error(), err)
	}
	t1, err := parseFloatBytes(fs.get(4)) //nolint:mnd // u-domain end
	if err != nil {
		return nil, decodeErrCause(errBadSurf.Error(), err)
	}

	verts := make([]surfVert, 0, fs.count-5) //nolint:mnd // base fields
	for i := 5; i < fs.count; i++ {
		ref, err := parseFaceVertexBytes(fs.get(i))
		if err != nil {
			return nil, decodeErrCause(errBadSurfVtx.Error(), err)
		}
		verts = append(verts, surfVert(ref))
	}

	return &surface{
		s0: s0, s1: s1, t0: t0, t1: t1,
		ctrlPts: verts,
		state:   state,
	}, nil
}

func parseParmB(c3 *curve3D, c2 *curve2D, s *surface, fs *fieldSplitter) error {
	if fs.count < minParmFields {
		return decodeErr(errBadParm.Error())
	}

	dir := fs.get(1)
	isV := len(dir) == 1 && dir[0] == 'v'
	startIdx := 2
	if len(dir) != 1 || (dir[0] != 'u' && dir[0] != 'v') {
		startIdx = 1
	}

	knots := make([]float32, 0, fs.count-startIdx)
	for i := startIdx; i < fs.count; i++ {
		v, err := parseFloatBytes(fs.get(i))
		if err != nil {
			return decodeErrCause(errBadParm.Error(), err)
		}
		knots = append(knots, v)
	}

	switch {
	case s != nil:
		if isV {
			s.knotsV = knots
		} else {
			s.knotsU = knots
		}
	case c3 != nil:
		c3.knotsU = knots
	case c2 != nil:
		c2.knotsU = knots
	}
	return nil
}

func parseTrimHoleB(s *surface, fs *fieldSplitter, isHole bool) error {
	if s == nil || fs.count < minTrimFields {
		return nil
	}

	var segs []trimSeg
	for i := 1; i+2 < fs.count; i += 3 {
		u0, err := parseFloatBytes(fs.get(i))
		if err != nil {
			return decodeErrCause(errBadTrim.Error(), err)
		}
		u1, err := parseFloatBytes(fs.get(i + 1))
		if err != nil {
			return decodeErrCause(errBadTrim.Error(), err)
		}
		idx, err := parseIntBytes(fs.get(i + 2)) //nolint:mnd // skip 2 loop vars
		if err != nil {
			return decodeErrCause(errBadTrim.Error(), err)
		}
		segs = append(segs, trimSeg{u0: u0, u1: u1, curv2Idx: idx})
	}

	tl := trimLoop{segments: segs}
	if isHole {
		s.holes = append(s.holes, tl)
	} else {
		s.trims = append(s.trims, tl)
	}
	return nil
}

func parseEndB(data *objData, c3 *curve3D, c2 *curve2D, s *surface) {
	switch {
	case c3 != nil:
		data.curves3D = append(data.curves3D, *c3)
	case c2 != nil:
		data.curves2D = append(data.curves2D, *c2)
	case s != nil:
		data.surfaces = append(data.surfaces, *s)
	}
}
