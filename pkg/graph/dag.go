package graph

import (
	"sync"
)

// AdjacencyDAG implements DAG using adjacency lists
type AdjacencyDAG struct {
	mu sync.RWMutex

	// Core data structures
	nodes    map[string]*Node            // nodeID -> Node
	outgoing map[string]map[string]*Edge // fromID -> toID -> Edge
	incoming map[string]map[string]*Edge // toID -> fromID -> Edge

	// Cached computations
	cache *GraphCache
}

// NewAdjacencyDAG creates a new DAG
func NewAdjacencyDAG(enableCache bool) *AdjacencyDAG {
	dag := &AdjacencyDAG{
		nodes:    make(map[string]*Node),
		outgoing: make(map[string]map[string]*Edge),
		incoming: make(map[string]map[string]*Edge),
	}

	if enableCache {
		dag.cache = NewGraphCache(dag)
	}

	return dag
}

// AddNode adds a node to the graph
func (g *AdjacencyDAG) AddNode(node *Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node == nil {
		return ErrNilNode
	}

	if _, exists := g.nodes[node.ID]; exists {
		return ErrNodeExists
	}

	g.nodes[node.ID] = node
	g.outgoing[node.ID] = make(map[string]*Edge)
	g.incoming[node.ID] = make(map[string]*Edge)

	if g.cache != nil {
		g.cache.InvalidateAll()
	}

	return nil
}

// GetNode retrieves a node by ID
func (g *AdjacencyDAG) GetNode(id string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	if !exists {
		return nil, ErrNodeNotFound
	}

	return node, nil
}

// HasNode checks if a node exists
func (g *AdjacencyDAG) HasNode(id string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.nodes[id]
	return exists
}

// NodeCount returns the number of nodes
func (g *AdjacencyDAG) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// AddEdge adds an edge (without cycle check)
func (g *AdjacencyDAG) AddEdge(edge *Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.addEdgeUnsafe(edge)
}

// addEdgeUnsafe adds edge without locking (internal use)
func (g *AdjacencyDAG) addEdgeUnsafe(edge *Edge) error {
	if edge == nil {
		return ErrNilEdge
	}

	// Validate nodes exist
	if _, exists := g.nodes[edge.FromID]; !exists {
		return ErrNodeNotFound
	}
	if _, exists := g.nodes[edge.ToID]; !exists {
		return ErrNodeNotFound
	}

	// Prevent self-loops
	if edge.FromID == edge.ToID {
		return ErrSelfLoop
	}

	// Add edge
	g.outgoing[edge.FromID][edge.ToID] = edge
	g.incoming[edge.ToID][edge.FromID] = edge

	if g.cache != nil {
		g.cache.InvalidateIndegree(edge.ToID)
		g.cache.InvalidateReady()
	}

	return nil
}

// AddEdgeWithCycleCheck adds an edge and checks for cycles
func (g *AdjacencyDAG) AddEdgeWithCycleCheck(edge *Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Temporarily add edge
	if err := g.addEdgeUnsafe(edge); err != nil {
		return err
	}

	// Check for cycle
	cycle, err := g.detectCycleUnsafe()
	if err != nil {
		return err
	}

	if len(cycle) > 0 {
		// Remove edge and return cycle error
		delete(g.outgoing[edge.FromID], edge.ToID)
		delete(g.incoming[edge.ToID], edge.FromID)
		return &CycleError{Cycle: cycle}
	}

	return nil
}

// GetIndegree returns the indegree of a node
func (g *AdjacencyDAG) GetIndegree(nodeID string) int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cache != nil {
		if indegree, ok := g.cache.GetIndegree(nodeID); ok {
			return indegree
		}
	}

	indegree := len(g.incoming[nodeID])

	if g.cache != nil {
		g.cache.SetIndegree(nodeID, indegree)
	}

	return indegree
}

// GetTransitiveDependencies returns all transitive dependencies up to depth
func (g *AdjacencyDAG) GetTransitiveDependencies(nodeID string, depth int) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}

	visited := make(map[string]bool)
	result := []string{}

	g.bfsTransitive(nodeID, depth, visited, &result)

	return result, nil
}

// bfsTransitive performs BFS traversal for transitive dependencies
func (g *AdjacencyDAG) bfsTransitive(startID string, maxDepth int, visited map[string]bool, result *[]string) {
	type queueItem struct {
		nodeID string
		depth  int
	}

	queue := []queueItem{{nodeID: startID, depth: 0}}
	visited[startID] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth && maxDepth > 0 {
			continue
		}

		// Get all incoming edges (dependencies)
		for fromID := range g.incoming[item.nodeID] {
			if !visited[fromID] {
				visited[fromID] = true
				*result = append(*result, fromID)
				queue = append(queue, queueItem{nodeID: fromID, depth: item.depth + 1})
			}
		}
	}
}

// RemoveNode removes a node and its associated edges
func (g *AdjacencyDAG) RemoveNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; !exists {
		return ErrNodeNotFound
	}

	// Remove outgoing edges
	for toID := range g.outgoing[id] {
		delete(g.incoming[toID], id)
		if g.cache != nil {
			g.cache.InvalidateIndegree(toID)
		}
	}
	delete(g.outgoing, id)

	// Remove incoming edges
	for fromID := range g.incoming[id] {
		delete(g.outgoing[fromID], id)
	}
	delete(g.incoming, id)

	// Remove node
	delete(g.nodes, id)

	if g.cache != nil {
		g.cache.InvalidateAll()
	}

	return nil
}

// EdgeCount returns the number of edges
func (g *AdjacencyDAG) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, edges := range g.outgoing {
		count += len(edges)
	}
	return count
}

// GetEdges returns all edges for a node
func (g *AdjacencyDAG) GetEdges(nodeID string) ([]*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}
	edges := []*Edge{}
	for _, e := range g.outgoing[nodeID] {
		edges = append(edges, e)
	}
	for _, e := range g.incoming[nodeID] {
		edges = append(edges, e)
	}
	return edges, nil
}

// GetOutgoingEdges returns outgoing edges for a node
func (g *AdjacencyDAG) GetOutgoingEdges(nodeID string) ([]*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}
	edges := []*Edge{}
	for _, e := range g.outgoing[nodeID] {
		edges = append(edges, e)
	}
	return edges, nil
}

// GetIncomingEdges returns incoming edges for a node
func (g *AdjacencyDAG) GetIncomingEdges(nodeID string) ([]*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}
	edges := []*Edge{}
	for _, e := range g.incoming[nodeID] {
		edges = append(edges, e)
	}
	return edges, nil
}

// RemoveEdge removes an edge
func (g *AdjacencyDAG) RemoveEdge(fromID, toID string, depType DependencyType) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[fromID]; !exists {
		return ErrNodeNotFound
	}
	if _, exists := g.nodes[toID]; !exists {
		return ErrNodeNotFound
	}

	if edge, exists := g.outgoing[fromID][toID]; exists {
		if depType == "" || edge.Type == depType {
			delete(g.outgoing[fromID], toID)
			delete(g.incoming[toID], fromID)
			if g.cache != nil {
				g.cache.InvalidateIndegree(toID)
				g.cache.InvalidateReady()
			}
			return nil
		}
	}

	return nil
}

// GetSuccessors returns successor node IDs
func (g *AdjacencyDAG) GetSuccessors(nodeID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}
	ids := []string{}
	for id := range g.outgoing[nodeID] {
		ids = append(ids, id)
	}
	return ids, nil
}

// GetPredecessors returns predecessor node IDs
func (g *AdjacencyDAG) GetPredecessors(nodeID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}
	ids := []string{}
	for id := range g.incoming[nodeID] {
		ids = append(ids, id)
	}
	return ids, nil
}

// GetOutdegree returns outdegree of a node
func (g *AdjacencyDAG) GetOutdegree(nodeID string) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.outgoing[nodeID])
}

// GetAllNodes returns all nodes in the graph
func (g *AdjacencyDAG) GetAllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	nodes := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// GetAllEdges returns all edges in the graph
func (g *AdjacencyDAG) GetAllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edges := []*Edge{}
	for _, out := range g.outgoing {
		for _, e := range out {
			edges = append(edges, e)
		}
	}
	return edges
}

// IsReachable checks if toID is reachable from fromID
func (g *AdjacencyDAG) IsReachable(fromID, toID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	queue := []string{fromID}
	visited[fromID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr == toID {
			return true
		}

		for next := range g.outgoing[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	return false
}
