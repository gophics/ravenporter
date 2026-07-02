package models

import (
	"math"
	"sync"

	"github.com/gophics/ravenporter/internal/process/core"
	"github.com/gophics/ravenporter/ir"
)

var cachePool = sync.Pool{New: func() any { return &cacheBuf{} }}

type cacheBuf struct {
	ints    []int
	floats  []float64
	emitted []bool
}

type optimizeCacheStep struct{}

func (s *optimizeCacheStep) Name() string      { return "OptimizeCache" }
func (s *optimizeCacheStep) Flag() core.PPFlag { return core.PPOptimizeCache }

const (
	defaultCacheSize = 32
	maxValenceBoost  = 32
	vertsPerTriangle = 3
	intBufPartitions = 3

	cacheDecayPower   = 1.5
	lastTriScoreBoost = 0.75
	valenceBoostScale = 2.0
	valenceBoostPower = 0.5
)

func (s *optimizeCacheStep) Apply(asset *ir.Asset, _ core.Options) (*ir.Asset, error) {
	for i := range asset.Meshes {
		mesh := asset.Meshes[i]
		if mesh == nil {
			continue
		}
		for j := range mesh.Primitives {
			p := &mesh.Primitives[j]
			if p.Mode != ir.Triangles || !p.Data.HasIndices() || len(p.Data.Indices) < vertsPerTriangle {
				continue
			}
			optimizeVertexCache(&p.Data, defaultCacheSize)
		}
	}
	return asset, nil
}

//nolint:funlen // Forsyth vertex cache optimizer is inherently procedural
func optimizeVertexCache(d *ir.MeshData, cacheSize int) {
	numTris := len(d.Indices) / vertsPerTriangle
	numVerts := d.VertexCount
	if numVerts == 0 {
		numVerts = len(d.Positions)
	}

	cb := cachePool.Get().(*cacheBuf) //nolint:errcheck // pool New guarantees type

	intNeed := numVerts * intBufPartitions
	if cap(cb.ints) < intNeed {
		cb.ints = make([]int, intNeed)
	} else {
		cb.ints = cb.ints[:intNeed]
		clear(cb.ints)
	}
	liveTriCount := cb.ints[:numVerts]
	offsets := cb.ints[numVerts : numVerts*2]
	insertCursor := cb.ints[numVerts*2 : numVerts*intBufPartitions]

	for _, idx := range d.Indices {
		if int(idx) < numVerts {
			liveTriCount[idx]++
		}
	}

	totalEntries := 0
	for v, c := range liveTriCount {
		offsets[v] = totalEntries
		totalEntries += c
	}

	vertTriList := make([]int, totalEntries)
	for t := 0; t < numTris; t++ {
		for k := range vertsPerTriangle {
			v := d.Indices[t*vertsPerTriangle+k]
			if int(v) < numVerts {
				vertTriList[offsets[v]+insertCursor[v]] = t
				insertCursor[v]++
			}
		}
	}

	floatNeed := numVerts + numTris
	if cap(cb.floats) < floatNeed {
		cb.floats = make([]float64, floatNeed)
	} else {
		cb.floats = cb.floats[:floatNeed]
		clear(cb.floats)
	}
	scores := cb.floats[:numVerts]
	triScores := cb.floats[numVerts:]

	if cap(cb.emitted) < numTris {
		cb.emitted = make([]bool, numTris)
	} else {
		cb.emitted = cb.emitted[:numTris]
		clear(cb.emitted)
	}
	triEmitted := cb.emitted

	for v := range scores {
		scores[v] = computeVertexScore(-1, liveTriCount[v])
	}

	for t := 0; t < numTris; t++ {
		for k := range vertsPerTriangle {
			v := d.Indices[t*vertsPerTriangle+k]
			if int(v) < numVerts {
				triScores[t] += scores[v]
			}
		}
	}

	cache := make([]uint32, 0, cacheSize)
	newIndices := make([]uint32, 0, len(d.Indices))

	for len(newIndices) < len(d.Indices) {
		bestTri := -1
		bestScore := -1.0
		for t, s := range triScores {
			if !triEmitted[t] && s > bestScore {
				bestScore = s
				bestTri = t
			}
		}

		if bestTri < 0 {
			break
		}

		triEmitted[bestTri] = true
		baseIdx := bestTri * vertsPerTriangle
		triVerts := [vertsPerTriangle]uint32{
			d.Indices[baseIdx],
			d.Indices[baseIdx+1],
			d.Indices[baseIdx+2],
		}
		newIndices = append(newIndices, triVerts[0], triVerts[1], triVerts[2])

		for _, v := range triVerts {
			if int(v) < numVerts {
				liveTriCount[v]--
			}
		}

		updateCache(&cache, triVerts, cacheSize)

		for ci, v := range cache {
			if int(v) < numVerts {
				scores[v] = computeVertexScore(ci, liveTriCount[v])
			}
		}

		updateAffectedTriScores(d, scores, triEmitted, insertCursor, offsets,
			liveTriCount, triVerts, triScores, numVerts, vertTriList)
	}

	copy(d.Indices, newIndices)
	cachePool.Put(cb)
}

func updateCache(cache *[]uint32, triVerts [vertsPerTriangle]uint32, cacheSize int) {
outer:
	for _, v := range triVerts {
		for ci, cv := range *cache {
			if cv == v {
				copy((*cache)[1:ci+1], (*cache)[:ci])
				(*cache)[0] = v
				continue outer
			}
		}
		*cache = append(*cache, 0)
		copy((*cache)[1:], (*cache)[:len(*cache)-1])
		(*cache)[0] = v
	}
	if len(*cache) > cacheSize {
		*cache = (*cache)[:cacheSize]
	}
}

func updateAffectedTriScores(
	d *ir.MeshData,
	scores []float64,
	triEmitted []bool,
	insertCursor, offsets, liveTriCount []int,
	triVerts [vertsPerTriangle]uint32,
	triScores []float64,
	numVerts int,
	vertTriList []int,
) {
	for _, v := range triVerts {
		if int(v) >= numVerts {
			continue
		}
		count := insertCursor[v]
		if count > liveTriCount[v]+1 {
			count = liveTriCount[v] + 1
		}
		for ai := 0; ai < count; ai++ {
			t := vertTriList[offsets[v]+ai]
			if triEmitted[t] {
				continue
			}
			var sum float64
			base := t * vertsPerTriangle
			for k := range vertsPerTriangle {
				sv := d.Indices[base+k]
				if int(sv) < numVerts {
					sum += scores[sv]
				}
			}
			triScores[t] = sum
		}
	}
}

func computeVertexScore(cachePos, liveTriCount int) float64 {
	if liveTriCount <= 0 {
		return -1.0
	}

	var score float64

	if cachePos >= 0 {
		if cachePos < vertsPerTriangle {
			score = lastTriScoreBoost
		} else {
			scaler := 1.0 / float64(defaultCacheSize-vertsPerTriangle)
			score = math.Pow(1.0-float64(cachePos-vertsPerTriangle)*scaler, cacheDecayPower)
		}
	}

	valence := liveTriCount
	if valence > maxValenceBoost {
		valence = maxValenceBoost
	}
	score += valenceBoostScale * math.Pow(float64(valence), -valenceBoostPower)

	return score
}
