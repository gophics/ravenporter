package fbx

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gophics/ravenporter/ir"
)

func TestResolveASCIISkins(t *testing.T) {
	tests := []struct {
		name             string
		clusters         []asciiCluster
		deformerIDs      []int64
		deformerIDsTypes []string
		conns            []asciiConnection
		meshNodes        int
		wantSkels        int
		wantJoints       int
	}{
		{
			name:      "NoClusters",
			meshNodes: 1,
		},
		{
			name: "OneSkinTwoClusters",
			clusters: []asciiCluster{
				{id: 301, idxs: []int32{0, 1}, weights: []float64{1.0, 0.5}, ibm: [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}},
				{id: 302, idxs: []int32{1, 2}, weights: []float64{0.5, 1.0}, ibm: [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 1, 2, 3, 1}},
			},
			deformerIDs:      []int64{200, 301, 302},
			deformerIDsTypes: []string{"Skin", "Cluster", "Cluster"},
			conns: []asciiConnection{
				{child: 301, parent: 200},
				{child: 302, parent: 200},
				{child: 200, parent: 100},
			},
			meshNodes:  1,
			wantSkels:  1,
			wantJoints: 2,
		},
		{
			name: "ClusterWithoutSkin",
			clusters: []asciiCluster{
				{id: 301, ibm: [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}},
			},
			deformerIDs:      []int64{301},
			deformerIDsTypes: []string{"Cluster"},
			meshNodes:        1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &ir.Asset{}
			for range tt.meshNodes {
				asset.Nodes = append(asset.Nodes, ir.Node{
					LODGroupIndex: ir.NoIndex,
					Name:          "MeshNode",
					MeshIndex:     0,
					SkinIndex:     ir.NoIndex,
					CameraIndex:   ir.NoIndex,
					LightIndex:    ir.NoIndex,
				})
			}
			asset.Meshes = append(asset.Meshes, &ir.Mesh{Name: "TestMesh"})

			res := asciiParseResult{
				clusters:      tt.clusters,
				deformerIDs:   tt.deformerIDs,
				deformerTypes: tt.deformerIDsTypes,
			}
			resolveASCIISkins(asset, res, tt.conns)

			assert.Len(t, asset.Skeletons, tt.wantSkels)
			totalJoints := 0
			for _, skeleton := range asset.Skeletons {
				totalJoints += len(skeleton.Joints)
				assert.Len(t, skeleton.InverseBindMatrices, len(skeleton.Joints))
			}
			assert.Equal(t, tt.wantJoints, totalJoints)

			if tt.wantSkels > 0 {
				assert.GreaterOrEqual(t, asset.Nodes[0].SkinIndex, 0, "mesh node should have skin assigned")
			}
		})
	}
}
