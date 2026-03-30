package pipeline

import (
	"cmp"
	"slices"

	"github.com/gophics/ravenporter/ir"
)

const (
	// StageDetect marks issues raised during format detection.
	StageDetect = "detect"

	// StageDecode marks issues raised during decoder execution.
	StageDecode = "decode"

	// StageValidateStructural marks issues raised during structural validation.
	StageValidateStructural = "validate-structural"

	// StageProcess marks issues raised during post-processing.
	StageProcess = "process"

	// StageValidateSemantic marks issues raised during semantic validation.
	StageValidateSemantic = "validate-semantic"
)

const (
	// SeverityError marks a fatal import issue.
	SeverityError = "error"

	// SeverityWarning marks a non-fatal import issue.
	SeverityWarning = "warning"

	// SeverityInfo marks an informational import issue.
	SeverityInfo = "info"
)

// Result is the public pipeline return type for a single import.
type Result struct {
	Asset  *ir.Asset `json:"asset,omitempty"`
	Report Report    `json:"report"`
}

// Report is the structured machine-readable import report.
type Report struct {
	Source       SourceReport `json:"source"`
	Issues       []Issue      `json:"issues,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
	Summary      AssetSummary `json:"summary"`
}

// SourceReport describes input provenance and effective import behavior.
type SourceReport struct {
	InputName      string              `json:"input_name,omitempty"`
	InputPath      string              `json:"input_path,omitempty"`
	DetectedFormat ir.FormatID         `json:"detected_format,omitempty"`
	Metadata       ir.AssetMetadata    `json:"metadata"`
	Preset         string              `json:"preset,omitempty"`
	ProfileFile    string              `json:"profile_file,omitempty"`
	Options        Profile             `json:"options"`
	Notes          map[string][]string `json:"notes,omitempty"`
}

// Issue is a stable pipeline finding entry.
type Issue struct {
	Stage    string `json:"stage"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// Dependency is a direct external dependency discovered during import.
type Dependency struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	Relation   string `json:"relation"`
	ReportedBy string `json:"reported_by"`
}

// AssetSummary records high-level counts for the imported asset.
type AssetSummary struct {
	Scenes          int `json:"scenes"`
	Meshes          int `json:"meshes"`
	Materials       int `json:"materials"`
	Textures        int `json:"textures"`
	Nodes           int `json:"nodes"`
	InstancedMeshes int `json:"instanced_meshes"`
	InstanceNodes   int `json:"instance_nodes"`
	Animations      int `json:"animations"`
	Skeletons       int `json:"skeletons"`
	Cameras         int `json:"cameras"`
	Lights          int `json:"lights"`
	AudioClips      int `json:"audio_clips"`
	Fonts           int `json:"fonts"`
	Images          int `json:"images"`
	LODGroups       int `json:"lod_groups"`
	CollisionMeshes int `json:"collision_meshes"`
}

type reportCollector struct {
	dependencies map[Dependency]struct{}
	notes        map[string][]string
}

func newReportCollector() *reportCollector {
	return &reportCollector{
		dependencies: make(map[Dependency]struct{}),
		notes:        make(map[string][]string),
	}
}

func (c *reportCollector) AddDependency(kind, path, relation, reportedBy string) {
	if c == nil || path == "" {
		return
	}
	c.dependencies[Dependency{
		Kind:       kind,
		Path:       path,
		Relation:   relation,
		ReportedBy: reportedBy,
	}] = struct{}{}
}

func (c *reportCollector) AddProvenanceNote(key, value string) {
	if c == nil || key == "" || value == "" {
		return
	}
	existing := c.notes[key]
	if slices.Contains(existing, value) {
		return
	}
	c.notes[key] = append(existing, value)
}

func (c *reportCollector) dependenciesList() []Dependency {
	if c == nil || len(c.dependencies) == 0 {
		return nil
	}
	out := make([]Dependency, 0, len(c.dependencies))
	for dep := range c.dependencies {
		out = append(out, dep)
	}
	slices.SortFunc(out, func(a, b Dependency) int {
		if n := cmp.Compare(a.Path, b.Path); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Kind, b.Kind); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Relation, b.Relation); n != 0 {
			return n
		}
		return cmp.Compare(a.ReportedBy, b.ReportedBy)
	})
	return out
}

func (c *reportCollector) noteMap() map[string][]string {
	if c == nil || len(c.notes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(c.notes))
	for key := range c.notes {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	out := make(map[string][]string, len(c.notes))
	for _, key := range keys {
		values := slices.Clone(c.notes[key])
		slices.Sort(values)
		out[key] = values
	}
	return out
}

func collectAssetDependencies(asset *ir.Asset, collector *reportCollector) {
	if asset == nil || collector == nil {
		return
	}
	for _, img := range asset.Images {
		if img == nil || img.SourcePath == "" {
			continue
		}
		collector.AddDependency("image", img.SourcePath, "image", "asset")
	}
}

func assetSummary(asset *ir.Asset) AssetSummary {
	if asset == nil {
		return AssetSummary{}
	}
	instancedMeshes, instanceNodes := meshInstanceStats(asset)
	return AssetSummary{
		Scenes:          len(asset.Scenes),
		Meshes:          len(asset.Meshes),
		Materials:       len(asset.Materials),
		Textures:        len(asset.Textures),
		Nodes:           len(asset.Nodes),
		InstancedMeshes: instancedMeshes,
		InstanceNodes:   instanceNodes,
		Animations:      len(asset.Animations),
		Skeletons:       len(asset.Skeletons),
		Cameras:         len(asset.Cameras),
		Lights:          len(asset.Lights),
		AudioClips:      len(asset.AudioClips),
		Fonts:           len(asset.Fonts),
		Images:          len(asset.Images),
		LODGroups:       len(asset.LODGroups),
		CollisionMeshes: len(asset.CollisionMeshes),
	}
}

func meshInstanceStats(asset *ir.Asset) (instancedMeshes, instanceNodes int) {
	if asset == nil || len(asset.Nodes) == 0 {
		return 0, 0
	}

	refCounts := make(map[int]int, len(asset.Meshes))
	for i := range asset.Nodes {
		node := &asset.Nodes[i]
		if node.MeshIndex != ir.NoIndex {
			refCounts[node.MeshIndex]++
		}
	}

	for _, count := range refCounts {
		if count > 1 {
			instancedMeshes++
			instanceNodes += count
		}
	}
	return instancedMeshes, instanceNodes
}
