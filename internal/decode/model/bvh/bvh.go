package bvh

import (
	"io"

	"github.com/gophics/ravenporter/detect"
	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
)

const (
	formatName = "BVH"
	extBVH     = ".bvh"
	endSite    = "EndSite"

	channelXpos = "Xposition"
	channelYpos = "Yposition"
	channelZpos = "Zposition"
	channelXrot = "Xrotation"
	channelYrot = "Yrotation"
	channelZrot = "Zrotation"
	channelXsca = "Xscale"
	channelYsca = "Yscale"
	channelZsca = "Zscale"

	defaultFrameTime = 0.033333
	frameTimeFields  = 3
	axisCount        = 3
	offsetFields     = 4
	chanFieldsSkip   = 2
	initialJointCap  = 16
	initialFieldCap  = 8
	minJointFields   = 2
)

var (
	headerBVH  = []byte("HIERARCHY")
	extensions = []string{extBVH}

	kwRootB   = []byte("ROOT")
	kwJointB  = []byte("JOINT")
	kwEndB    = []byte("End")
	kwOffsetB = []byte("OFFSET")
	kwChanB   = []byte("CHANNELS")
	kwMotionB = []byte("MOTION")
	kwFramesB = []byte("Frames:")
)

type Decoder struct{}

func Registrations() []detect.Registration {
	return []detect.Registration{{Format: ir.FormatBVH, Decoder: &Decoder{}}}
}

func (d *Decoder) Probe(r io.ReadSeeker) bool { return decutil.ProbeBytes(r, headerBVH) }

func (d *Decoder) Decode(r detect.ReadSeekerAt, opts detect.DecodeOptions) (*ir.Asset, error) {
	if err := decutil.CheckStreamSize(r, opts.MaxFileSize); err != nil {
		return nil, decutil.DecodeErr(ir.FormatBVH, "size", err)
	}

	data, err := decutil.ReadAll(r)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatBVH, "read", err)
	}

	scan := &decutil.LineScanner{Data: data}

	joints, err := parseHierarchy(scan)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatBVH, "hierarchy", err)
	}

	anim, err := parseMotion(scan, joints)
	if err != nil {
		return nil, decutil.DecodeErr(ir.FormatBVH, "motion", err)
	}

	return buildAsset(joints, anim), nil
}

func (d *Decoder) Extensions() []string { return extensions }
func (d *Decoder) FormatName() string   { return formatName }

func buildAsset(joints []joint, anim *ir.Animation) *ir.Asset {
	asset, scene := ir.NewAssetWithScene(ir.FormatBVH, "")
	asset.UpAxis = ir.YUp

	nodes := make([]ir.Node, len(joints))
	var rootNodes []int

	for i, j := range joints {
		t := ir.IdentityTransform()
		t.Translation = j.offset
		nodes[i] = ir.Node{
			LODGroupIndex: ir.NoIndex,
			ParentIndex:   j.parent,
			Name:          j.name,
			IsJoint:       true,
			MeshIndex:     ir.NoIndex,
			SkinIndex:     ir.NoIndex,
			CameraIndex:   ir.NoIndex,
			LightIndex:    ir.NoIndex,
			Transform:     t,
		}
		if j.parent >= 0 {
			nodes[j.parent].Children = append(nodes[j.parent].Children, i)
		} else {
			rootNodes = append(rootNodes, i)
		}
	}

	asset.Nodes = nodes
	scene.RootNodes = rootNodes

	jointIndices := make([]int, len(joints))
	for i := range joints {
		jointIndices[i] = i
	}

	rootIdx := 0
	if len(rootNodes) > 0 {
		rootIdx = rootNodes[0]
	}

	asset.Skeletons = []*ir.Skeleton{{
		Name:    formatName,
		Joints:  jointIndices,
		RootIdx: rootIdx,
	}}

	if anim != nil {
		asset.Animations = []*ir.Animation{anim}
	}

	return asset
}
