package models

import (
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type populateArmatureDataStep struct{}

func (s *populateArmatureDataStep) Name() string      { return "PopulateArmatureData" }
func (s *populateArmatureDataStep) Flag() core.PPFlag { return core.PPPopulateArmatureData }

func (s *populateArmatureDataStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Skeletons {
		skel := asset.Skeletons[i]
		if skel == nil {
			continue
		}
		if need := len(skel.Joints) - len(skel.InverseBindMatrices); need > 0 {
			extras := make([][16]float32, need)
			for ei := range extras {
				extras[ei] = mathx.IdentityMat4
			}
			skel.InverseBindMatrices = append(skel.InverseBindMatrices, extras...)
		}

		for _, jIdx := range skel.Joints {
			if jIdx >= 0 && jIdx < len(asset.Nodes) {
				asset.Nodes[jIdx].IsJoint = true
			}
		}
	}
	return asset, nil
}
