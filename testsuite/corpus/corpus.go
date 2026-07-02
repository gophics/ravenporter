package corpus

// Audio Files.
const (
	AudioAIFF      = "audio/test.aiff"
	AudioAIFF8bit  = "audio/aiff_8bit.aif"
	AudioAIFF24bit = "audio/aiff_24bit.aif"
	AudioFLAC      = "audio/blocksize4096.flac"
	AudioFLACSmall = "audio/flac_small.flac"
	AudioMP3       = "audio/outFoxing.mp3"
	AudioMP3Small  = "audio/mp3_small.mp3"
	AudioOGG       = "audio/Example.ogg"
	AudioOGGSmall  = "audio/ogg_small.ogg"
	AudioOpus      = "audio/test.opus"
	AudioWAV       = "audio/outFoxing.wav"
	AudioWAVSmall  = "audio/wav_small.wav"
)

// Font Files.
const (
	FontOTFMinimal   = "fonts/minimal.otf"
	FontOpenSans     = "fonts/OpenSans.ttf"
	FontRoboto       = "fonts/Roboto.ttf"
	FontWOFFMinimal  = "fonts/minimal.woff"
	FontWOFF2Minimal = "fonts/minimal.woff2"
)

// Image Files.
const (
	ImageBMPRed      = "images/red4x4.bmp"
	ImageBMPPhoto    = "images/bmp_photo.bmp"
	ImageDDSMinimal  = "images/minimal.dds"
	ImageEXRMinimal  = "images/minimal.exr"
	ImageEXRTest     = "images/exr_test.exr"
	ImageHDRTest     = "images/test.hdr"
	ImageHDROutdoor  = "images/hdr_outdoor.hdr"
	ImageJPEGRed     = "images/red4x4.jpg"
	ImageJPEGPhoto   = "images/jpeg_photo.jpg"
	ImageKTXMinimal  = "images/minimal.ktx"
	ImagePNGRed      = "images/red4x4.png"
	ImagePNGPhoto    = "images/png_photo.png"
	ImagePSDMinimal  = "images/minimal.psd"
	ImageTGABlue     = "images/blue_2x2.tga"
	ImageTGA32bit    = "images/tga_test.tga"
	ImageTIFFTest    = "images/test.tiff"
	ImageTIFFGray    = "images/tiff_gray.tif"
	ImageWebPGallery = "images/gallery1.webp"
	ImageWebPSmall   = "images/webp_small.webp"
)

// Model Files.
const (
	Model3DSCube              = "3DS/fels.3ds"
	Model3DSVariant           = "3DS/test1.3ds"
	Model3MFBox               = "3MF/box.3mf"
	ModelABCCube              = "ABC/cube.abc"
	ModelBVHMoCap             = "BVH/01_01.bvh"
	ModelDAEDuck              = "Collada/duck.dae"
	ModelDAEAnimated          = "Collada/anim_cube.dae"
	ModelFBXBox               = "FBX/box.fbx"
	ModelFBXPhongCube         = "FBX/phong_cube.fbx"
	ModelFBXASCII             = "FBX/cubes_with_names.fbx"
	ModelGLTF2BoxTextured     = "glTF2/BoxTextured.glb"
	ModelGLTF2CylinderEngine  = "glTF2/2CylinderEngine.glb"
	ModelGLTF2Avocado         = "glTF2/Avocado.glb"
	ModelGLTF2KhronosBoxDraco = "third_party/khronos/Box/glTF-Draco/Box.gltf"
	ModelOBJCube              = "OBJ/cube.obj"
	ModelOBJBunny             = "OBJ/bunny.obj"
	ModelPLYCubeASCII         = "PLY/cube.ply"
	ModelPLYCubeBinary        = "PLY/cube_binary.ply"
	ModelSTLASCII             = "STL/Spider_ascii.stl"
	ModelSTLBinary            = "STL/Spider_binary.stl"
	ModelUSDAComprehensive    = "USDA/comprehensive.usda"
	ModelUSDCComprehensive    = "USDA/comprehensive.usdc"
)

// Rejection Files.
const (
	RejectAudioWAV  = "rejection/audio/bad_header.wav"
	RejectAudioFLAC = "rejection/audio/broken_flac_header.flac"
	RejectAudioOPUS = "rejection/audio/bad_opus_gain.opus"
	RejectAudioMP3  = "rejection/audio/bad.mp3"
	RejectAudioOGG  = "rejection/audio/bad.ogg"
	RejectAudioAIFF = "rejection/audio/bad.aiff"

	RejectFontTTF   = "rejection/fonts/bad_header.ttf"
	RejectFontOTF   = "rejection/fonts/bad.otf"
	RejectFontWOFF  = "rejection/fonts/bad.woff"
	RejectFontWOFF2 = "rejection/fonts/bad.woff2"

	RejectImageDDS  = "rejection/images/bad_header.dds"
	RejectImageTGA  = "rejection/images/invalid_rle.tga"
	RejectImageKTX  = "rejection/images/bad_mip_count.ktx"
	RejectImageBMP  = "rejection/images/bad.bmp"
	RejectImageEXR  = "rejection/images/bad.exr"
	RejectImageHDR  = "rejection/images/bad.hdr"
	RejectImageJPEG = "rejection/images/bad.jpeg"
	RejectImagePNG  = "rejection/images/bad.png"
	RejectImagePSD  = "rejection/images/bad.psd"
	RejectImageTIFF = "rejection/images/bad.tiff"
	RejectImageWEBP = "rejection/images/bad.webp"

	RejectModelGLTF = "rejection/models/bad_mesh_index.gltf"
	RejectModelOBJ  = "rejection/models/bad_geometry.obj"
	RejectModelFBX  = "rejection/models/bad_header.fbx"
	RejectModelDAE  = "rejection/models/nan_floats.dae"
	RejectModelABC  = "rejection/models/bad.abc"
	RejectModelBVH  = "rejection/models/bad.bvh"
	RejectModelPLY  = "rejection/models/bad.ply"
	RejectModelSTL  = "rejection/models/bad.stl"
	RejectModelTDS  = "rejection/models/bad.tds"
	RejectModel3MF  = "rejection/models/bad.3mf"
	RejectModelUSDA = "rejection/models/bad.usda"
)

// Isolation Files.
const (
	IsoAudioFeaturesWAV  = "isolation/audio/features.wav"
	IsoAudioFeaturesFLAC = "isolation/audio/features.flac"

	IsoImageNPOT = "isolation/images/npot.tga"
	IsoImageRLE  = "isolation/images/rle.tga"

	IsoModelDracoGLTF   = "isolation/models/gltf_draco_triangle.gltf"
	IsoModelPBRGLTF     = "isolation/models/gltf_pbr_exhaustive.gltf"
	IsoModelMeshoptGLTF = "isolation/models/gltf_meshopt_indices.gltf"
	IsoModelBadGeoOBJ   = "isolation/models/nan_positions.obj"
)
