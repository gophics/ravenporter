package pipeline

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/ir"
	"github.com/gophics/ravenporter/process"
)

const (
	// ProfileVersion is the TOML schema version for import profiles.
	ProfileVersion = 1

	// BuiltInPresetFast is the generic built-in preset optimized for speed.
	BuiltInPresetFast = "fast"

	// BuiltInPresetQuality is the generic built-in preset tuned for balanced quality.
	BuiltInPresetQuality = "quality"

	// BuiltInPresetMaxQuality is the generic built-in preset tuned for the highest built-in quality.
	BuiltInPresetMaxQuality = "max-quality"
)

const profileFilePerm = 0o600

var builtInPresets = map[string]process.PPFlag{ //nolint:gochecknoglobals // static preset table
	BuiltInPresetFast:       process.PresetFast,
	BuiltInPresetQuality:    process.PresetQuality,
	BuiltInPresetMaxQuality: process.PresetMaxQuality,
}

type processStepDef struct {
	name string
	flag process.PPFlag
}

type componentFlagDef struct {
	name string
	flag process.ComponentFlag
}

const (
	componentNameNormals   = "normals"
	componentNameTangents  = "tangents"
	componentNameTexCoord0 = "texcoord0"
	componentNameTexCoord1 = "texcoord1"
	componentNameColors0   = "colors0"
	componentNameJoints    = "joints"
	componentNameWeights   = "weights"
)

var processStepDefs = []processStepDef{ //nolint:gochecknoglobals // stable serialization order
	{name: "triangulate", flag: process.PPTriangulate},
	{name: "gen-normals", flag: process.PPGenNormals},
	{name: "gen-smooth-normals", flag: process.PPGenSmoothNormals},
	{name: "calc-tangent-space", flag: process.PPCalcTangentSpace},
	{name: "join-identical-vertices", flag: process.PPJoinIdenticalVertices},
	{name: "optimize-cache", flag: process.PPOptimizeCache},
	{name: "remove-degenerates", flag: process.PPRemoveDegenerates},
	{name: "split-large-meshes", flag: process.PPSplitLargeMeshes},
	{name: "sort-by-ptype", flag: process.PPSortByPtype},
	{name: "fix-winding", flag: process.PPFixWinding},
	{name: "fix-infacing-normals", flag: process.PPFixInfacingNormals},
	{name: "gen-uv-coords", flag: process.PPGenUVCoords},
	{name: "transform-uv-coords", flag: process.PPTransformUVCoords},
	{name: "flip-uvs", flag: process.PPFlipUVs},
	{name: "flip-winding-order", flag: process.PPFlipWindingOrder},
	{name: "find-instances", flag: process.PPFindInstances},
	{name: "optimize-meshes", flag: process.PPOptimizeMeshes},
	{name: "flatten-hierarchy", flag: process.PPFlattenHierarchy},
	{name: "optimize-graph", flag: process.PPOptimizeGraph},
	{name: "pre-transform", flag: process.PPPreTransform},
	{name: "global-scale", flag: process.PPGlobalScale},
	{name: "fix-up-axis", flag: process.PPFixUpAxis},
	{name: "make-left-handed", flag: process.PPMakeLeftHanded},
	{name: "remove-component", flag: process.PPRemoveComponent},
	{name: "remove-redundant-materials", flag: process.PPRemoveRedundantMaterials},
	{name: "validate-materials", flag: process.PPValidateMaterials},
	{name: "embed-textures", flag: process.PPEmbedTextures},
	{name: "convert-to-pbr", flag: process.PPConvertToPBR},
	{name: "limit-bone-weights", flag: process.PPLimitBoneWeights},
	{name: "debone", flag: process.PPDebone},
	{name: "validate-animations", flag: process.PPValidateAnimations},
	{name: "validate", flag: process.PPValidate},
	{name: "find-invalid", flag: process.PPFindInvalid},
	{name: "report-stats", flag: process.PPReportStats},
	{name: "resample-audio", flag: process.PPResampleAudio},
	{name: "mixdown-audio", flag: process.PPMixdownAudio},
	{name: "gen-bounding-boxes", flag: process.PPGenBoundingBoxes},
	{name: "force-gen-normals", flag: process.PPForceGenNormals},
	{name: "drop-normals", flag: process.PPDropNormals},
	{name: "split-by-bone-count", flag: process.PPSplitByBoneCount},
	{name: "populate-armature-data", flag: process.PPPopulateArmatureData},
	{name: "generate-mipmaps", flag: process.PPGenerateMipmaps},
	{name: "resize-images", flag: process.PPResizeImages},
	{name: "generate-font-atlas", flag: process.PPGenerateFontAtlas},
	{name: "normalize-audio", flag: process.PPNormalizeAudio},
	{name: "trim-audio", flag: process.PPTrimAudio},
	{name: "decode-pixels", flag: process.PPDecodePixels},
	{name: "decode-samples", flag: process.PPDecodeSamples},
}

var componentFlagDefs = []componentFlagDef{ //nolint:gochecknoglobals // stable serialization order
	{name: componentNameNormals, flag: process.CompNormals},
	{name: componentNameTangents, flag: process.CompTangents},
	{name: componentNameTexCoord0, flag: process.CompTexCoord0},
	{name: componentNameTexCoord1, flag: process.CompTexCoord1},
	{name: componentNameColors0, flag: process.CompColors0},
	{name: componentNameJoints, flag: process.CompJoints},
	{name: componentNameWeights, flag: process.CompWeights},
}

// Profile is the public TOML-serializable import configuration schema.
type Profile struct {
	Version int            `json:"version,omitempty"`
	Preset  string         `json:"preset,omitempty"`
	Decode  DecodeProfile  `json:"decode,omitempty"`
	Process ProcessProfile `json:"process,omitempty"`
}

// DecodeProfile stores serializable decode safeguards.
type DecodeProfile struct {
	MaxFileSize     *int64 `json:"max_file_size,omitempty"`
	MaxVertices     *int   `json:"max_vertices,omitempty"`
	MaxImagePixels  *int   `json:"max_image_pixels,omitempty"`
	MaxAudioSamples *int   `json:"max_audio_samples,omitempty"`
}

// ProcessProfile stores serializable process overrides layered on top of a preset.
type ProcessProfile struct {
	EnabledSteps       []string `json:"enable_steps,omitempty"`
	DisabledSteps      []string `json:"disable_steps,omitempty"`
	SmoothNormalAngle  *float64 `json:"smooth_normal_angle,omitempty"`
	MaxBoneWeights     *int     `json:"max_bone_weights,omitempty"`
	MaxVerticesPerMesh *int     `json:"max_vertices_per_mesh,omitempty"`
	MaxBonesPerMesh    *int     `json:"max_bones_per_mesh,omitempty"`
	MaxTextureSize     *int     `json:"max_texture_size,omitempty"`
	AtlasFontSize      *int     `json:"atlas_font_size,omitempty"`
	GlobalScale        *float64 `json:"global_scale,omitempty"`
	TargetUpAxis       *string  `json:"target_up_axis,omitempty"`
	RemoveFlags        []string `json:"remove_flags,omitempty"`
	TargetSampleRate   *int     `json:"target_sample_rate,omitempty"`
	TargetChannels     *int     `json:"target_channels,omitempty"`
	DegenerateMode     *string  `json:"degenerate_mode,omitempty"`
	DeboneThreshold    *float64 `json:"debone_threshold,omitempty"`
}

// BuiltInPresetNames returns the supported generic preset names.
func BuiltInPresetNames() []string {
	return []string{BuiltInPresetFast, BuiltInPresetQuality, BuiltInPresetMaxQuality}
}

// ResolveBuiltInPreset maps a generic preset name to its process flag mask.
func ResolveBuiltInPreset(name string) (process.PPFlag, error) {
	canonical := canonicalPresetName(name)
	flag, ok := builtInPresets[canonical]
	if !ok {
		return 0, fmt.Errorf("unknown preset %q (supported: %s)", name, strings.Join(BuiltInPresetNames(), ", "))
	}
	return flag, nil
}

// HasContent reports whether the profile contains any serializable behavior.
func (p Profile) HasContent() bool {
	return p.Version != 0 ||
		p.Preset != "" ||
		decodeProfileHasContent(p.Decode) ||
		processProfileHasContent(p.Process)
}

// MergeProfiles overlays the override profile onto the base profile.
func MergeProfiles(base, override Profile) Profile {
	merged := base
	if merged.Version == 0 {
		merged.Version = ProfileVersion
	}
	if override.Version != 0 {
		merged.Version = override.Version
	}
	if override.Preset != "" {
		merged.Preset = canonicalPresetName(override.Preset)
	}
	if override.Decode.MaxFileSize != nil {
		merged.Decode.MaxFileSize = cloneInt64Ptr(override.Decode.MaxFileSize)
	}
	if override.Decode.MaxVertices != nil {
		merged.Decode.MaxVertices = cloneIntPtr(override.Decode.MaxVertices)
	}
	if override.Decode.MaxImagePixels != nil {
		merged.Decode.MaxImagePixels = cloneIntPtr(override.Decode.MaxImagePixels)
	}
	if override.Decode.MaxAudioSamples != nil {
		merged.Decode.MaxAudioSamples = cloneIntPtr(override.Decode.MaxAudioSamples)
	}
	if len(override.Process.EnabledSteps) > 0 {
		merged.Process.EnabledSteps = mergeStringSlices(merged.Process.EnabledSteps, override.Process.EnabledSteps)
	}
	if len(override.Process.DisabledSteps) > 0 {
		merged.Process.DisabledSteps = mergeStringSlices(merged.Process.DisabledSteps, override.Process.DisabledSteps)
	}
	if override.Process.SmoothNormalAngle != nil {
		merged.Process.SmoothNormalAngle = cloneFloat64Ptr(override.Process.SmoothNormalAngle)
	}
	if override.Process.MaxBoneWeights != nil {
		merged.Process.MaxBoneWeights = cloneIntPtr(override.Process.MaxBoneWeights)
	}
	if override.Process.MaxVerticesPerMesh != nil {
		merged.Process.MaxVerticesPerMesh = cloneIntPtr(override.Process.MaxVerticesPerMesh)
	}
	if override.Process.MaxBonesPerMesh != nil {
		merged.Process.MaxBonesPerMesh = cloneIntPtr(override.Process.MaxBonesPerMesh)
	}
	if override.Process.MaxTextureSize != nil {
		merged.Process.MaxTextureSize = cloneIntPtr(override.Process.MaxTextureSize)
	}
	if override.Process.AtlasFontSize != nil {
		merged.Process.AtlasFontSize = cloneIntPtr(override.Process.AtlasFontSize)
	}
	if override.Process.GlobalScale != nil {
		merged.Process.GlobalScale = cloneFloat64Ptr(override.Process.GlobalScale)
	}
	if override.Process.TargetUpAxis != nil {
		axis := canonicalAxisName(*override.Process.TargetUpAxis)
		merged.Process.TargetUpAxis = &axis
	}
	if len(override.Process.RemoveFlags) > 0 {
		merged.Process.RemoveFlags = mergeComponentFlagNames(merged.Process.RemoveFlags, override.Process.RemoveFlags)
	}
	if override.Process.TargetSampleRate != nil {
		merged.Process.TargetSampleRate = cloneIntPtr(override.Process.TargetSampleRate)
	}
	if override.Process.TargetChannels != nil {
		merged.Process.TargetChannels = cloneIntPtr(override.Process.TargetChannels)
	}
	if override.Process.DegenerateMode != nil {
		mode := canonicalDegenerateModeName(*override.Process.DegenerateMode)
		merged.Process.DegenerateMode = &mode
	}
	if override.Process.DeboneThreshold != nil {
		merged.Process.DeboneThreshold = cloneFloat64Ptr(override.Process.DeboneThreshold)
	}
	return merged
}

func applyProfile(opts *config, profile Profile) error {
	if err := validateProfileVersion(profile.Version); err != nil {
		return err
	}
	if err := applyPresetOverride(opts, profile.Preset); err != nil {
		return err
	}
	if err := applyStepOverrides(opts, profile.Process); err != nil {
		return err
	}

	applyDecodeProfile(opts, profile.Decode)
	if err := applySceneProcessProfile(opts, profile.Process); err != nil {
		return err
	}
	if err := applyGeometryProcessProfile(opts, profile.Process); err != nil {
		return err
	}
	applyImageProcessProfile(opts, profile.Process)
	applyAudioProcessProfile(opts, profile.Process)
	return nil
}

func profileFromConfig(opts config) Profile {
	profile := Profile{Version: ProfileVersion}
	if opts.Preset != "" {
		profile.Preset = canonicalPresetName(opts.Preset)
	}
	if opts.DecodeOpts.MaxFileSize != 0 {
		profile.Decode.MaxFileSize = cloneInt64Ptr(&opts.DecodeOpts.MaxFileSize)
	}
	if opts.DecodeOpts.MaxVertices != 0 {
		profile.Decode.MaxVertices = cloneIntPtr(&opts.DecodeOpts.MaxVertices)
	}
	if opts.DecodeOpts.MaxImagePixels != 0 {
		profile.Decode.MaxImagePixels = cloneIntPtr(&opts.DecodeOpts.MaxImagePixels)
	}
	if opts.DecodeOpts.MaxAudioSamples != 0 {
		profile.Decode.MaxAudioSamples = cloneIntPtr(&opts.DecodeOpts.MaxAudioSamples)
	}

	profile.Process.EnabledSteps = enabledStepsFromOptions(opts)
	if opts.ProcessOpts.SmoothNormalAngle != 0 {
		profile.Process.SmoothNormalAngle = cloneFloat64Ptr(&opts.ProcessOpts.SmoothNormalAngle)
	}
	if opts.ProcessOpts.MaxBoneWeights != 0 {
		profile.Process.MaxBoneWeights = cloneIntPtr(&opts.ProcessOpts.MaxBoneWeights)
	}
	if opts.ProcessOpts.MaxVerticesPerMesh != 0 {
		profile.Process.MaxVerticesPerMesh = cloneIntPtr(&opts.ProcessOpts.MaxVerticesPerMesh)
	}
	if opts.ProcessOpts.MaxBonesPerMesh != 0 {
		profile.Process.MaxBonesPerMesh = cloneIntPtr(&opts.ProcessOpts.MaxBonesPerMesh)
	}
	if opts.ProcessOpts.MaxTextureSize != 0 {
		profile.Process.MaxTextureSize = cloneIntPtr(&opts.ProcessOpts.MaxTextureSize)
	}
	if opts.ProcessOpts.AtlasFontSize != 0 {
		profile.Process.AtlasFontSize = cloneIntPtr(&opts.ProcessOpts.AtlasFontSize)
	}
	if opts.ProcessFlags&process.PPGlobalScale != 0 {
		profile.Process.GlobalScale = cloneFloat64Ptr(&opts.ProcessOpts.GlobalScale)
	}
	if opts.ProcessFlags&process.PPFixUpAxis != 0 {
		axis := axisName(opts.ProcessOpts.TargetUpAxis)
		profile.Process.TargetUpAxis = &axis
	}
	if opts.ProcessFlags&process.PPRemoveComponent != 0 && opts.ProcessOpts.RemoveFlags != 0 {
		profile.Process.RemoveFlags = componentFlagNames(opts.ProcessOpts.RemoveFlags)
	}
	if opts.ProcessOpts.TargetSampleRate != 0 {
		profile.Process.TargetSampleRate = cloneIntPtr(&opts.ProcessOpts.TargetSampleRate)
	}
	if opts.ProcessOpts.TargetChannels != 0 {
		profile.Process.TargetChannels = cloneIntPtr(&opts.ProcessOpts.TargetChannels)
	}
	if opts.ProcessFlags&process.PPRemoveDegenerates != 0 && opts.ProcessOpts.DegenerateMode != process.DegenerateModeRemove {
		mode := degenerateModeName(opts.ProcessOpts.DegenerateMode)
		profile.Process.DegenerateMode = &mode
	}
	if opts.ProcessOpts.DeboneThreshold != 0 {
		value := float64(opts.ProcessOpts.DeboneThreshold)
		profile.Process.DeboneThreshold = &value
	}

	return profile
}

// LoadProfile reads a TOML profile from disk.
func LoadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Profile{}, err
	}
	return ParseProfileTOML(data)
}

// SaveProfile writes a TOML profile to disk.
func SaveProfile(path string, profile Profile) error {
	data, err := profile.MarshalTOML()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(path), data, profileFilePerm)
}

// ParseProfileTOML parses the v1 TOML import profile schema.
func ParseProfileTOML(data []byte) (Profile, error) {
	profile := Profile{Version: ProfileVersion}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	section := ""

	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
			if line == "" {
				continue
			}
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return Profile{}, fmt.Errorf("profile line %d: malformed section header", lineNo)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section != "decode" && section != "process" {
				return Profile{}, fmt.Errorf("profile line %d: unknown section %q", lineNo, section)
			}
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return Profile{}, fmt.Errorf("profile line %d: expected key = value", lineNo)
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)

		if err := parseProfileKey(&profile, section, key, rawValue); err != nil {
			return Profile{}, fmt.Errorf("profile line %d: %w", lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return Profile{}, err
	}
	if profile.Version == 0 {
		profile.Version = ProfileVersion
	}
	if err := validateProfileVersion(profile.Version); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

// MarshalTOML encodes the profile to the supported TOML schema.
func (p Profile) MarshalTOML() ([]byte, error) {
	if err := validateProfileVersion(p.Version); err != nil {
		return nil, err
	}

	var b strings.Builder
	writeProfileHeader(&b, p)
	writeDecodeSection(&b, p.Decode)
	writeProcessSection(&b, p.Process)
	return []byte(b.String()), nil
}

func parseDecodeKey(profile *Profile, key, rawValue string) error {
	switch key {
	case "max_file_size":
		value, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid decode.max_file_size: %w", err)
		}
		profile.Decode.MaxFileSize = &value
	case "max_vertices":
		value, err := strconv.Atoi(rawValue)
		if err != nil {
			return fmt.Errorf("invalid decode.max_vertices: %w", err)
		}
		profile.Decode.MaxVertices = &value
	case "max_image_pixels":
		value, err := strconv.Atoi(rawValue)
		if err != nil {
			return fmt.Errorf("invalid decode.max_image_pixels: %w", err)
		}
		profile.Decode.MaxImagePixels = &value
	case "max_audio_samples":
		value, err := strconv.Atoi(rawValue)
		if err != nil {
			return fmt.Errorf("invalid decode.max_audio_samples: %w", err)
		}
		profile.Decode.MaxAudioSamples = &value
	default:
		return fmt.Errorf("unknown decode key %q", key)
	}
	return nil
}

func validateProfileVersion(version int) error {
	if version == 0 || version == ProfileVersion {
		return nil
	}
	return fmt.Errorf("unsupported profile version %d", version)
}

func decodeProfileHasContent(profile DecodeProfile) bool {
	return profile.MaxFileSize != nil ||
		profile.MaxVertices != nil ||
		profile.MaxImagePixels != nil ||
		profile.MaxAudioSamples != nil
}

func processProfileHasContent(profile ProcessProfile) bool {
	return len(profile.EnabledSteps) > 0 ||
		len(profile.DisabledSteps) > 0 ||
		profile.SmoothNormalAngle != nil ||
		profile.MaxBoneWeights != nil ||
		profile.MaxVerticesPerMesh != nil ||
		profile.MaxBonesPerMesh != nil ||
		profile.MaxTextureSize != nil ||
		profile.AtlasFontSize != nil ||
		profile.GlobalScale != nil ||
		profile.TargetUpAxis != nil ||
		len(profile.RemoveFlags) > 0 ||
		profile.TargetSampleRate != nil ||
		profile.TargetChannels != nil ||
		profile.DegenerateMode != nil ||
		profile.DeboneThreshold != nil
}

func applyPresetOverride(opts *config, preset string) error {
	if preset == "" {
		return nil
	}
	flags, err := ResolveBuiltInPreset(preset)
	if err != nil {
		return err
	}
	opts.ProcessFlags = flags
	return nil
}

func applyStepOverrides(opts *config, profile ProcessProfile) error {
	for _, stepName := range profile.EnabledSteps {
		flag, err := resolveProcessStep(stepName)
		if err != nil {
			return err
		}
		opts.ProcessFlags |= flag
	}
	for _, stepName := range profile.DisabledSteps {
		flag, err := resolveProcessStep(stepName)
		if err != nil {
			return err
		}
		opts.ProcessFlags &^= flag
	}
	return nil
}

func applyDecodeProfile(opts *config, profile DecodeProfile) {
	if profile.MaxFileSize != nil {
		opts.DecodeOpts.MaxFileSize = *profile.MaxFileSize
	}
	if profile.MaxVertices != nil {
		opts.DecodeOpts.MaxVertices = *profile.MaxVertices
	}
	if profile.MaxImagePixels != nil {
		opts.DecodeOpts.MaxImagePixels = *profile.MaxImagePixels
	}
	if profile.MaxAudioSamples != nil {
		opts.DecodeOpts.MaxAudioSamples = *profile.MaxAudioSamples
	}
}

func applySceneProcessProfile(opts *config, profile ProcessProfile) error {
	if profile.GlobalScale != nil {
		opts.ProcessFlags |= process.PPGlobalScale
		opts.ProcessOpts.GlobalScale = *profile.GlobalScale
	}
	if profile.TargetUpAxis == nil {
		return nil
	}

	axis, err := parseProfileAxis(*profile.TargetUpAxis)
	if err != nil {
		return err
	}
	opts.ProcessFlags |= process.PPFixUpAxis
	opts.ProcessOpts.TargetUpAxis = axis
	return nil
}

func applyGeometryProcessProfile(opts *config, profile ProcessProfile) error {
	if profile.SmoothNormalAngle != nil {
		opts.ProcessFlags |= process.PPGenSmoothNormals
		opts.ProcessOpts.SmoothNormalAngle = *profile.SmoothNormalAngle
	}
	if profile.MaxBoneWeights != nil {
		opts.ProcessFlags |= process.PPLimitBoneWeights
		opts.ProcessOpts.MaxBoneWeights = *profile.MaxBoneWeights
	}
	if profile.MaxVerticesPerMesh != nil {
		opts.ProcessFlags |= process.PPSplitLargeMeshes
		opts.ProcessOpts.MaxVerticesPerMesh = *profile.MaxVerticesPerMesh
	}
	if profile.MaxBonesPerMesh != nil {
		opts.ProcessFlags |= process.PPSplitByBoneCount
		opts.ProcessOpts.MaxBonesPerMesh = *profile.MaxBonesPerMesh
	}
	if len(profile.RemoveFlags) > 0 {
		removeFlags, err := parseComponentFlags(profile.RemoveFlags)
		if err != nil {
			return err
		}
		opts.ProcessFlags |= process.PPRemoveComponent
		opts.ProcessOpts.RemoveFlags = removeFlags
	}
	if profile.DegenerateMode != nil {
		mode, err := parseDegenerateMode(*profile.DegenerateMode)
		if err != nil {
			return err
		}
		opts.ProcessFlags |= process.PPRemoveDegenerates
		opts.ProcessOpts.DegenerateMode = mode
	}
	if profile.DeboneThreshold != nil {
		opts.ProcessFlags |= process.PPDebone
		opts.ProcessOpts.DeboneThreshold = float32(*profile.DeboneThreshold)
	}
	return nil
}

func applyImageProcessProfile(opts *config, profile ProcessProfile) {
	if profile.MaxTextureSize != nil {
		opts.ProcessFlags |= process.PPResizeImages
		opts.ProcessOpts.MaxTextureSize = *profile.MaxTextureSize
	}
	if profile.AtlasFontSize != nil {
		opts.ProcessFlags |= process.PPGenerateFontAtlas
		opts.ProcessOpts.AtlasFontSize = *profile.AtlasFontSize
	}
}

func applyAudioProcessProfile(opts *config, profile ProcessProfile) {
	if profile.TargetSampleRate != nil {
		opts.ProcessFlags |= process.PPResampleAudio
		opts.ProcessOpts.TargetSampleRate = *profile.TargetSampleRate
	}
	if profile.TargetChannels != nil {
		opts.ProcessFlags |= process.PPMixdownAudio
		opts.ProcessOpts.TargetChannels = *profile.TargetChannels
	}
}

func parseProfileKey(profile *Profile, section, key, rawValue string) error {
	switch section {
	case "":
		return parseRootKey(profile, key, rawValue)
	case "decode":
		return parseDecodeKey(profile, key, rawValue)
	case "process":
		return parseProcessKey(profile, key, rawValue)
	default:
		return fmt.Errorf("unknown section %q", section)
	}
}

func parseRootKey(profile *Profile, key, rawValue string) error {
	switch key {
	case "version":
		value, err := strconv.Atoi(rawValue)
		if err != nil {
			return fmt.Errorf("invalid version: %w", err)
		}
		profile.Version = value
		return nil
	case "preset":
		value, err := parseTOMLString(rawValue)
		if err != nil {
			return fmt.Errorf("invalid preset: %w", err)
		}
		profile.Preset = canonicalPresetName(value)
		return nil
	default:
		return fmt.Errorf("unknown key %q", key)
	}
}

func writeProfileHeader(b *strings.Builder, profile Profile) {
	version := profile.Version
	if version == 0 {
		version = ProfileVersion
	}

	writeIntEntry(b, "version", version)
	if profile.Preset != "" {
		writeStringEntry(b, "preset", canonicalPresetName(profile.Preset))
	}
}

func writeDecodeSection(b *strings.Builder, profile DecodeProfile) {
	if !decodeProfileHasContent(profile) {
		return
	}

	b.WriteString("\n[decode]\n")
	writeInt64PtrEntry(b, "max_file_size", profile.MaxFileSize)
	writeIntPtrEntry(b, "max_vertices", profile.MaxVertices)
	writeIntPtrEntry(b, "max_image_pixels", profile.MaxImagePixels)
	writeIntPtrEntry(b, "max_audio_samples", profile.MaxAudioSamples)
}

func writeProcessSection(b *strings.Builder, profile ProcessProfile) {
	if !processProfileHasContent(profile) {
		return
	}

	b.WriteString("\n[process]\n")
	writeStringArrayEntry(b, "enable_steps", profile.EnabledSteps, canonicalStepName)
	writeStringArrayEntry(b, "disable_steps", profile.DisabledSteps, canonicalStepName)
	writeFloat64PtrEntry(b, "smooth_normal_angle", profile.SmoothNormalAngle)
	writeIntPtrEntry(b, "max_bone_weights", profile.MaxBoneWeights)
	writeIntPtrEntry(b, "max_vertices_per_mesh", profile.MaxVerticesPerMesh)
	writeIntPtrEntry(b, "max_bones_per_mesh", profile.MaxBonesPerMesh)
	writeIntPtrEntry(b, "max_texture_size", profile.MaxTextureSize)
	writeIntPtrEntry(b, "atlas_font_size", profile.AtlasFontSize)
	writeFloat64PtrEntry(b, "global_scale", profile.GlobalScale)
	writeStringPtrEntry(b, "target_up_axis", profile.TargetUpAxis, canonicalAxisName)
	writeStringArrayEntry(b, "remove_flags", profile.RemoveFlags, canonicalComponentFlagName)
	writeIntPtrEntry(b, "target_sample_rate", profile.TargetSampleRate)
	writeIntPtrEntry(b, "target_channels", profile.TargetChannels)
	writeStringPtrEntry(b, "degenerate_mode", profile.DegenerateMode, canonicalDegenerateModeName)
	writeFloat64PtrEntry(b, "debone_threshold", profile.DeboneThreshold)
}

func writeIntEntry(b *strings.Builder, key string, value int) {
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.Itoa(value))
	b.WriteByte('\n')
}

func writeStringEntry(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.Quote(value))
	b.WriteByte('\n')
}

func writeInt64PtrEntry(b *strings.Builder, key string, value *int64) {
	if value == nil {
		return
	}
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.FormatInt(*value, 10))
	b.WriteByte('\n')
}

func writeIntPtrEntry(b *strings.Builder, key string, value *int) {
	if value == nil {
		return
	}
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.Itoa(*value))
	b.WriteByte('\n')
}

func writeFloat64PtrEntry(b *strings.Builder, key string, value *float64) {
	if value == nil {
		return
	}
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.FormatFloat(*value, 'f', -1, 64))
	b.WriteByte('\n')
}

func writeStringPtrEntry(b *strings.Builder, key string, value *string, canonicalize func(string) string) {
	if value == nil {
		return
	}
	writeStringEntry(b, key, canonicalize(*value))
}

func writeStringArrayEntry(b *strings.Builder, key string, values []string, canonicalize func(string) string) {
	if len(values) == 0 {
		return
	}

	b.WriteString(key)
	b.WriteString(" = [")
	for i, value := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(canonicalize(value)))
	}
	b.WriteString("]\n")
}

func parseProcessStepList(profile *Profile, key, rawValue string) error {
	values, err := parseTOMLStringArray(rawValue)
	if err != nil {
		return fmt.Errorf("invalid process.%s: %w", key, err)
	}
	for _, value := range values {
		if _, err := resolveProcessStep(value); err != nil {
			return err
		}
	}
	if key == "enable_steps" {
		profile.Process.EnabledSteps = mergeStringSlices(nil, values)
		return nil
	}
	profile.Process.DisabledSteps = mergeStringSlices(nil, values)
	return nil
}

func parseProcessFloatKey(profile *Profile, key, rawValue string) error {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return fmt.Errorf("invalid process.%s: %w", key, err)
	}
	switch key {
	case "smooth_normal_angle":
		profile.Process.SmoothNormalAngle = &value
	case "global_scale":
		profile.Process.GlobalScale = &value
	case "debone_threshold":
		profile.Process.DeboneThreshold = &value
	}
	return nil
}

func parseProcessIntKey(profile *Profile, key, rawValue string) error {
	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return fmt.Errorf("invalid process.%s: %w", key, err)
	}
	switch key {
	case "max_bone_weights":
		profile.Process.MaxBoneWeights = &value
	case "max_vertices_per_mesh":
		profile.Process.MaxVerticesPerMesh = &value
	case "max_bones_per_mesh":
		profile.Process.MaxBonesPerMesh = &value
	case "max_texture_size":
		profile.Process.MaxTextureSize = &value
	case "atlas_font_size":
		profile.Process.AtlasFontSize = &value
	case "target_sample_rate":
		profile.Process.TargetSampleRate = &value
	case "target_channels":
		profile.Process.TargetChannels = &value
	}
	return nil
}

func parseProcessAxisKey(profile *Profile, rawValue string) error {
	value, err := parseTOMLString(rawValue)
	if err != nil {
		return fmt.Errorf("invalid process.target_up_axis: %w", err)
	}
	value = canonicalAxisName(value)
	if _, err := parseProfileAxis(value); err != nil {
		return err
	}
	profile.Process.TargetUpAxis = &value
	return nil
}

func parseProcessRemoveFlagsKey(profile *Profile, rawValue string) error {
	values, err := parseTOMLStringArray(rawValue)
	if err != nil {
		return fmt.Errorf("invalid process.remove_flags: %w", err)
	}
	if _, err := parseComponentFlags(values); err != nil {
		return err
	}
	profile.Process.RemoveFlags = mergeComponentFlagNames(nil, values)
	return nil
}

func parseProcessDegenerateModeKey(profile *Profile, rawValue string) error {
	value, err := parseTOMLString(rawValue)
	if err != nil {
		return fmt.Errorf("invalid process.degenerate_mode: %w", err)
	}
	value = canonicalDegenerateModeName(value)
	if _, err := parseDegenerateMode(value); err != nil {
		return err
	}
	profile.Process.DegenerateMode = &value
	return nil
}

func parseProcessKey(profile *Profile, key, rawValue string) error {
	switch key {
	case "enable_steps", "disable_steps":
		return parseProcessStepList(profile, key, rawValue)
	case "smooth_normal_angle", "global_scale", "debone_threshold":
		return parseProcessFloatKey(profile, key, rawValue)
	case "max_bone_weights", "max_vertices_per_mesh", "max_bones_per_mesh",
		"max_texture_size", "atlas_font_size", "target_sample_rate", "target_channels":
		return parseProcessIntKey(profile, key, rawValue)
	case "target_up_axis":
		return parseProcessAxisKey(profile, rawValue)
	case "remove_flags":
		return parseProcessRemoveFlagsKey(profile, rawValue)
	case "degenerate_mode":
		return parseProcessDegenerateModeKey(profile, rawValue)
	default:
		return fmt.Errorf("unknown process key %q", key)
	}
}

func parseTOMLString(raw string) (string, error) {
	return strconv.Unquote(raw)
}

func parseTOMLStringArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("expected [\"...\"] array")
	}
	raw = strings.TrimSpace(raw[1 : len(raw)-1])
	if raw == "" {
		return nil, nil
	}

	var values []string
	var token strings.Builder
	inString := false
	escaped := false

	for _, r := range raw {
		switch {
		case escaped:
			token.WriteRune(r)
			escaped = false
		case r == '\\':
			token.WriteRune(r)
			escaped = true
		case r == '"':
			token.WriteRune(r)
			inString = !inString
		case r == ',' && !inString:
			value, err := parseTOMLString(strings.TrimSpace(token.String()))
			if err != nil {
				return nil, err
			}
			values = append(values, value)
			token.Reset()
		default:
			token.WriteRune(r)
		}
	}

	if inString {
		return nil, fmt.Errorf("unterminated string array")
	}

	if token.Len() > 0 {
		value, err := parseTOMLString(strings.TrimSpace(token.String()))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	return values, nil
}

func enabledStepsFromOptions(opts config) []string {
	steps := make([]string, 0, len(processStepDefs))
	for _, def := range processStepDefs {
		if opts.ProcessFlags&def.flag != 0 && !isDedicatedProcessStep(def.flag, opts.ProcessOpts) {
			steps = append(steps, def.name)
		}
	}
	return steps
}

func resolveProcessStep(name string) (process.PPFlag, error) {
	canonical := canonicalStepName(name)
	for _, def := range processStepDefs {
		if def.name == canonical {
			return def.flag, nil
		}
	}
	return 0, fmt.Errorf("unknown process step %q", name)
}

func canonicalPresetName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func canonicalStepName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func canonicalAxisName(name string) string {
	return strings.TrimSpace(strings.ToUpper(name))
}

func canonicalComponentFlagName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, "_", "-")
	compact := strings.ReplaceAll(name, "-", "")
	for _, def := range componentFlagDefs {
		if strings.ReplaceAll(def.name, "-", "") == compact {
			return def.name
		}
	}
	return name
}

func canonicalDegenerateModeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func parseProfileAxis(name string) (ir.Axis, error) {
	switch canonicalAxisName(name) {
	case "Y":
		return ir.YUp, nil
	case "Z":
		return ir.ZUp, nil
	default:
		return ir.YUp, fmt.Errorf("unsupported axis %q", name)
	}
}

func parseComponentFlags(names []string) (process.ComponentFlag, error) {
	var flags process.ComponentFlag
	for _, name := range names {
		flag, err := resolveComponentFlag(name)
		if err != nil {
			return 0, err
		}
		flags |= flag
	}
	return flags, nil
}

func resolveComponentFlag(name string) (process.ComponentFlag, error) {
	canonical := canonicalComponentFlagName(name)
	for _, def := range componentFlagDefs {
		if def.name == canonical {
			return def.flag, nil
		}
	}
	return 0, fmt.Errorf("unknown remove flag %q", name)
}

func componentFlagNames(flags process.ComponentFlag) []string {
	names := make([]string, 0, len(componentFlagDefs))
	for _, def := range componentFlagDefs {
		if flags&def.flag != 0 {
			names = append(names, def.name)
		}
	}
	return names
}

func parseDegenerateMode(name string) (process.DegenerateMode, error) {
	switch canonicalDegenerateModeName(name) {
	case "", "remove":
		return process.DegenerateModeRemove, nil
	case "convert":
		return process.DegenerateModeConvert, nil
	default:
		return process.DegenerateModeRemove, fmt.Errorf("unsupported degenerate mode %q", name)
	}
}

func degenerateModeName(mode process.DegenerateMode) string {
	if mode == process.DegenerateModeConvert {
		return "convert"
	}
	return "remove"
}

func axisName(axis ir.Axis) string {
	if axis == ir.ZUp {
		return "Z"
	}
	return "Y"
}

func mergeStringSlices(base, extra []string) []string {
	return mergeCanonicalStrings(base, extra, canonicalStepName)
}

func mergeComponentFlagNames(base, extra []string) []string {
	return mergeCanonicalStrings(base, extra, canonicalComponentFlagName)
}

func mergeCanonicalStrings(base, extra []string, canonicalize func(string) string) []string {
	if len(extra) == 0 {
		return slices.Clone(base)
	}
	merged := slices.Clone(base)
	for _, item := range extra {
		canonical := canonicalize(item)
		if !slices.Contains(merged, canonical) {
			merged = append(merged, canonical)
		}
	}
	return merged
}

func isDedicatedProcessStep(flag process.PPFlag, opts process.Options) bool {
	switch flag {
	case process.PPGlobalScale, process.PPFixUpAxis:
		return true
	case process.PPGenSmoothNormals:
		return opts.SmoothNormalAngle != 0
	case process.PPLimitBoneWeights:
		return opts.MaxBoneWeights != 0
	case process.PPSplitLargeMeshes:
		return opts.MaxVerticesPerMesh != 0
	case process.PPSplitByBoneCount:
		return opts.MaxBonesPerMesh != 0
	case process.PPResizeImages:
		return opts.MaxTextureSize != 0
	case process.PPGenerateFontAtlas:
		return opts.AtlasFontSize != 0
	case process.PPRemoveComponent:
		return opts.RemoveFlags != 0
	case process.PPResampleAudio:
		return opts.TargetSampleRate != 0
	case process.PPMixdownAudio:
		return opts.TargetChannels != 0
	case process.PPRemoveDegenerates:
		return opts.DegenerateMode != process.DegenerateModeRemove
	case process.PPDebone:
		return opts.DeboneThreshold != 0
	default:
		return false
	}
}

func cloneInt64Ptr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	value := *v
	return &value
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	value := *v
	return &value
}

func cloneFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	value := *v
	return &value
}
