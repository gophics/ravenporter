package ir

// FormatID identifies an input asset format.
type FormatID string

// Supported import formats.
const (
	FormatUnknown FormatID = ""
	FormatGLTF    FormatID = "gltf"
	FormatGLB     FormatID = "glb"
	FormatFBX     FormatID = "fbx"
	FormatOBJ     FormatID = "obj"
	FormatDAE     FormatID = "dae"
	FormatSTL     FormatID = "stl"
	FormatPLY     FormatID = "ply"
	Format3MF     FormatID = "3mf"
	FormatBVH     FormatID = "bvh"
	Format3DS     FormatID = "3ds"
	FormatUSD     FormatID = "usd"
	FormatAlembic FormatID = "abc"

	// Image formats.
	FormatPNG  FormatID = "png"
	FormatJPEG FormatID = "jpeg"
	FormatBMP  FormatID = "bmp"
	FormatTGA  FormatID = "tga"
	FormatHDR  FormatID = "hdr"
	FormatWebP FormatID = "webp"
	FormatTIFF FormatID = "tiff"
	FormatEXR  FormatID = "exr"
	FormatDDS  FormatID = "dds"
	FormatKTX  FormatID = "ktx"
	FormatPSD  FormatID = "psd"

	// Audio formats.
	FormatWAV  FormatID = "wav"
	FormatOGG  FormatID = "ogg"
	FormatMP3  FormatID = "mp3"
	FormatFLAC FormatID = "flac"
	FormatAIFF FormatID = "aiff"
	FormatOpus FormatID = "opus"

	// Font formats.
	FormatTTF   FormatID = "ttf"
	FormatOTF   FormatID = "otf"
	FormatWOFF  FormatID = "woff"
	FormatWOFF2 FormatID = "woff2"
)

// ImageFormat identifies an image encoding.
type ImageFormat string

// Supported image formats.
const (
	ImagePNG  ImageFormat = "png"
	ImageJPEG ImageFormat = "jpeg"
	ImageWebP ImageFormat = "webp"
	ImageTGA  ImageFormat = "tga"
	ImageBMP  ImageFormat = "bmp"
	ImageDDS  ImageFormat = "dds"
	ImageKTX  ImageFormat = "ktx"
	ImageEXR  ImageFormat = "exr"
	ImageHDR  ImageFormat = "hdr"
	ImagePSD  ImageFormat = "psd"
	ImageTIFF ImageFormat = "tiff"
)

// ImageTopology identifies the logical texture shape carried by an image asset.
type ImageTopology string

// Supported image topology values.
const (
	ImageTopology2D        ImageTopology = "2D"
	ImageTopology3D        ImageTopology = "3D"
	ImageTopologyCube      ImageTopology = "Cube"
	ImageTopology2DArray   ImageTopology = "2DArray"
	ImageTopologyCubeArray ImageTopology = "CubeArray"
)

// AudioFormat identifies an audio encoding.
type AudioFormat string

// Supported audio formats.
const (
	AudioWAV  AudioFormat = "wav"
	AudioOGG  AudioFormat = "ogg"
	AudioMP3  AudioFormat = "mp3"
	AudioFLAC AudioFormat = "flac"
	AudioAIFF AudioFormat = "aiff"
	AudioOpus AudioFormat = "opus"
)

// FontFormat identifies a font file format.
type FontFormat string

// Supported font formats.
const (
	FontTTF    FontFormat = "ttf"
	FontOTF    FontFormat = "otf"
	FontWOFF   FontFormat = "woff"
	FontWOFF2  FontFormat = "woff2"
	FontBMFont FontFormat = "bmfont"
)

// Axis represents a coordinate axis.
type Axis int

// Axis constants.
const (
	YUp Axis = iota
	ZUp
)

// ColorSpace identifies a color space for images and textures.
type ColorSpace string

// Color space constants.
const (
	ColorSRGB   ColorSpace = "sRGB"
	ColorLinear ColorSpace = "linear"
)

// AlphaMode defines how material transparency is handled.
type AlphaMode int

// Alpha mode constants.
const (
	AlphaOpaque AlphaMode = iota
	AlphaMask
	AlphaBlend
)

// PrimitiveMode defines the mesh primitive type.
type PrimitiveMode int

// Primitive mode constants.
const (
	Triangles PrimitiveMode = iota
	TriangleStrip
	TriangleFan
	Lines
	LineStrip
	LineLoop
	Points
)

// Interpolation defines animation interpolation method.
type Interpolation int

// Interpolation constants.
const (
	InterpolationLinear Interpolation = iota
	InterpolationStep
	InterpolationCubicSpline
)

// TextureWrap defines texture wrapping behavior.
type TextureWrap int

// Texture wrap constants.
const (
	WrapRepeat TextureWrap = iota
	WrapClamp
	WrapMirror
)

// TextureFilter defines texture filtering mode.
type TextureFilter int

// Texture filter constants.
const (
	FilterNearest TextureFilter = iota
	FilterLinear
)

// ChannelTarget identifies which property an animation channel targets.
type ChannelTarget int

// Channel target constants.
const (
	TargetTranslation ChannelTarget = iota
	TargetRotation
	TargetScale
	TargetMorphWeights
	TargetPointer        // generic JSON-pointer-resolved target
	TargetMaterialColor  // baseColorFactor, emissiveFactor, etc.
	TargetMaterialScalar // metallicFactor, roughnessFactor, etc.
	TargetCameraFOV
	TargetLightColor
	TargetLightIntensity
)

// ChannelLayout identifies audio channel arrangement.
type ChannelLayout string

// Channel layout constants.
const (
	LayoutMono   ChannelLayout = "mono"
	LayoutStereo ChannelLayout = "stereo"
	Layout5_1    ChannelLayout = "5.1"
	Layout7_1    ChannelLayout = "7.1"
)

// Channels returns the channel count for a given layout.
func (l ChannelLayout) Channels() int {
	switch l {
	case LayoutMono:
		return 1
	case LayoutStereo:
		return channelsStereo
	case Layout5_1:
		return channels5_1
	case Layout7_1:
		return channels7_1
	default:
		return 0
	}
}

// Channel count constants for ChannelLayout.Channels().
const (
	channelsStereo = 2
	channels5_1    = 6
	channels7_1    = 8
)

// ChannelCount identifies image channel configuration.
type ChannelCount int

// Image channel count constants.
const (
	ChannelGray      ChannelCount = 1
	ChannelGrayAlpha ChannelCount = 2
	ChannelRGB       ChannelCount = 3
	ChannelRGBA      ChannelCount = 4
)

// BitDepth identifies sample precision for audio and image data.
type BitDepth int

// Bit depth constants.
const (
	BitDepth8  BitDepth = 8
	BitDepth16 BitDepth = 16
	BitDepth24 BitDepth = 24
	BitDepth32 BitDepth = 32
)

// NoIndex is the sentinel for unattached optional indices.
const NoIndex = -1

// Transform holds a node's local transformation.
// If Matrix is non-zero it takes precedence over TRS.
type Transform struct {
	Translation [3]float32
	Rotation    [4]float32 // Quaternion (x, y, z, w)
	Scale       [3]float32
	Matrix      [16]float32 // Column-major 4x4, zero = use TRS
}

// IdentityTransform returns a default identity transform.
func IdentityTransform() Transform {
	return Transform{Scale: [3]float32{1, 1, 1}, Rotation: [4]float32{0, 0, 0, 1}}
}

// VertexRemap maps new vertex indices back to original vertex indices.
type VertexRemap []int

// MobilityState defines whether an actor can move at runtime.
type MobilityState int

// Mobility constants.
const (
	MobilityStatic MobilityState = iota
	MobilityStationary
	MobilityMovable
)
