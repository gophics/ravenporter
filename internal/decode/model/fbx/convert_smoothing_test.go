package fbx

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeNormalsFromSmoothing(t *testing.T) {
	tests := []struct {
		name     string
		pos      [][3]float32
		idx      []uint32
		groups   []int32
		wantLen  int
		sameSoft bool
	}{
		{
			"SameSmoothGroup",
			[][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}},
			[]uint32{0, 1, 2, 1, 3, 2},
			[]int32{1, 1},
			6,
			true,
		},
		{
			"DifferentSmoothGroups",
			[][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}},
			[]uint32{0, 1, 2, 1, 3, 2},
			[]int32{1, 2},
			6,
			false,
		},
		{
			"EmptyIndices",
			[][3]float32{{0, 0, 0}},
			nil,
			nil,
			0,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normals := computeNormalsFromSmoothing(tt.pos, tt.idx, tt.groups)
			if tt.wantLen == 0 {
				assert.Nil(t, normals)
				return
			}
			require.Len(t, normals, tt.wantLen)

			for i, n := range normals {
				length := math.Sqrt(float64(n[0]*n[0] + n[1]*n[1] + n[2]*n[2]))
				assert.InDelta(t, 1.0, length, 0.01, "normal[%d] unit length", i)
			}

			if tt.sameSoft {
				assert.Equal(t, normals[1], normals[3], "shared vertex same smooth group")
			}
		})
	}
}

func TestParseSmoothingGroups(t *testing.T) {
	tests := []struct {
		name   string
		node   fbxNode
		expect []int32
	}{
		{
			"HasData",
			fbxNode{children: []fbxNode{
				{name: leSmoothData, properties: []fbxProp{i32Prop([]int32{1, 2, 1})}},
			}},
			[]int32{1, 2, 1},
		},
		{
			"NoData",
			fbxNode{children: []fbxNode{
				{name: "other", properties: []fbxProp{i32Prop([]int32{1})}},
			}},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSmoothingGroups(&tt.node)
			assert.Equal(t, tt.expect, result)
		})
	}
}
