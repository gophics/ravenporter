package ir

// NormalizeGraph rebuilds node parent links and keeps primary root lists consistent.
func (a *Asset) NormalizeGraph() {
	if a == nil {
		return
	}

	rootSet := make([]bool, len(a.Nodes))
	a.RootNodes = sanitizeNodeIndices(a.RootNodes, len(a.Nodes), rootSet)
	for _, scene := range a.Scenes {
		if scene == nil {
			continue
		}
		scene.RootNodes = sanitizeNodeIndices(scene.RootNodes, len(a.Nodes), rootSet)
	}

	for i := range a.Nodes {
		a.Nodes[i].Children = sanitizeNodeIndices(a.Nodes[i].Children, len(a.Nodes), nil)
		a.Nodes[i].ParentIndex = NoIndex
	}

	for parent := range a.Nodes {
		for _, child := range a.Nodes[parent].Children {
			if child == parent || rootSet[child] || a.Nodes[child].ParentIndex != NoIndex {
				continue
			}
			a.Nodes[child].ParentIndex = parent
		}
	}

	derivedRoots := deriveRootNodes(a.Nodes)
	if len(a.Scenes) == 0 {
		if len(a.RootNodes) == 0 {
			a.RootNodes = derivedRoots
		}
		return
	}

	primary := a.primarySceneEntry()
	switch {
	case len(a.RootNodes) > 0:
		if primary != nil {
			primary.RootNodes = cloneIndices(a.RootNodes)
		}
	case primary != nil && len(primary.RootNodes) > 0:
		a.RootNodes = cloneIndices(primary.RootNodes)
	default:
		a.RootNodes = derivedRoots
		if primary != nil {
			primary.RootNodes = cloneIndices(derivedRoots)
		}
	}
}

func sanitizeNodeIndices(indices []int, limit int, seen []bool) []int {
	if len(indices) == 0 || limit == 0 {
		return nil
	}
	localSeen := seen
	if localSeen == nil {
		localSeen = make([]bool, limit)
	}
	out := make([]int, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= limit || localSeen[idx] {
			continue
		}
		localSeen[idx] = true
		out = append(out, idx)
	}
	return out
}

func deriveRootNodes(nodes []Node) []int {
	if len(nodes) == 0 {
		return nil
	}
	roots := make([]int, 0, len(nodes))
	for i := range nodes {
		if nodes[i].ParentIndex == NoIndex {
			roots = append(roots, i)
		}
	}
	return roots
}

func cloneIndices(indices []int) []int {
	if len(indices) == 0 {
		return nil
	}
	cloned := make([]int, len(indices))
	copy(cloned, indices)
	return cloned
}
