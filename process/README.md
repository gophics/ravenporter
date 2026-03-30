# Process Package

The `process` package acts as the centralized orchestrator and post-processing pipeline for RavenPorter. It natively applies sequence-dependent mutations to the in-memory intermediate representation (`ir.Scene`) via a standardized bitflag orchestration registry.

## Supported Features

The following 46 processing features are fully supported and implemented natively in zero-allocation Go routines.

| Feature | Flag | Supported |
|---|---|---|
| Triangulate | `PPTriangulate` | ✅ |
| Generate Normals | `PPGenNormals` | ✅ |
| Generate Smooth Normals | `PPGenSmoothNormals` | ✅ |
| Calculate Tangent Space | `PPCalcTangentSpace` | ✅ |
| Join Identical Vertices | `PPJoinIdenticalVertices` | ✅ |
| Optimize Cache | `PPOptimizeCache` | ✅ |
| Remove Degenerates | `PPRemoveDegenerates` | ✅ |
| Split Large Meshes | `PPSplitLargeMeshes` | ✅ |
| Sort By Primitive Type | `PPSortByPtype` | ✅ |
| Fix Winding | `PPFixWinding` | ✅ |
| Fix Infacing Normals | `PPFixInfacingNormals` | ✅ |
| Generate UV Coordinates | `PPGenUVCoords` | ✅ |
| Transform UV Coordinates | `PPTransformUVCoords` | ✅ |
| Flip UVs | `PPFlipUVs` | ✅ |
| Flip Winding Order | `PPFlipWindingOrder` | ✅ |
| Find Instances | `PPFindInstances` | ✅ |
| Optimize Meshes | `PPOptimizeMeshes` | ✅ |
| Flatten Hierarchy | `PPFlattenHierarchy` | ✅ |
| Optimize Graph | `PPOptimizeGraph` | ✅ |
| Pre-Transform | `PPPreTransform` | ✅ |
| Global Scale | `PPGlobalScale` | ✅ |
| Fix Up-Axis | `PPFixUpAxis` | ✅ |
| Make Left-Handed | `PPMakeLeftHanded` | ✅ |
| Remove Component | `PPRemoveComponent` | ✅ |
| Remove Redundant Materials | `PPRemoveRedundantMaterials` | ✅ |
| Validate Materials | `PPValidateMaterials` | ✅ |
| Embed Textures | `PPEmbedTextures` | ✅ |
| Convert To PBR | `PPConvertToPBR` | ✅ |
| Limit Bone Weights | `PPLimitBoneWeights` | ✅ |
| Debone | `PPDebone` | ✅ |
| Validate Animations | `PPValidateAnimations` | ✅ |
| Validate | `PPValidate` | ✅ |
| Find Invalid | `PPFindInvalid` | ✅ |
| Report Stats | `PPReportStats` | ✅ |
| Resample Audio | `PPResampleAudio` | ✅ |
| Mixdown Audio | `PPMixdownAudio` | ✅ |
| Generate Bounding Boxes | `PPGenBoundingBoxes` | ✅ |
| Force Generate Normals | `PPForceGenNormals` | ✅ |
| Drop Normals | `PPDropNormals` | ✅ |
| Split By Bone Count | `PPSplitByBoneCount` | ✅ |
| Populate Armature Data | `PPPopulateArmatureData` | ✅ |
| Generate Mipmaps | `PPGenerateMipmaps` | ✅ |
| Resize Images | `PPResizeImages` | ✅ |
| Generate Font Atlas | `PPGenerateFontAtlas` | ✅ |
| Normalize Audio | `PPNormalizeAudio` | ✅ |
| Trim Audio | `PPTrimAudio` | ✅ |
