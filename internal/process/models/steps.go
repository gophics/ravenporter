package models

import "github.com/gophics/ravenporter/internal/process/core"

// Steps returns the built-in model post-processing steps.
func Steps() []core.Step {
	return []core.Step{
		&populateArmatureDataStep{},
		&limitBoneWeightsStep{},
		&genBoundingBoxesStep{},
		&optimizeCacheStep{},
		&fixUpAxisStep{},
		&makeLeftHandedStep{},
		&deboneStep{},
		&findInstancesStep{},
		&removeDegeneratesStep{},
		&embedTexturesStep{},
		&preTransformStep{},
		&flattenHierarchyStep{},
		&flipUVsStep{},
		&fixInfacingNormalsStep{},
		&removeRedundantMaterialsStep{},
		&removeComponentStep{},
		&findInvalidStep{},
		&optimizeMeshesStep{},
		&genNormalsStep{},
		&genSmoothNormalsStep{},
		&forceGenNormalsStep{},
		&dropNormalsStep{},
		&optimizeGraphStep{},
		&convertToPBRStep{},
		&reportStatsStep{},
		&sortPTypeStep{},
		&splitLargeMeshesStep{},
		&splitByBoneCountStep{},
		&calcTangentSpaceStep{},
		&globalScaleStep{},
		&triangulateStep{},
		&genUVCoordsStep{},
		&transformUVCoordsStep{},
		&validateStep{},
		&validateMaterialsStep{},
		&weldVerticesStep{},
		&flipWindingOrderStep{},
		&fixWindingCCWStep{},
	}
}
