package dae

import (
	"context"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	semanticPosition = "POSITION"
	semanticVertex   = "VERTEX"
	semanticNormal   = "NORMAL"
	semanticTexcoord = "TEXCOORD"
	semanticColor    = "COLOR"
	semanticTangent  = "TANGENT"
	semanticInput    = "INPUT"
	semanticOutput   = "OUTPUT"

	upAxisZ = "Z_UP"

	vertsPerTri = 3
)

func convertDocument(sysCtx context.Context, doc *collada) (*ir.Asset, error) {
	if sysCtx == nil {
		sysCtx = context.Background()
	}
	asset := ir.NewAsset(ir.FormatDAE)
	asset.UpAxis = parseUpAxis(doc.Asset.UpAxis)
	asset.Unit = doc.Asset.Unit.Meter
	asset.Metadata.SourceVersion = doc.Version

	if asset.Unit == 0 {
		asset.Unit = 1.0
	}

	effectMap := buildEffectMap(doc.LibEffects.Effects)
	imageMap := buildImageMap(doc.LibImages.Images)
	asset.Materials = convertMaterials(doc.LibMaterials.Materials, effectMap, imageMap, asset)

	matNameMap := buildMaterialNameMap(doc.LibMaterials.Materials, asset.Materials)

	geoMap := make(map[string]int)
	geoPositionMap := make(map[string][][3]float32)
	for i := range doc.LibGeometries.Geometries {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		g := &doc.LibGeometries.Geometries[i]
		sourceMap := buildSourceMap(sysCtx, g.Mesh.Sources)
		vertSrc := resolveVertexSource(g.Mesh.Vertices)
		if posData := sourceMap[vertSrc]; len(posData) > 0 {
			positions := make([][3]float32, len(posData)/vertsPerTri)
			for j := range positions {
				positions[j] = readVec3(posData, j)
			}
			geoPositionMap[g.ID] = positions
		}
		mesh, err := convertGeometry(sysCtx, g, matNameMap)
		if err != nil || mesh == nil {
			continue
		}
		geoMap["#"+g.ID] = len(asset.Meshes)
		asset.Meshes = append(asset.Meshes, mesh)
	}

	camMap := convertDAECameras(doc.LibCameras.Cameras, asset)
	lightMap := convertDAELights(doc.LibLights.Lights, asset)

	if len(doc.LibVisualScenes.Scenes) > 0 {
		convertVisualScene(&doc.LibVisualScenes.Scenes[0], asset, geoMap, camMap, lightMap)
	}

	asset.Animations = convertAnimations(sysCtx, doc.LibAnimations.Animations, asset)
	asset.Skeletons = convertSkins(sysCtx, doc.LibControllers.Controllers, asset, geoPositionMap)

	return asset, nil
}

func parseUpAxis(s string) ir.Axis {
	switch s {
	case upAxisZ:
		return ir.ZUp
	default:
		return ir.YUp
	}
}

//nolint:funlen // context checks add statements
func convertGeometry(
	sysCtx context.Context, g *geometry, matNameMap map[string]int,
) (*ir.Mesh, error) {
	sourceMap := buildSourceMap(sysCtx, g.Mesh.Sources)
	vertSrc := resolveVertexSource(g.Mesh.Vertices)

	var primitives []ir.Primitive

	for _, t := range g.Mesh.Triangles {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		prim := convertTriangles(sysCtx, &t, sourceMap, vertSrc, matNameMap)
		if prim != nil {
			primitives = append(primitives, *prim)
		}
	}

	for _, p := range g.Mesh.Polylist {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		prim := convertPolylist(sysCtx, &p, sourceMap, vertSrc, matNameMap)
		if prim != nil {
			primitives = append(primitives, *prim)
		}
	}

	for _, l := range g.Mesh.Lines {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		prim := convertLines(sysCtx, &l, sourceMap, vertSrc, matNameMap)
		if prim != nil {
			primitives = append(primitives, *prim)
		}
	}

	for _, ts := range g.Mesh.Tristrips {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		prim := convertTristrips(sysCtx, &ts, sourceMap, vertSrc, matNameMap)
		if prim != nil {
			primitives = append(primitives, *prim)
		}
	}

	for _, tf := range g.Mesh.Trifans {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		prim := convertTrifans(sysCtx, &tf, sourceMap, vertSrc, matNameMap)
		if prim != nil {
			primitives = append(primitives, *prim)
		}
	}

	for _, ls := range g.Mesh.Linestrips {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		prim := convertLinestrips(sysCtx, &ls, sourceMap, vertSrc, matNameMap)
		if prim != nil {
			primitives = append(primitives, *prim)
		}
	}

	for _, pg := range g.Mesh.Polygons {
		if err := sysCtx.Err(); err != nil {
			return nil, err
		}
		prim := convertPolygons(sysCtx, &pg, sourceMap, vertSrc, matNameMap)
		if prim != nil {
			primitives = append(primitives, *prim)
		}
	}

	if len(primitives) == 0 {
		return nil, nil
	}

	name := g.Name
	if name == "" {
		name = g.ID
	}

	return &ir.Mesh{
		Name:       name,
		Primitives: primitives,
	}, nil
}

func buildSourceMap(sysCtx context.Context, sources []source) map[string][]float32 {
	m := make(map[string][]float32, len(sources))
	for _, s := range sources {
		m["#"+s.ID] = parseFloats(sysCtx, s.FloatArray.Data)
	}
	return m
}

func resolveVertexSource(v vertices) string {
	for _, inp := range v.Inputs {
		if inp.Semantic == semanticPosition {
			return inp.Source
		}
	}
	return ""
}

func convertTriangles(
	sysCtx context.Context, t *xmlTris, sourceMap map[string][]float32,
	vertSrc string, matNameMap map[string]int,
) *ir.Primitive {
	indices := parseInts(sysCtx, t.P)
	if len(indices) == 0 {
		return nil
	}

	stride := inputStride(t.Inputs)
	off := findOffsets(t.Inputs)

	posData := sourceMap[vertSrc]
	normData := lookupSource(t.Inputs, semanticNormal, sourceMap)
	uvData := lookupSource(t.Inputs, semanticTexcoord, sourceMap)
	colorData := lookupSource(t.Inputs, semanticColor, sourceMap)
	tanData := lookupSource(t.Inputs, semanticTangent, sourceMap)
	uv1Data := lookupTexcoord1(t.Inputs, sourceMap)

	prim := buildPrimitive(
		indices, stride, t.Count, off,
		posData, normData, uvData, colorData, tanData, uv1Data,
	)
	if prim != nil {
		prim.MaterialIndex = resolveMaterialSymbol(t.Material, matNameMap)
	}
	return prim
}

func convertPolylist(
	sysCtx context.Context, p *polylist, sourceMap map[string][]float32,
	vertSrc string, matNameMap map[string]int,
) *ir.Primitive {
	indices := parseInts(sysCtx, p.P)
	vcounts := parseInts(sysCtx, p.VCount)
	if len(indices) == 0 {
		return nil
	}

	stride := inputStride(p.Inputs)
	triIndices := triangulatePolylist(indices, vcounts, stride)

	off := findOffsets(p.Inputs)
	posData := sourceMap[vertSrc]
	normData := lookupSource(p.Inputs, semanticNormal, sourceMap)
	uvData := lookupSource(p.Inputs, semanticTexcoord, sourceMap)
	colorData := lookupSource(p.Inputs, semanticColor, sourceMap)
	tanData := lookupSource(p.Inputs, semanticTangent, sourceMap)
	uv1Data := lookupTexcoord1(p.Inputs, sourceMap)

	triCount := len(triIndices) / (stride * vertsPerTri)
	prim := buildPrimitive(
		triIndices, stride, triCount, off,
		posData, normData, uvData, colorData, tanData, uv1Data,
	)
	if prim != nil {
		prim.MaterialIndex = resolveMaterialSymbol(p.Material, matNameMap)
	}
	return prim
}

type inputOffsets struct {
	pos, norm, uv, color, tangent, uv1 int
}

func buildPrimitive(
	indices []int, stride, triCount int, off inputOffsets,
	posData, normData, uvData, colorData, tanData, uv1Data []float32,
) *ir.Primitive {
	var positions [][3]float32
	var normals [][3]float32
	var uvs [][2]float32
	var colors [][4]float32
	var tangents [][4]float32
	var uv1s [][2]float32
	triIndices := make([]uint32, 0, triCount*vertsPerTri)

	dedup := make(map[[3]int]uint32)

	for i := 0; i < len(indices); i += stride {
		pi := safeIdx(indices, i+off.pos)
		ni := safeIdx(indices, i+off.norm)
		ui := safeIdx(indices, i+off.uv)

		key := [3]int{pi, ni, ui}
		if idx, ok := dedup[key]; ok {
			triIndices = append(triIndices, idx)
			continue
		}

		idx := uint32(len(positions)) //nolint:gosec // bounded by dedup
		triIndices = append(triIndices, idx)
		dedup[key] = idx

		positions = append(positions, readVec3(posData, pi))
		if normData != nil && off.norm >= 0 {
			normals = append(normals, readVec3(normData, ni))
		}
		if uvData != nil && off.uv >= 0 {
			uvs = append(uvs, readVec2(uvData, ui))
		}
		if colorData != nil && off.color >= 0 {
			ci := safeIdx(indices, i+off.color)
			colors = append(colors, readVec4Color(colorData, ci))
		}
		if tanData != nil && off.tangent >= 0 {
			ti := safeIdx(indices, i+off.tangent)
			tangents = append(tangents, readVec4(tanData, ti))
		}
		if uv1Data != nil && off.uv1 >= 0 {
			u1i := safeIdx(indices, i+off.uv1)
			uv1s = append(uv1s, readVec2(uv1Data, u1i))
		}
	}

	data := ir.MeshData{
		VertexCount: len(positions),
		Positions:   positions,
		Indices:     triIndices,
	}
	if len(normals) == len(positions) {
		data.Normals = normals
	}
	if len(uvs) == len(positions) {
		data.TexCoord0 = uvs
	}
	if len(colors) == len(positions) {
		data.Colors0 = colors
	}
	if len(tangents) == len(positions) {
		data.Tangents = tangents
	}
	if len(uv1s) == len(positions) {
		data.TexCoord1 = uv1s
	}

	return &ir.Primitive{
		Mode: ir.Triangles,
		Data: data,
	}
}

func convertLines(
	sysCtx context.Context, t *xmlTris, sourceMap map[string][]float32,
	vertSrc string, matNameMap map[string]int,
) *ir.Primitive {
	indices := parseInts(sysCtx, t.P)
	if len(indices) == 0 {
		return nil
	}

	stride := inputStride(t.Inputs)
	off := findOffsets(t.Inputs)
	posData := sourceMap[vertSrc]

	var positions [][3]float32
	var lineIndices []uint32

	for i := 0; i < len(indices); i += stride {
		pi := safeIdx(indices, i+off.pos)
		idx := uint32(len(positions)) //nolint:gosec // bounded
		lineIndices = append(lineIndices, idx)
		positions = append(positions, readVec3(posData, pi))
	}

	prim := &ir.Primitive{
		Mode: ir.Lines,
		Data: ir.MeshData{
			VertexCount: len(positions),
			Positions:   positions,
			Indices:     lineIndices,
		},
	}
	prim.MaterialIndex = resolveMaterialSymbol(t.Material, matNameMap)
	return prim
}

const stripMinVerts = 3

func convertTristrips(
	sysCtx context.Context, t *xmlTris, sourceMap map[string][]float32,
	vertSrc string, matNameMap map[string]int,
) *ir.Primitive {
	indices := parseInts(sysCtx, t.P)
	stride := inputStride(t.Inputs)
	verts := len(indices) / stride
	if verts < stripMinVerts {
		return nil
	}

	var expanded []int
	for i := 2; i < verts; i++ {
		a, b, c := (i-2)*stride, (i-1)*stride, i*stride //nolint:mnd // strip vertex offsets
		if i%2 == 0 {
			expanded = append(expanded, indices[a:a+stride]...)
			expanded = append(expanded, indices[b:b+stride]...)
		} else {
			expanded = append(expanded, indices[b:b+stride]...)
			expanded = append(expanded, indices[a:a+stride]...)
		}
		expanded = append(expanded, indices[c:c+stride]...)
	}

	off := findOffsets(t.Inputs)
	posData := sourceMap[vertSrc]
	normData := lookupSource(t.Inputs, semanticNormal, sourceMap)
	uvData := lookupSource(t.Inputs, semanticTexcoord, sourceMap)
	colorData := lookupSource(t.Inputs, semanticColor, sourceMap)
	tanData := lookupSource(t.Inputs, semanticTangent, sourceMap)
	uv1Data := lookupTexcoord1(t.Inputs, sourceMap)

	triCount := len(expanded) / (stride * vertsPerTri)
	prim := buildPrimitive(expanded, stride, triCount, off, posData, normData, uvData, colorData, tanData, uv1Data)
	if prim != nil {
		prim.MaterialIndex = resolveMaterialSymbol(t.Material, matNameMap)
	}
	return prim
}

func convertTrifans(
	sysCtx context.Context, t *xmlTris, sourceMap map[string][]float32,
	vertSrc string, matNameMap map[string]int,
) *ir.Primitive {
	indices := parseInts(sysCtx, t.P)
	stride := inputStride(t.Inputs)
	verts := len(indices) / stride
	if verts < stripMinVerts {
		return nil
	}

	var expanded []int
	for i := 2; i < verts; i++ {
		expanded = append(expanded, indices[0:stride]...)
		expanded = append(expanded, indices[(i-1)*stride:i*stride]...)
		expanded = append(expanded, indices[i*stride:(i+1)*stride]...)
	}

	off := findOffsets(t.Inputs)
	posData := sourceMap[vertSrc]
	normData := lookupSource(t.Inputs, semanticNormal, sourceMap)
	uvData := lookupSource(t.Inputs, semanticTexcoord, sourceMap)
	colorData := lookupSource(t.Inputs, semanticColor, sourceMap)
	tanData := lookupSource(t.Inputs, semanticTangent, sourceMap)
	uv1Data := lookupTexcoord1(t.Inputs, sourceMap)

	triCount := len(expanded) / (stride * vertsPerTri)
	prim := buildPrimitive(expanded, stride, triCount, off, posData, normData, uvData, colorData, tanData, uv1Data)
	if prim != nil {
		prim.MaterialIndex = resolveMaterialSymbol(t.Material, matNameMap)
	}
	return prim
}

const lineMinVerts = 2

func convertLinestrips(
	sysCtx context.Context, t *xmlTris, sourceMap map[string][]float32,
	vertSrc string, matNameMap map[string]int,
) *ir.Primitive {
	indices := parseInts(sysCtx, t.P)
	stride := inputStride(t.Inputs)
	off := findOffsets(t.Inputs)
	posData := sourceMap[vertSrc]
	verts := len(indices) / stride
	if verts < lineMinVerts {
		return nil
	}

	var positions [][3]float32
	var lineIndices []uint32
	for i := 0; i < verts-1; i++ {
		a := i * stride
		b := (i + 1) * stride
		piA := safeIdx(indices, a+off.pos)
		piB := safeIdx(indices, b+off.pos)
		idxA := uint32(len(positions)) //nolint:gosec // bounded
		positions = append(positions, readVec3(posData, piA))
		idxB := uint32(len(positions)) //nolint:gosec // bounded
		positions = append(positions, readVec3(posData, piB))
		lineIndices = append(lineIndices, idxA, idxB)
	}

	prim := &ir.Primitive{
		Mode: ir.Lines,
		Data: ir.MeshData{
			VertexCount: len(positions),
			Positions:   positions,
			Indices:     lineIndices,
		},
	}
	prim.MaterialIndex = resolveMaterialSymbol(t.Material, matNameMap)
	return prim
}

func convertPolygons(
	sysCtx context.Context, pg *xmlPolygons, sourceMap map[string][]float32,
	vertSrc string, matNameMap map[string]int,
) *ir.Primitive {
	if len(pg.Ps) == 0 {
		return nil
	}

	stride := inputStride(pg.Inputs)
	var allIndices []int
	var vcounts []int

	for _, ps := range pg.Ps {
		if err := sysCtx.Err(); err != nil {
			return nil
		}
		polyInds := parseInts(sysCtx, ps)
		if len(polyInds) == 0 {
			continue
		}
		allIndices = append(allIndices, polyInds...)
		vcounts = append(vcounts, len(polyInds)/stride)
	}

	if len(allIndices) == 0 {
		return nil
	}

	triIndices := triangulatePolylist(allIndices, vcounts, stride)

	off := findOffsets(pg.Inputs)
	posData := sourceMap[vertSrc]
	normData := lookupSource(pg.Inputs, semanticNormal, sourceMap)
	uvData := lookupSource(pg.Inputs, semanticTexcoord, sourceMap)
	colorData := lookupSource(pg.Inputs, semanticColor, sourceMap)
	tanData := lookupSource(pg.Inputs, semanticTangent, sourceMap)
	uv1Data := lookupTexcoord1(pg.Inputs, sourceMap)

	triCount := len(triIndices) / (stride * vertsPerTri)
	prim := buildPrimitive(triIndices, stride, triCount, off, posData, normData, uvData, colorData, tanData, uv1Data)
	if prim != nil {
		prim.MaterialIndex = resolveMaterialSymbol(pg.Material, matNameMap)
	}
	return prim
}

func triangulatePolylist(indices, vcounts []int, stride int) []int {
	estCap := 0
	for _, vc := range vcounts {
		if vc > 2 { //nolint:mnd // fan-triangulation: polygon must have >2 vertices
			estCap += (vc - 2) * 3 * stride //nolint:mnd // (vc-2) triangles × 3 vertices
		}
	}

	out := make([]int, 0, estCap)
	offset := 0
	for _, vc := range vcounts {
		expectedEnd := offset + vc*stride
		if vc < 0 || expectedEnd > len(indices) {
			break
		}
		for j := 2; j < vc; j++ {
			out = append(out, indices[offset:offset+stride]...)
			out = append(out, indices[offset+(j-1)*stride:offset+j*stride]...)
			out = append(out, indices[offset+j*stride:offset+(j+1)*stride]...)
		}
		offset = expectedEnd
	}
	return out
}

func inputStride(inputs []input) int {
	maxOff := 0
	for _, inp := range inputs {
		if inp.Offset > maxOff {
			maxOff = inp.Offset
		}
	}
	return maxOff + 1
}

func findOffsets(inputs []input) inputOffsets {
	off := inputOffsets{pos: 0, norm: -1, uv: -1, color: -1, tangent: -1, uv1: -1}
	uvCount := 0
	for _, inp := range inputs {
		switch inp.Semantic {
		case semanticVertex, semanticPosition:
			off.pos = inp.Offset
		case semanticNormal:
			off.norm = inp.Offset
		case semanticTexcoord:
			switch uvCount {
			case 0:
				off.uv = inp.Offset
			case 1:
				off.uv1 = inp.Offset
			}
			uvCount++
		case semanticColor:
			off.color = inp.Offset
		case semanticTangent:
			off.tangent = inp.Offset
		}
	}
	return off
}

func lookupSource(inputs []input, semantic string, sourceMap map[string][]float32) []float32 {
	for _, inp := range inputs {
		if inp.Semantic == semantic {
			return sourceMap[inp.Source]
		}
	}
	return nil
}

func readVec3(data []float32, idx int) [3]float32 {
	base := idx * vertsPerTri
	if base+2 >= len(data) {
		return [3]float32{}
	}
	return [3]float32{data[base], data[base+1], data[base+2]}
}

func readVec2(data []float32, idx int) [2]float32 {
	base := idx * 2 //nolint:mnd // vec2 stride
	if base+1 >= len(data) {
		return [2]float32{}
	}
	return [2]float32{data[base], data[base+1]}
}

const vec4Stride = 4

func readVec4(data []float32, idx int) [4]float32 {
	base := idx * vec4Stride
	if base+3 >= len(data) {
		return [4]float32{}
	}
	return [4]float32{data[base], data[base+1], data[base+2], data[base+3]}
}

func readVec4Color(data []float32, idx int) [4]float32 {
	base := idx * vertsPerTri
	if base+2 >= len(data) {
		return [4]float32{1, 1, 1, 1}
	}
	r, g, b := data[base], data[base+1], data[base+2]
	base4 := idx * vec4Stride
	if base4+3 < len(data) && len(data)%vec4Stride == 0 {
		return [4]float32{data[base4], data[base4+1], data[base4+2], data[base4+3]}
	}
	return [4]float32{r, g, b, 1.0}
}

func lookupTexcoord1(
	inputs []input, sourceMap map[string][]float32,
) []float32 {
	for _, inp := range inputs {
		if inp.Semantic == semanticTexcoord && inp.Set == "1" {
			return sourceMap[inp.Source]
		}
	}
	count := 0
	for _, inp := range inputs {
		if inp.Semantic == semanticTexcoord {
			if count == 1 {
				return sourceMap[inp.Source]
			}
			count++
		}
	}
	return nil
}

func safeIdx(indices []int, off int) int {
	if off >= 0 && off < len(indices) {
		return indices[off]
	}
	return 0
}

func countFields(s string) int {
	n := 0
	inField := false
	for i := range len(s) {
		if s[i] == ' ' {
			inField = false
		} else if !inField {
			inField = true
			n++
		}
	}
	return n
}

func parseFloats(sysCtx context.Context, s string) []float32 {
	out := make([]float32, 0, countFields(s))
	iters := 0
	for s != "" {
		if iters%1024 == 0 && sysCtx.Err() != nil {
			return nil
		}
		iters++
		i := strings.IndexByte(s, ' ')
		var field string
		if i < 0 {
			field = s
			s = ""
		} else {
			field = s[:i]
			s = s[i+1:]
		}
		if field != "" {
			out = append(out, decutil.ParseF32(field))
		}
	}
	return out
}

func parseInts(sysCtx context.Context, s string) []int {
	out := make([]int, 0, countFields(s))
	iters := 0
	for s != "" {
		if iters%1024 == 0 && sysCtx.Err() != nil {
			return nil
		}
		iters++
		i := strings.IndexByte(s, ' ')
		var field string
		if i < 0 {
			field = s
			s = ""
		} else {
			field = s[:i]
			s = s[i+1:]
		}
		if field != "" {
			if v, err := strconv.Atoi(field); err == nil {
				out = append(out, v)
			}
		}
	}
	return out
}
