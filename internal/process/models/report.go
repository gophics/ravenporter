package models

import (
	"log/slog"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type reportStatsStep struct{}

func (s *reportStatsStep) Name() string      { return "ReportStats" }
func (s *reportStatsStep) Flag() core.PPFlag { return core.PPReportStats }

func (s *reportStatsStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	var totalPrims, totalVerts, totalTris int
	for _, mesh := range asset.Meshes {
		if mesh == nil {
			continue
		}
		for pi := range mesh.Primitives {
			p := &mesh.Primitives[pi]
			totalPrims++
			totalVerts += p.Data.VertexCount
			if p.Data.HasIndices() {
				totalTris += len(p.Data.Indices) / 3 //nolint:mnd // triangles
			}
		}
	}

	logger.Info("asset statistics",
		slog.Int("meshes", len(asset.Meshes)),
		slog.Int("primitives", totalPrims),
		slog.Int("vertices", totalVerts),
		slog.Int("triangles", totalTris),
		slog.Int("materials", len(asset.Materials)),
		slog.Int("textures", len(asset.Textures)),
		slog.Int("nodes", len(asset.Nodes)),
		slog.Int("animations", len(asset.Animations)),
		slog.Int("skeletons", len(asset.Skeletons)),
	)

	return asset, nil
}
