package core

import (
	"math"
	"sort"
	"sync"

	"github.com/gophics/ravenporter/ir"
)

// stepOrder defines deterministic execution order for registered steps.
// Lower values run first. Steps not in this map run last.
var stepOrder = map[PPFlag]int{
	PPTriangulate:              0,
	PPForceGenNormals:          1,
	PPDropNormals:              1,
	PPGenNormals:               2,
	PPGenSmoothNormals:         2,
	PPCalcTangentSpace:         3,
	PPJoinIdenticalVertices:    4,
	PPRemoveDegenerates:        5,
	PPFixWinding:               6,
	PPFixInfacingNormals:       6,
	PPGenUVCoords:              7,
	PPTransformUVCoords:        8,
	PPFlipUVs:                  8,
	PPFlipWindingOrder:         8,
	PPSortByPtype:              9,
	PPSplitLargeMeshes:         10,
	PPSplitByBoneCount:         10,
	PPOptimizeCache:            11,
	PPOptimizeMeshes:           12,
	PPFindInstances:            13,
	PPFlattenHierarchy:         14,
	PPOptimizeGraph:            15,
	PPPreTransform:             16,
	PPGlobalScale:              17,
	PPFixUpAxis:                18,
	PPMakeLeftHanded:           19,
	PPRemoveComponent:          20,
	PPRemoveRedundantMaterials: 21,
	PPValidateMaterials:        22,
	PPEmbedTextures:            23,
	PPConvertToPBR:             24,
	PPLimitBoneWeights:         25,
	PPDebone:                   26,
	PPPopulateArmatureData:     27,
	PPGenBoundingBoxes:         28,
	PPValidateAnimations:       29,
	PPValidate:                 30,
	PPDecodeSamples:            31,
	PPFindInvalid:              32,
	PPResampleAudio:            33,
	PPMixdownAudio:             34,
	PPNormalizeAudio:           35,
	PPTrimAudio:                36,
	PPDecodePixels:             37,
	PPResizeImages:             38,
	PPGenerateMipmaps:          39,
	PPGenerateFontAtlas:        40,
	PPReportStats:              41,
}

func stepPriority(flag PPFlag) int {
	if order, ok := stepOrder[flag]; ok {
		return order
	}
	return math.MaxInt
}

func sortSteps(steps []Step) {
	sort.SliceStable(steps, func(i, j int) bool {
		oi := stepPriority(steps[i].Flag())
		oj := stepPriority(steps[j].Flag())
		return oi < oj
	})
}

// Registry holds an ordered set of post-processing steps.
type Registry struct {
	mu    sync.RWMutex
	steps []Step
}

// NewRegistry creates a step registry from declarative step registrations.
func NewRegistry(steps ...Step) *Registry {
	registry := &Registry{}
	registry.RegisterAll(steps...)
	return registry
}

// Register adds a processing step to the registry.
func (r *Registry) Register(step Step) {
	r.RegisterAll(step)
}

// RegisterAll adds multiple processing steps to the registry.
func (r *Registry) RegisterAll(steps ...Step) {
	if len(steps) == 0 {
		return
	}

	r.mu.Lock()
	combined := make([]Step, 0, len(r.steps)+len(steps))
	combined = append(combined, r.steps...)
	combined = append(combined, steps...)
	sortSteps(combined)
	r.steps = combined
	r.mu.Unlock()
}

// Steps returns the ordered steps currently registered.
func (r *Registry) Steps() []Step {
	r.mu.RLock()
	steps := make([]Step, len(r.steps))
	copy(steps, r.steps)
	r.mu.RUnlock()
	return steps
}

// Apply runs all flagged processing steps on the asset.
func (r *Registry) Apply(asset *ir.Asset, flags PPFlag, opts Options) error {
	r.mu.RLock()
	steps := r.steps
	r.mu.RUnlock()

	if asset != nil {
		asset.NormalizeGraph()
	}

	for _, step := range steps {
		if flags&step.Flag() != 0 {
			var err error
			asset, err = step.Apply(asset, opts)
			if err != nil {
				return err
			}
			if asset != nil {
				asset.NormalizeGraph()
			}
		}
	}
	return nil
}
