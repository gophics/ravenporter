package fbx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gophics/ravenporter/ir"
)

func intProp(v int64) fbxProp     { return fbxProp{intVal: v} }
func strProp(v string) fbxProp    { return fbxProp{strVal: v} }
func i32Prop(v []int32) fbxProp   { return fbxProp{arrI32: v} }
func i64Prop(v []int64) fbxProp   { return fbxProp{arrI64: v} }
func f64Prop(v []float64) fbxProp { return fbxProp{arrF64: v} }

func obj(name string, id int64, displayName string, children ...fbxNode) fbxNode {
	return fbxNode{
		name:       name,
		properties: []fbxProp{intProp(id), strProp(displayName)},
		children:   children,
	}
}

func objTyped(id int64, displayName, subType string, children ...fbxNode) fbxNode {
	return fbxNode{
		name:       objDeformer,
		properties: []fbxProp{intProp(id), strProp(displayName), strProp(subType)},
		children:   children,
	}
}

func connOONode(childID, parentID int64) fbxNode {
	return fbxNode{
		name:       "C",
		properties: []fbxProp{strProp("OO"), intProp(childID), intProp(parentID)},
	}
}

func TestResolveAnimations(t *testing.T) {
	tests := []struct {
		name       string
		objects    fbxNode
		conns      fbxNode
		wantChans  int
		wantTarget ir.ChannelTarget
		wantTimes  int
		wantName   string
	}{
		{
			name: "Translation",
			objects: fbxNode{name: nodeObjects, children: []fbxNode{
				obj(objAnimStack, 100, "Walk"),
				obj(objAnimCurveNode, 200, animTargetT),
				obj(objAnimCurve, 300, "AnimCurve",
					fbxNode{name: animKeyTime, properties: []fbxProp{i64Prop([]int64{0, int64(fbxKTimeScale)})}},
					fbxNode{name: animKeyValue, properties: []fbxProp{f64Prop([]float64{0.0, 5.0})}},
				),
				obj(objModel, 400, "Bone01"),
			}},
			conns: fbxNode{name: nodeConnections, children: []fbxNode{
				connOONode(200, 400), connOONode(300, 200),
			}},
			wantChans: 1, wantTarget: ir.TargetTranslation, wantTimes: 2, wantName: "Walk",
		},
		{
			name: "Rotation",
			objects: fbxNode{name: nodeObjects, children: []fbxNode{
				obj(objAnimStack, 100, "Rotate"),
				obj(objAnimCurveNode, 200, animTargetR),
				obj(objAnimCurve, 300, "Curve",
					fbxNode{name: animKeyTime, properties: []fbxProp{i64Prop([]int64{0, int64(fbxKTimeScale)})}},
					fbxNode{name: animKeyValue, properties: []fbxProp{f64Prop([]float64{0.0, 90.0})}},
				),
				obj(objModel, 400, "Joint"),
			}},
			conns: fbxNode{name: nodeConnections, children: []fbxNode{
				connOONode(200, 400), connOONode(300, 200),
			}},
			wantChans: 1, wantTarget: ir.TargetRotation, wantTimes: 2, wantName: "Rotate",
		},
		{
			name: "NoData",
			objects: fbxNode{name: nodeObjects, children: []fbxNode{
				obj(objAnimStack, 100, "Empty"),
				obj(objAnimCurveNode, 200, animTargetT),
				obj(objAnimCurve, 300, "Curve"), // no KeyTime/KeyValue
				obj(objModel, 400, "Node"),
			}},
			conns: fbxNode{name: nodeConnections, children: []fbxNode{
				connOONode(200, 400), connOONode(300, 200),
			}},
			wantChans: 0, wantTimes: 0, wantName: "Empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := convertFBX([]fbxNode{tt.objects, tt.conns}, 7400)
			require.Len(t, asset.Animations, 1)
			assert.Equal(t, tt.wantName, asset.Animations[0].Name)

			if tt.wantChans > 0 {
				require.Len(t, asset.Animations[0].Channels, tt.wantChans)
				ch := asset.Animations[0].Channels[0]
				assert.Equal(t, tt.wantTarget, ch.Target)
				assert.Equal(t, ir.InterpolationLinear, ch.Interpolation)
				require.Len(t, ch.Times, tt.wantTimes)
			}
		})
	}
}

func TestResolveSkins(t *testing.T) {
	ibm := []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

	objects := fbxNode{
		name: nodeObjects,
		children: []fbxNode{
			obj(objModel, 600, "Joint1"),
			obj(objModel, 601, "Joint2"),
			objTyped(500, "Cluster1", deformerCluster,
				fbxNode{name: clusterIndexes, properties: []fbxProp{i32Prop([]int32{0, 1, 2})}},
				fbxNode{name: clusterWeights, properties: []fbxProp{f64Prop([]float64{1.0, 0.8, 0.6})}},
				fbxNode{name: clusterTransformLink, properties: []fbxProp{f64Prop(ibm)}},
			),
			objTyped(501, "Cluster2", deformerCluster,
				fbxNode{name: clusterIndexes, properties: []fbxProp{i32Prop([]int32{2, 3})}},
				fbxNode{name: clusterWeights, properties: []fbxProp{f64Prop([]float64{0.4, 1.0})}},
				fbxNode{name: clusterTransformLink, properties: []fbxProp{f64Prop(ibm)}},
			),
		},
	}

	connections := fbxNode{
		name: nodeConnections,
		children: []fbxNode{
			connOONode(500, 600), connOONode(501, 601),
		},
	}

	asset := convertFBX([]fbxNode{objects, connections}, 7400)

	require.Len(t, asset.Skeletons, 1)
	skel := asset.Skeletons[0]
	assert.Equal(t, "FBX_Skin", skel.Name)
	require.Len(t, skel.Joints, 2)
	assert.Equal(t, 0, skel.Joints[0])
	assert.Equal(t, 1, skel.Joints[1])
	assert.True(t, asset.Nodes[0].IsJoint)
	assert.True(t, asset.Nodes[1].IsJoint)
}

func TestExtractClusterEmpty(t *testing.T) {
	node := fbxNode{name: objDeformer, properties: []fbxProp{intProp(1), strProp("x"), strProp(deformerCluster)}}
	cl := extractCluster(&node, 1)
	assert.Equal(t, int64(1), cl.id)
	assert.Nil(t, cl.idxs)
	assert.Nil(t, cl.weights)
}

func TestExtractAnimTargetNames(t *testing.T) {
	tests := []struct {
		name   string
		expect string
	}{
		{"T", animTargetT},
		{"Translation", animTargetT},
		{"R", animTargetR},
		{"Rotation", animTargetR},
		{"S", animTargetS},
		{"Scaling", animTargetS},
		{"Visibility", "Visibility"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := fbxNode{properties: []fbxProp{intProp(0), strProp(tt.name)}}
			assert.Equal(t, tt.expect, extractAnimTarget(&node))
		})
	}
}

func TestFBXFlagsToInterp(t *testing.T) {
	tests := []struct {
		name   string
		flag   int32
		expect ir.Interpolation
	}{
		{"Zero/Linear", 0x0, ir.InterpolationLinear},
		{"Constant", 0x2, ir.InterpolationStep},
		{"ConstantNext", 0x6, ir.InterpolationStep},
		{"Cubic", 0x8, ir.InterpolationCubicSpline},
		{"CubicConstant", 0xA, ir.InterpolationCubicSpline},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, fbxFlagsToInterp(tt.flag))
		})
	}
}

func p70Property(name string, vals ...float64) fbxNode {
	props := []fbxProp{strProp(name), strProp(""), strProp(""), strProp("")}
	for _, v := range vals {
		props = append(props, fbxProp{floatVal: v})
	}
	return fbxNode{name: nodeP, properties: props}
}

func TestReadModelTransform(t *testing.T) {
	tests := []struct {
		name  string
		props []fbxNode
		check func(t *testing.T, tr ir.Transform)
	}{
		{"Identity", nil, func(t *testing.T, tr ir.Transform) {
			assert.Equal(t, [3]float32{0, 0, 0}, tr.Translation)
			assert.Equal(t, [3]float32{1, 1, 1}, tr.Scale)
		}},
		{"TranslationOnly", []fbxNode{
			p70Property(propLclTranslate, 10, 20, 30),
		}, func(t *testing.T, tr ir.Transform) {
			assert.Equal(t, [3]float32{10, 20, 30}, tr.Translation)
		}},
		{"ScaleOnly", []fbxNode{
			p70Property(propLclScaling, 2, 3, 4),
		}, func(t *testing.T, tr ir.Transform) {
			assert.Equal(t, [3]float32{2, 3, 4}, tr.Scale)
		}},
		{"WithGeometricTranslation", []fbxNode{
			p70Property(propLclTranslate, 1, 0, 0),
			p70Property(propGeomTranslate, 0, 5, 0),
		}, func(t *testing.T, tr ir.Transform) {
			assert.NotEqual(t, [16]float32{}, tr.Matrix, "geometric offsets produce a matrix")
			assert.InDelta(t, float32(1), tr.Matrix[12], 0.01, "node translation X in matrix")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p70 := &fbxNode{name: nodeProperties70, children: tt.props}
			tr := readModelTransform(p70)
			tt.check(t, tr)
		})
	}
}
