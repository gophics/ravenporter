package gltf

import (
	"strconv"
	"strings"

	"github.com/gophics/ravenporter/internal/decutil"
	"github.com/gophics/ravenporter/ir"
	"github.com/valyala/fastjson"
)

const (
	modePoints        = 0
	modeLines         = 1
	modeLineLoop      = 2
	modeLineStrip     = 3
	modeTriangles     = 4
	modeTriangleStrip = 5
	modeTriangleFan   = 6
)

func (d *doc) convertMeshesChecked() ([]*ir.Mesh, error) {
	arr := d.root.GetArray(keyMeshes)
	out := make([]*ir.Mesh, len(arr))
	for i, m := range arr {
		mesh, err := d.convertMesh(m)
		if err != nil {
			return nil, err
		}
		out[i] = mesh
	}
	return out, nil
}

func (d *doc) convertMesh(m *fastjson.Value) (*ir.Mesh, error) {
	prims := m.GetArray(keyPrimitives)
	mesh := &ir.Mesh{
		Name:         string(m.GetStringBytes(keyName)),
		Primitives:   make([]ir.Primitive, len(prims)),
		MorphWeights: getFloat32Slice(m, keyWeights),
	}
	for i, p := range prims {
		prim, err := d.convertPrimitive(p)
		if err != nil {
			return nil, err
		}
		mesh.Primitives[i] = prim
	}
	return mesh, nil
}

func (d *doc) convertPrimitive(p *fastjson.Value) (ir.Primitive, error) {
	data, err := d.readAttributes(p)
	if err != nil {
		return ir.Primitive{}, err
	}

	if idxVal := p.Get(keyIndices); idxVal != nil {
		a := d.getAccessor(idxVal.GetInt())
		data.Indices = d.bufs.readIndices(a)
	}

	matIdx := ir.NoIndex
	if matVal := p.Get(keyMaterial); matVal != nil {
		matIdx = matVal.GetInt()
	}

	return ir.Primitive{
		Data:          data,
		MaterialIndex: matIdx,
		Mode:          primitiveMode(p.GetInt(keyMode)),
		MorphTargets:  d.convertMorphTargets(p),
	}, nil
}

func (d *doc) parsePerPrimitiveVariants(asset *ir.Asset) {
	meshArr := d.root.GetArray(keyMeshes)
	for mi, m := range meshArr {
		for pi, p := range m.GetArray(keyPrimitives) {
			ext := p.Get(keyExtensions, keyKHRMatVariants)
			if ext == nil {
				continue
			}
			mappings := ext.GetArray(keyMappings)
			if len(mappings) == 0 {
				continue
			}
			var parts []string
			for _, mp := range mappings {
				matI := mp.GetInt(keyMaterial)
				for _, vi := range mp.GetArray(keyVariants) {
					parts = append(parts, strconv.Itoa(vi.GetInt())+":"+strconv.Itoa(matI))
				}
			}
			if len(parts) > 0 {
				if asset.Metadata.ExtraProperties == nil {
					asset.Metadata.ExtraProperties = make(map[string]string)
				}
				key := "variantMappings_mesh" + strconv.Itoa(mi) + "_prim" + strconv.Itoa(pi)
				asset.Metadata.ExtraProperties[key] = strings.Join(parts, ",")
			}
		}
	}
}

func (d *doc) readAttributes(p *fastjson.Value) (ir.MeshData, error) {
	attrs := p.Get(keyAttributes)
	if attrs == nil {
		return ir.MeshData{}, nil
	}

	var data ir.MeshData

	if v := attrs.Get(attrPosition); v != nil {
		a := d.getAccessor(v.GetInt())
		if d.opts.MaxVertices > 0 && a.count > d.opts.MaxVertices {
			return ir.MeshData{}, decutil.DecodeErr(ir.FormatGLTF, "vertex limit exceeded", nil)
		}
		data.Positions = d.bufs.readVec3s(a)
		data.VertexCount = len(data.Positions)
	}
	if v := attrs.Get(attrNormal); v != nil {
		data.Normals = d.bufs.readVec3s(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrTangent); v != nil {
		data.Tangents = d.bufs.readVec4s(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrTexCoord0); v != nil {
		data.TexCoord0 = d.bufs.readVec2s(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrTexCoord1); v != nil {
		data.TexCoord1 = d.bufs.readVec2s(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrTexCoord2); v != nil {
		data.TexCoord2 = d.bufs.readVec2s(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrTexCoord3); v != nil {
		data.TexCoord3 = d.bufs.readVec2s(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrColor0); v != nil {
		data.Colors0 = d.bufs.readColors(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrJoints0); v != nil {
		data.Joints0 = d.bufs.readJoints(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrJoints1); v != nil {
		data.Joints1 = d.bufs.readJoints(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrWeights0); v != nil {
		data.Weights0 = d.bufs.readVec4s(d.getAccessor(v.GetInt()))
	}
	if v := attrs.Get(attrWeights1); v != nil {
		data.Weights1 = d.bufs.readVec4s(d.getAccessor(v.GetInt()))
	}

	return data, nil
}

func (d *doc) convertMorphTargets(p *fastjson.Value) []ir.MorphTarget {
	arr := p.GetArray(keyTargets)
	if len(arr) == 0 {
		return nil
	}
	out := make([]ir.MorphTarget, len(arr))
	for i, t := range arr {
		mt := ir.MorphTarget{}
		if v := t.Get(attrPosition); v != nil {
			mt.Positions = d.bufs.readVec3s(d.getAccessor(v.GetInt()))
		}
		if v := t.Get(attrNormal); v != nil {
			mt.Normals = d.bufs.readVec3s(d.getAccessor(v.GetInt()))
		}
		if v := t.Get(attrTangent); v != nil {
			vec4s := d.bufs.readVec4s(d.getAccessor(v.GetInt()))
			mt.Tangents = make([][3]float32, len(vec4s))
			for j, v4 := range vec4s {
				mt.Tangents[j] = [3]float32{v4[0], v4[1], v4[2]}
			}
		}
		out[i] = mt
	}
	return out
}

func primitiveMode(m int) ir.PrimitiveMode {
	switch m {
	case modeTriangles:
		return ir.Triangles
	case modeTriangleFan:
		return ir.TriangleFan
	case modeTriangleStrip:
		return ir.TriangleStrip
	case modeLines:
		return ir.Lines
	case modeLineLoop:
		return ir.LineLoop
	case modeLineStrip:
		return ir.LineStrip
	case modePoints:
		return ir.Points
	default:
		return ir.Triangles
	}
}
