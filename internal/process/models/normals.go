package models

import (
	"math"

	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

type genNormalsStep struct{}

func (s *genNormalsStep) Name() string      { return "GenNormals" }
func (s *genNormalsStep) Flag() core.PPFlag { return core.PPGenNormals }

func (s *genNormalsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if len(p.Data.Normals) > 0 || p.Mode != ir.Triangles {
				continue
			}
			generateFlatNormals(&p.Data)
		}
	}
	return asset, nil
}

type genSmoothNormalsStep struct{}

func (s *genSmoothNormalsStep) Name() string      { return "GenSmoothNormals" }
func (s *genSmoothNormalsStep) Flag() core.PPFlag { return core.PPGenSmoothNormals }

func (s *genSmoothNormalsStep) Apply(asset *ir.Asset, opts core.Options) (*ir.Asset, error) {
	angle := opts.SmoothNormalAngle
	if angle <= 0 {
		const defaultSmoothAngle = 60.0
		const halfCircle = 180.0
		angle = defaultSmoothAngle * (math.Pi / halfCircle)
	}
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if len(p.Data.Normals) > 0 || p.Mode != ir.Triangles {
				continue
			}
			generateSmoothNormals(&p.Data, float32(angle))
		}
	}
	return asset, nil
}

type forceGenNormalsStep struct{}

func (s *forceGenNormalsStep) Name() string      { return "ForceGenNormals" }
func (s *forceGenNormalsStep) Flag() core.PPFlag { return core.PPForceGenNormals }

func (s *forceGenNormalsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			mesh.Primitives[j].Data.Normals = nil
			if mesh.Primitives[j].Mode == ir.Triangles {
				generateFlatNormals(&mesh.Primitives[j].Data)
			}
		}
	}
	return asset, nil
}

type dropNormalsStep struct{}

func (s *dropNormalsStep) Name() string      { return "DropNormals" }
func (s *dropNormalsStep) Flag() core.PPFlag { return core.PPDropNormals }

func (s *dropNormalsStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			mesh.Primitives[j].Data.Normals = nil
			mesh.Primitives[j].Data.Tangents = nil
		}
	}
	return asset, nil
}

func generateFlatNormals(d *ir.MeshData) {
	if len(d.Positions) == 0 {
		return
	}
	d.Normals = make([][3]float32, len(d.Positions))

	if d.HasIndices() {
		for i := 0; i+2 < len(d.Indices); i += 3 {
			i0, i1, i2 := d.Indices[i], d.Indices[i+1], d.Indices[i+2]
			v0, v1, v2 := d.Positions[i0], d.Positions[i1], d.Positions[i2]

			normal := mathx.Normalize3(mathx.Cross3(mathx.Sub3(v1, v0), mathx.Sub3(v2, v0)))

			d.Normals[i0] = add3(d.Normals[i0], normal)
			d.Normals[i1] = add3(d.Normals[i1], normal)
			d.Normals[i2] = add3(d.Normals[i2], normal)
		}
	} else {
		for i := 0; i+2 < len(d.Positions); i += 3 {
			v0, v1, v2 := d.Positions[i], d.Positions[i+1], d.Positions[i+2]
			normal := mathx.Normalize3(mathx.Cross3(mathx.Sub3(v1, v0), mathx.Sub3(v2, v0)))
			d.Normals[i] = normal
			d.Normals[i+1] = normal
			d.Normals[i+2] = normal
		}
	}

	for i := range d.Normals {
		d.Normals[i] = mathx.Normalize3(d.Normals[i])
	}
}

func generateSmoothNormals(d *ir.MeshData, maxAngle float32) {
	if len(d.Positions) == 0 {
		return
	}

	if !d.HasIndices() {
		generateFlatNormals(d)
		return
	}

	const vertsPerTri = 3
	faces := len(d.Indices) / vertsPerTri
	faceNormals := make([][3]float32, faces)

	// First pass: count how many faces reference each vertex.
	vertFaceCounts := make([]int, len(d.Positions))
	for f := 0; f < faces; f++ {
		idx := f * vertsPerTri
		i0, i1, i2 := d.Indices[idx], d.Indices[idx+1], d.Indices[idx+2]
		v0, v1, v2 := d.Positions[i0], d.Positions[i1], d.Positions[i2]

		faceNormals[f] = mathx.Normalize3(mathx.Cross3(mathx.Sub3(v1, v0), mathx.Sub3(v2, v0)))

		vertFaceCounts[i0]++
		vertFaceCounts[i1]++
		vertFaceCounts[i2]++
	}

	// Compute total adjacency entries and prefix-sum offsets.
	totalEntries := 0
	offsets := make([]int, len(d.Positions))
	for v, c := range vertFaceCounts {
		offsets[v] = totalEntries
		totalEntries += c
	}

	// Single flat backing buffer for all vertex-face adjacency.
	flatAdj := make([]int, totalEntries)

	// Reuse counts as insertion cursors (reset to zero).
	for i := range vertFaceCounts {
		vertFaceCounts[i] = 0
	}

	// Second pass: fill the flat adjacency buffer.
	for f := 0; f < faces; f++ {
		idx := f * vertsPerTri
		for _, vi := range d.Indices[idx : idx+vertsPerTri] {
			flatAdj[offsets[vi]+vertFaceCounts[vi]] = f
			vertFaceCounts[vi]++
		}
	}

	d.Normals = make([][3]float32, len(d.Positions))
	minDot := float32(math.Cos(float64(maxAngle)))

	for v := range d.Positions {
		faceCount := vertFaceCounts[v]
		if faceCount == 0 {
			continue
		}

		start := offsets[v]
		f0 := flatAdj[start]
		baseNormal := faceNormals[f0]
		sumNormal := baseNormal

		for fi := 1; fi < faceCount; fi++ {
			fn := faceNormals[flatAdj[start+fi]]
			dot := dot3(baseNormal, fn)
			if dot >= minDot {
				sumNormal = add3(sumNormal, fn)
			}
		}

		d.Normals[v] = mathx.Normalize3(sumNormal)
	}
}

func add3(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func dot3(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}
