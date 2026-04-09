package fbx

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/ir"
)

const (
	attrCamera = "Camera"
	attrLight  = "Light"
)

func buildASCIIMesh(geo asciiMeshData) *ir.Mesh {
	name := geo.name
	if name == "" {
		name = defaultMeshName
	}

	controlPts := geo.positions
	attrCount := max(len(controlPts), len(geo.normals), len(geo.uvs))

	var positions [][3]float32
	var indices []uint32
	if attrCount > len(controlPts) && len(geo.polyIndices) > 0 {
		positions = expandPositions(controlPts, geo.polyIndices)
		indices = triangulateExpanded(geo.polyIndices)
	} else if len(geo.polyIndices) > 0 {
		positions = controlPts
		indices = triangulatePolygons(geo.polyIndices)
	} else {
		positions = controlPts
		indices = geo.indices
	}

	data := ir.MeshData{
		VertexCount: len(positions),
		Positions:   positions,
		Indices:     indices,
	}
	if len(geo.normals) == len(positions) {
		data.Normals = geo.normals
	}
	if len(geo.uvs) == len(positions) {
		data.TexCoord0 = geo.uvs
	}

	return &ir.Mesh{
		Name: name,
		Primitives: []ir.Primitive{{
			Mode:          ir.Triangles,
			MaterialIndex: ir.NoIndex,
			Data:          data,
		}},
	}
}

func appendUnique(s []string, v string) []string {
	for _, existing := range s {
		if existing == v {
			return s
		}
	}
	return append(s, v)
}

func buildASCIIHierarchy(meshes []*ir.Mesh, geoIDs []int64, models []asciiModelInfo, conns []asciiConnection) []ir.Node {
	geoToMesh := make(map[int64]int, len(geoIDs))
	for i, id := range geoIDs {
		geoToMesh[id] = i
	}

	nodes := make([]ir.Node, len(models))
	modelToNode := make(map[int64]int, len(models))
	for i, m := range models {
		nodes[i] = ir.Node{LODGroupIndex: ir.NoIndex,
			Name:        m.name,
			Transform:   ir.IdentityTransform(),
			MeshIndex:   ir.NoIndex,
			SkinIndex:   ir.NoIndex,
			CameraIndex: ir.NoIndex,
			LightIndex:  ir.NoIndex,
		}
		modelToNode[m.id] = i
	}

	for _, c := range conns {
		if meshIdx, ok := geoToMesh[c.child]; ok {
			if nodeIdx, ok := modelToNode[c.parent]; ok {
				nodes[nodeIdx].MeshIndex = meshIdx
				if meshIdx < len(meshes) {
					nodes[nodeIdx].Name = meshes[meshIdx].Name
					if nodes[nodeIdx].Name == defaultMeshName {
						nodes[nodeIdx].Name = models[nodeIdx].name
					}
				}
			}
			continue
		}
		childIdx, childOK := modelToNode[c.child]
		parentIdx, parentOK := modelToNode[c.parent]
		if childOK && parentOK {
			nodes[parentIdx].Children = append(nodes[parentIdx].Children, childIdx)
		}
	}

	if len(nodes) == 0 {
		return buildASCIINodes(meshes)
	}
	return nodes
}

func wireASCIICamerasLights(asset *ir.Asset, result asciiParseResult, conns []asciiConnection) {
	if len(result.nodeAttrIDs) == 0 {
		return
	}

	attrMap := make(map[int64][2]int, len(result.nodeAttrIDs))
	camIdx, lightIdx := 0, 0
	for i, id := range result.nodeAttrIDs {
		switch result.nodeAttrTypes[i] {
		case attrCamera:
			attrMap[id] = [2]int{0, camIdx}
			camIdx++
		case attrLight:
			attrMap[id] = [2]int{1, lightIdx}
			lightIdx++
		}
	}

	modelIDToNode := make(map[int64]int, len(result.models))
	for i, m := range result.models {
		if i < len(asset.Nodes) {
			modelIDToNode[m.id] = i
		}
	}

	for _, c := range conns {
		attr, ok := attrMap[c.child]
		if !ok {
			continue
		}
		nodeIdx, ok := modelIDToNode[c.parent]
		if !ok || nodeIdx >= len(asset.Nodes) {
			continue
		}
		switch attr[0] {
		case 0:
			asset.Nodes[nodeIdx].CameraIndex = attr[1]
		case 1:
			asset.Nodes[nodeIdx].LightIndex = attr[1]
		}
	}
}

func buildASCIINodes(meshes []*ir.Mesh) []ir.Node {
	nodes := make([]ir.Node, len(meshes))
	for i, m := range meshes {
		nodes[i] = ir.Node{LODGroupIndex: ir.NoIndex,
			Name:        m.Name,
			Transform:   ir.IdentityTransform(),
			MeshIndex:   i,
			SkinIndex:   ir.NoIndex,
			CameraIndex: ir.NoIndex,
			LightIndex:  ir.NoIndex,
		}
	}
	return nodes
}

func wireASCIITextures(asset *ir.Asset, conns []asciiConnection, matIDMap, texIDMap map[int64]int) {
	for _, c := range conns {
		texIdx, isTex := texIDMap[c.child]
		matIdx, isMat := matIDMap[c.parent]
		if !isTex || !isMat || matIdx >= len(asset.Materials) {
			continue
		}
		applyTextureConnection(asset.Materials[matIdx], c.propName, texIdx)
	}
}

func assignASCIIMaterials(asset *ir.Asset, conns []asciiConnection, models []asciiModelInfo, matIDMap map[int64]int) {
	modelMap := make(map[int64]int, len(models))
	for i, m := range models {
		if i < len(asset.Nodes) {
			modelMap[m.id] = i
		}
	}

	for _, c := range conns {
		matIdx, isMat := matIDMap[c.child]
		modelIdx, isModel := modelMap[c.parent]
		if !isMat || !isModel || matIdx >= len(asset.Materials) {
			continue
		}

		meshIdx := asset.Nodes[modelIdx].MeshIndex
		if meshIdx < 0 || meshIdx >= len(asset.Meshes) {
			continue
		}

		mesh := asset.Meshes[meshIdx]
		for i := range mesh.Primitives {
			if mesh.Primitives[i].MaterialIndex == ir.NoIndex {
				mesh.Primitives[i].MaterialIndex = matIdx
			}
		}
	}
}

func extractASCIIVersion(data []byte) string {
	end := bytes.IndexByte(data, '\n')
	if end < 0 {
		end = len(data)
	}
	line := string(data[:end])

	_, rest, ok := strings.Cut(line, "; FBX ")
	if !ok {
		return ""
	}
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return ""
	}
	vParts := strings.Split(parts[0], ".")
	if len(vParts) < 2 { //nolint:mnd // major.minor
		return parts[0]
	}
	major, _ := strconv.Atoi(vParts[0]) //nolint:errcheck // best-effort
	minor, _ := strconv.Atoi(vParts[1]) //nolint:errcheck // best-effort
	return strconv.Itoa(major*1000 + minor*100)
}

func resolveASCIISkins(asset *ir.Asset, res asciiParseResult, conns []asciiConnection) {
	if len(res.clusters) == 0 {
		return
	}

	clusterMap := make(map[int64]*asciiCluster, len(res.clusters))
	for i := range res.clusters {
		clusterMap[res.clusters[i].id] = &res.clusters[i]
	}

	skinIDMap := make(map[int64]bool)
	for i, id := range res.deformerIDs {
		if i < len(res.deformerTypes) && res.deformerTypes[i] == deformerSkin {
			skinIDMap[id] = true
		}
	}

	clusterToSkin := make(map[int64]int64)
	for _, c := range conns {
		if _, ok := clusterMap[c.child]; ok {
			if skinIDMap[c.parent] {
				clusterToSkin[c.child] = c.parent
			}
		}
	}

	skinClusters := make(map[int64][]*asciiCluster)
	for clID, skinID := range clusterToSkin {
		if cl, ok := clusterMap[clID]; ok {
			skinClusters[skinID] = append(skinClusters[skinID], cl)
		}
	}

	for skinID, clusters := range skinClusters {
		var jointIndices []int
		var ibms [][16]float32

		for _, cl := range clusters {
			nodeIdx := len(asset.Nodes)
			asset.Nodes = append(asset.Nodes, ir.Node{LODGroupIndex: ir.NoIndex,
				Name:        strconv.FormatInt(cl.id, 10),
				Transform:   ir.IdentityTransform(),
				MeshIndex:   ir.NoIndex,
				SkinIndex:   ir.NoIndex,
				CameraIndex: ir.NoIndex,
				LightIndex:  ir.NoIndex,
			})
			jointIndices = append(jointIndices, nodeIdx)
			ibms = append(ibms, cl.ibm)
		}

		skel := &ir.Skeleton{
			Name:                defaultSkinName,
			Joints:              jointIndices,
			InverseBindMatrices: ibms,
		}
		skinIdx := len(asset.Skeletons)
		asset.Skeletons = append(asset.Skeletons, skel)

		for _, c := range conns {
			if c.child != skinID {
				continue
			}
			for i := range asset.Nodes {
				if asset.Nodes[i].MeshIndex >= 0 {
					asset.Nodes[i].SkinIndex = skinIdx
					break
				}
			}
		}
	}
}
