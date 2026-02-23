package graph

// BFS performs a breadth-first search starting from nodeID
func (g *AdjacencyDAG) BFS(nodeID string, fn func(id string) bool) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.HasNode(nodeID) {
		return ErrNodeNotFound
	}

	queue := []string{nodeID}
	visited := make(map[string]bool)
	visited[nodeID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if !fn(curr) {
			break
		}

		for next := range g.outgoing[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	return nil
}

// DFS performs a depth-first search starting from nodeID
func (g *AdjacencyDAG) DFS(nodeID string, fn func(id string) bool) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.HasNode(nodeID) {
		return ErrNodeNotFound
	}

	visited := make(map[string]bool)
	var visit func(string) bool
	visit = func(id string) bool {
		visited[id] = true
		if !fn(id) {
			return false
		}
		for next := range g.outgoing[id] {
			if !visited[next] {
				if !visit(next) {
					return false
				}
			}
		}
		return true
	}

	visit(nodeID)
	return nil
}
