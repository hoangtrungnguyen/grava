package graph

// TopologicalSort performs Kahn's algorithm for topological sorting
// Returns sorted node IDs or error if cycle detected
func (g *AdjacencyDAG) TopologicalSort() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.topologicalSortUnsafe()
}

// topologicalSortUnsafe performs Kahn's algorithm without locking
func (g *AdjacencyDAG) topologicalSortUnsafe() ([]string, error) {
	// Calculate indegrees
	indegree := make(map[string]int)
	for nodeID := range g.nodes {
		indegree[nodeID] = len(g.incoming[nodeID])
	}

	// Initialize queue with zero-indegree nodes
	queue := []string{}
	for nodeID, deg := range indegree {
		if deg == 0 {
			queue = append(queue, nodeID)
		}
	}

	// Process nodes
	result := []string{}
	processed := 0

	for len(queue) > 0 {
		// Dequeue
		nodeID := queue[0]
		queue = queue[1:]

		result = append(result, nodeID)
		processed++

		// Reduce indegree of successors
		for toID := range g.outgoing[nodeID] {
			indegree[toID]--
			if indegree[toID] == 0 {
				queue = append(queue, toID)
			}
		}
	}

	// Check if all nodes processed (no cycle)
	if processed != len(g.nodes) {
		return nil, ErrCycleDetected
	}

	return result, nil
}

// DetectCycle returns the cycle path if one exists
func (g *AdjacencyDAG) DetectCycle() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.detectCycleUnsafe()
}

// detectCycleUnsafe uses DFS to detect cycles and return cycle path
func (g *AdjacencyDAG) detectCycleUnsafe() ([]string, error) {
	WHITE := 0 // Unvisited
	GRAY := 1  // Visiting
	BLACK := 2 // Visited

	color := make(map[string]int)
	parent := make(map[string]string)

	for nodeID := range g.nodes {
		color[nodeID] = WHITE
	}

	var dfs func(string) (bool, []string)
	dfs = func(nodeID string) (bool, []string) {
		color[nodeID] = GRAY

		for toID := range g.outgoing[nodeID] {
			if color[toID] == WHITE {
				parent[toID] = nodeID
				if found, cycle := dfs(toID); found {
					return true, cycle
				}
			} else if color[toID] == GRAY {
				// Found cycle - reconstruct path
				cycle := []string{toID}
				current := nodeID
				for current != toID {
					cycle = append([]string{current}, cycle...)
					current = parent[current]
				}
				cycle = append([]string{toID}, cycle...) // Complete the cycle
				return true, cycle
			}
		}

		color[nodeID] = BLACK
		return false, nil
	}

	for nodeID := range g.nodes {
		if color[nodeID] == WHITE {
			if found, cycle := dfs(nodeID); found {
				return cycle, nil
			}
		}
	}

	return nil, nil
}
