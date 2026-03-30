// Package process provides RavenPorter's built-in post-processing catalog.
package process

import (
	"github.com/gophics/ravenporter/internal/process/audio"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/internal/process/font"
	"github.com/gophics/ravenporter/internal/process/images"
	"github.com/gophics/ravenporter/internal/process/models"
	"github.com/gophics/ravenporter/ir"
)

var defaultRegistry = NewRegistry(BuiltInSteps()...)

// PPFlag exposes the core bitflag orchestrator type.
type PPFlag = core.PPFlag
type Options = core.Options
type Step = core.Step
type DegenerateMode = core.DegenerateMode
type ComponentFlag = core.ComponentFlag
type Registry = core.Registry

// Re-export constants for full backward compatibility.
const (
	DegenerateModeRemove  = core.DegenerateModeRemove
	DegenerateModeConvert = core.DegenerateModeConvert

	PresetFast       = core.PresetFast
	PresetQuality    = core.PresetQuality
	PresetMaxQuality = core.PresetMaxQuality

	CompNormals   = core.CompNormals
	CompTangents  = core.CompTangents
	CompTexCoord0 = core.CompTexCoord0
	CompTexCoord1 = core.CompTexCoord1
	CompColors0   = core.CompColors0
	CompJoints    = core.CompJoints
	CompWeights   = core.CompWeights

	PPTriangulate              = core.PPTriangulate
	PPGenNormals               = core.PPGenNormals
	PPGenSmoothNormals         = core.PPGenSmoothNormals
	PPCalcTangentSpace         = core.PPCalcTangentSpace
	PPJoinIdenticalVertices    = core.PPJoinIdenticalVertices
	PPOptimizeCache            = core.PPOptimizeCache
	PPRemoveDegenerates        = core.PPRemoveDegenerates
	PPSplitLargeMeshes         = core.PPSplitLargeMeshes
	PPSortByPtype              = core.PPSortByPtype
	PPFixWinding               = core.PPFixWinding
	PPFixInfacingNormals       = core.PPFixInfacingNormals
	PPGenUVCoords              = core.PPGenUVCoords
	PPTransformUVCoords        = core.PPTransformUVCoords
	PPFlipUVs                  = core.PPFlipUVs
	PPFlipWindingOrder         = core.PPFlipWindingOrder
	PPFindInstances            = core.PPFindInstances
	PPOptimizeMeshes           = core.PPOptimizeMeshes
	PPFlattenHierarchy         = core.PPFlattenHierarchy
	PPOptimizeGraph            = core.PPOptimizeGraph
	PPPreTransform             = core.PPPreTransform
	PPGlobalScale              = core.PPGlobalScale
	PPFixUpAxis                = core.PPFixUpAxis
	PPMakeLeftHanded           = core.PPMakeLeftHanded
	PPRemoveComponent          = core.PPRemoveComponent
	PPRemoveRedundantMaterials = core.PPRemoveRedundantMaterials
	PPValidateMaterials        = core.PPValidateMaterials
	PPEmbedTextures            = core.PPEmbedTextures
	PPConvertToPBR             = core.PPConvertToPBR
	PPLimitBoneWeights         = core.PPLimitBoneWeights
	PPDebone                   = core.PPDebone
	PPValidateAnimations       = core.PPValidateAnimations
	PPValidate                 = core.PPValidate
	PPFindInvalid              = core.PPFindInvalid
	PPReportStats              = core.PPReportStats
	PPResampleAudio            = core.PPResampleAudio
	PPMixdownAudio             = core.PPMixdownAudio
	PPGenBoundingBoxes         = core.PPGenBoundingBoxes
	PPForceGenNormals          = core.PPForceGenNormals
	PPDropNormals              = core.PPDropNormals
	PPSplitByBoneCount         = core.PPSplitByBoneCount
	PPPopulateArmatureData     = core.PPPopulateArmatureData
	PPGenerateMipmaps          = core.PPGenerateMipmaps
	PPResizeImages             = core.PPResizeImages
	PPGenerateFontAtlas        = core.PPGenerateFontAtlas
	PPNormalizeAudio           = core.PPNormalizeAudio
	PPTrimAudio                = core.PPTrimAudio
	PPDecodePixels             = core.PPDecodePixels
	PPDecodeSamples            = core.PPDecodeSamples
)

// BuiltInSteps returns the built-in processing step catalog.
func BuiltInSteps() []Step {
	var steps []Step
	steps = append(steps, audio.Steps()...)
	steps = append(steps, font.Steps()...)
	steps = append(steps, images.Steps()...)
	steps = append(steps, models.Steps()...)
	return steps
}

// NewRegistry returns a fresh registry containing the provided processing steps.
func NewRegistry(steps ...Step) *Registry {
	return core.NewRegistry(steps...)
}

// Apply handles orchestration logic by dispatching execution to the built-in registry.
func Apply(asset *ir.Asset, flags PPFlag, opts Options) error {
	return defaultRegistry.Apply(asset, flags, opts)
}
