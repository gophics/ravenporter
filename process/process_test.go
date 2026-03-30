package process_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

func TestPresetMaxQuality(t *testing.T) {
	// Just verify the preset includes the expected flags.
	assert.NotEqual(t, process.PPFlag(0), process.PresetMaxQuality&process.PPFindInstances)
	assert.NotEqual(t, process.PPFlag(0), process.PresetMaxQuality&process.PPOptimizeMeshes)
	assert.NotEqual(t, process.PPFlag(0), process.PresetMaxQuality&process.PPTriangulate)
}

type dummyStep struct{}

func (s *dummyStep) Name() string                                                { return "Dummy" }
func (s *dummyStep) Flag() process.PPFlag                                        { return 1 << 30 }
func (s *dummyStep) Apply(scene *ir.Asset, _ process.Options) (*ir.Asset, error) { return scene, nil }

func TestNewRegistry(t *testing.T) {
	s := &dummyStep{}
	registry := process.NewRegistry(s)
	require.NotNil(t, registry)
	require.Len(t, registry.Steps(), 1)
	assert.Same(t, s, registry.Steps()[0])
}

func TestBuiltInSteps(t *testing.T) {
	steps := process.BuiltInSteps()
	require.NotEmpty(t, steps)
}

func TestNilMeshGuards(t *testing.T) {
	scene := &ir.Asset{
		Meshes: []*ir.Mesh{nil, {
			Primitives: []ir.Primitive{{
				Data: ir.MeshData{VertexCount: 1, Positions: [][3]float32{{1, 2, 3}}},
			}},
		}},
	}

	steps := []process.PPFlag{
		process.PPGenNormals,
		process.PPGenBoundingBoxes,
		process.PPFlipUVs,
		process.PPFlipWindingOrder,
		process.PPFixInfacingNormals,
		process.PPCalcTangentSpace,
		process.PPFindInvalid,
		process.PPOptimizeMeshes,
		process.PPSplitLargeMeshes,
		process.PPSortByPtype,
		process.PPRemoveComponent,
		process.PPValidate,
	}

	for _, flag := range steps {
		require.NoError(t, process.Apply(scene, flag, process.Options{}))
	}
	assert.NotNil(t, scene.Meshes[1])
}

func TestApplyNormalizesGraph(t *testing.T) {
	asset := &ir.Asset{
		Scenes: []*ir.Scene{{Name: "Scene", RootNodes: []int{0}}},
		Nodes: []ir.Node{
			{Name: "Root", ParentIndex: 0, Children: []int{1}},
			{Name: "Leaf"},
		},
		Meshes: []*ir.Mesh{{Primitives: []ir.Primitive{{Data: ir.MeshData{
			VertexCount: 3,
			Positions:   [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		}}}}},
	}

	require.NoError(t, process.Apply(asset, process.PPFlattenHierarchy, process.Options{}))
	assert.Equal(t, ir.NoIndex, asset.Nodes[0].ParentIndex)
	assert.Equal(t, ir.NoIndex, asset.Nodes[1].ParentIndex)
	assert.Equal(t, []int{0, 1}, asset.RootNodes)
	require.Len(t, asset.Scenes, 1)
	assert.Equal(t, []int{0, 1}, asset.Scenes[0].RootNodes)
}
