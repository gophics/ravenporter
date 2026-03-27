package core

import (
	"io"
	"log/slog"

	"github.com/gophics/ravenporter/ir"
)

// PPFlag is a bitmask for post-processing steps.
type PPFlag uint64

// Post-processing flag constants.
const (
	PPTriangulate PPFlag = 1 << iota
	PPGenNormals
	PPGenSmoothNormals
	PPCalcTangentSpace
	PPJoinIdenticalVertices
	PPOptimizeCache
	PPRemoveDegenerates
	PPSplitLargeMeshes
	PPSortByPtype
	PPFixWinding
	PPFixInfacingNormals
	PPGenUVCoords
	PPTransformUVCoords
	PPFlipUVs
	PPFlipWindingOrder
	PPFindInstances
	PPOptimizeMeshes
	PPFlattenHierarchy
	PPOptimizeGraph
	PPPreTransform
	PPGlobalScale
	PPFixUpAxis
	PPMakeLeftHanded
	PPRemoveComponent
	PPRemoveRedundantMaterials
	PPValidateMaterials
	PPEmbedTextures
	PPConvertToPBR
	PPLimitBoneWeights
	PPDebone
	PPValidateAnimations
	PPValidate
	PPFindInvalid
	PPReportStats
	PPResampleAudio
	PPMixdownAudio
	PPGenBoundingBoxes
	PPForceGenNormals
	PPDropNormals
	PPSplitByBoneCount
	PPPopulateArmatureData
	PPGenerateMipmaps
	PPResizeImages
	PPGenerateFontAtlas
	PPNormalizeAudio
	PPTrimAudio
	PPDecodePixels
	PPDecodeSamples
)

// Preset combinations.
const (
	PresetFast = PPTriangulate | PPGenNormals | PPJoinIdenticalVertices

	PresetQuality = PPTriangulate | PPGenSmoothNormals |
		PPCalcTangentSpace | PPJoinIdenticalVertices |
		PPOptimizeCache | PPRemoveDegenerates |
		PPValidate | PPRemoveRedundantMaterials |
		PPGenBoundingBoxes | PPDecodePixels

	PresetMaxQuality = PresetQuality | PPFindInstances | PPOptimizeMeshes |
		PPFixInfacingNormals | PPTransformUVCoords
)

// DegenerateMode controls how degenerate triangles are handled.
type DegenerateMode int

// DegenerateMode constants.
const (
	DegenerateModeRemove  DegenerateMode = iota // remove (default)
	DegenerateModeConvert                       // convert to lines/points
)

// Step is an individual post-processing operation.
type Step interface {
	Name() string
	Flag() PPFlag
	Apply(asset *ir.Asset, opts Options) (*ir.Asset, error)
}

// Options configures post-processing behavior.
type Options struct {
	Logger             *slog.Logger
	SmoothNormalAngle  float64
	MaxBoneWeights     int
	MaxVerticesPerMesh int
	MaxBonesPerMesh    int
	MaxTextureSize     int
	AtlasFontSize      int
	GlobalScale        float64
	TargetUpAxis       ir.Axis
	RemoveFlags        ComponentFlag
	AssetDir           string
	AssetFS            interface {
		Open(name string) (io.ReadCloser, error)
	}
	TargetSampleRate int
	TargetChannels   int
	DegenerateMode   DegenerateMode
	DeboneThreshold  float32
}

// ComponentFlag identifies vertex attributes to remove.
type ComponentFlag uint32

// Component flags for PPRemoveComponent.
const (
	CompNormals ComponentFlag = 1 << iota
	CompTangents
	CompTexCoord0
	CompTexCoord1
	CompColors0
	CompJoints
	CompWeights
)
