package graph

import (
	"fmt"
	"sync"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
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

	// Persistence & Session
	store      dolt.Store
	actor      string
	agentModel string
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

// SetSession updates the session metadata for auditing
func (g *AdjacencyDAG) SetSession(actor, model string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.actor = actor
	g.agentModel = model
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

// SetNodeStatus updates the status of a node and invalidates relevant caches
func (g *AdjacencyDAG) SetNodeStatus(id string, status IssueStatus) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[id]
	if !exists {
		return ErrNodeNotFound
	}

	if node.Status == status {
		return nil
	}

	if g.store != nil {
		actor := g.actor
		if actor == "" {
			actor = "unknown"
		}

		query := "UPDATE issues SET status = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?"
		_, err := g.store.Exec(query, string(status), time.Now(), actor, g.agentModel, id)
		if err != nil {
			return fmt.Errorf("failed to update issue status in DB: %w", err)
		}

		err = g.store.LogEvent(id, "status_change", actor, g.agentModel, node.Status, status)
		if err != nil {
			// We log but don't fail the operation if only audit log fails?
			// Actually, plan says "log audit events". If it fails, maybe we should know.
			// Given it's local DB, let's be strict.
			return fmt.Errorf("failed to log status change event: %w", err)
		}
	}

	node.Status = status
	node.UpdatedAt = time.Now()
	if g.cache != nil {
		g.cache.MarkDirty(id)
		// Propagate priority change as status can affect inheritance (e.g. closing a task)
		g.cache.PropagatePriorityChange(id)

		// If status changed to closed, successors' blocking indegrees might change
		if status != StatusOpen {
			for toID, edge := range g.outgoing[id] {
				if edge.Type.IsBlockingType() {
					g.cache.InvalidateIndegree(toID)
				}
			}
		}
	}

	return nil
}

// SetNodePriority updates the priority of a node and propagates the change
func (g *AdjacencyDAG) SetNodePriority(id string, priority Priority) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[id]
	if !exists {
		return ErrNodeNotFound
	}

	if node.Priority == priority {
		return nil
	}

	if g.store != nil {
		actor := g.actor
		if actor == "" {
			actor = "unknown"
		}

		query := "UPDATE issues SET priority = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?"
		_, err := g.store.Exec(query, int(priority), time.Now(), actor, g.agentModel, id)
		if err != nil {
			return fmt.Errorf("failed to update issue priority in DB: %w", err)
		}

		err = g.store.LogEvent(id, "priority_change", actor, g.agentModel, int(node.Priority), int(priority))
		if err != nil {
			return fmt.Errorf("failed to log priority change event: %w", err)
		}
	}

	node.Priority = priority
	node.UpdatedAt = time.Now()
	if g.cache != nil {
		g.cache.PropagatePriorityChange(id)
	}

	return nil
}

// SetPriorityInheritanceDepth updates the depth limit for priority inheritance in the cache
func (g *AdjacencyDAG) SetPriorityInheritanceDepth(depth int) {
	if g.cache != nil {
		g.cache.SetPriorityInheritanceDepth(depth)
	}
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
		// When adding an edge A -> B:
		// 1. B's indegree and blocking indegree change
		g.cache.InvalidateIndegree(edge.ToID)
		// 2. A and A's predecessors' effective priority might change (inheritance from B)
		g.cache.PropagatePriorityChange(edge.FromID)
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

	type queueItem struct {
		nodeID string
		depth  int
	}

	queue := []queueItem{{nodeID: nodeID, depth: 0}}
	visited[nodeID] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= depth && depth > 0 {
			continue
		}

		// Get all incoming edges (dependencies)
		for fromID := range g.incoming[item.nodeID] {
			if !visited[fromID] {
				visited[fromID] = true
				result = append(result, fromID)
				queue = append(queue, queueItem{nodeID: fromID, depth: item.depth + 1})
			}
		}
	}

	return result, nil
}

// GetTransitiveBlockers returns all open issues that block the given node
func (g *AdjacencyDAG) GetTransitiveBlockers(nodeID string, depth int) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}

	visited := make(map[string]bool)
	result := []string{}

	type queueItem struct {
		nodeID string
		depth  int
	}

	queue := []queueItem{{nodeID: nodeID, depth: 0}}
	visited[nodeID] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= depth && depth > 0 {
			continue
		}

		// Get all incoming blocking edges
		for fromID, edge := range g.incoming[item.nodeID] {
			if !edge.Type.IsBlockingType() {
				continue
			}

			if !visited[fromID] {
				visited[fromID] = true
				fromNode := g.nodes[fromID]
				if fromNode.Status == StatusOpen {
					result = append(result, fromID)
					queue = append(queue, queueItem{nodeID: fromID, depth: item.depth + 1})
				}
			}
		}
	}

	return result, nil
}

// RemoveNode removes a node and its associated edges
func (g *AdjacencyDAG) RemoveNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; !exists {
		return ErrNodeNotFound
	}

	// Persist: tombstone in DB, delete dependencies, log audit event
	if g.store != nil {
		actor := g.actor
		if actor == "" {
			actor = "unknown"
		}

		// 1. Record deletion entry
		_, err := g.store.Exec(
			"INSERT INTO deletions (id, reason, actor, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)",
			id, "remove_node", actor, actor, actor, g.agentModel,
		)
		if err != nil {
			return fmt.Errorf("failed to record deletion for %s: %w", id, err)
		}

		// 2. Tombstone the issue
		_, err = g.store.Exec(
			"UPDATE issues SET status = 'tombstone', updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?",
			time.Now(), actor, g.agentModel, id,
		)
		if err != nil {
			return fmt.Errorf("failed to tombstone issue %s: %w", id, err)
		}

		// 3. Remove all dependencies involving this node
		_, err = g.store.Exec("DELETE FROM dependencies WHERE from_id = ? OR to_id = ?", id, id)
		if err != nil {
			return fmt.Errorf("failed to delete dependencies for %s: %w", id, err)
		}

		// 4. Audit log
		_ = g.store.LogEvent(id, "node_removed", actor, g.agentModel, nil, nil)
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
			// Persist: delete dependency row from DB
			if g.store != nil {
				actor := g.actor
				if actor == "" {
					actor = "unknown"
				}

				query := "DELETE FROM dependencies WHERE from_id = ? AND to_id = ?"
				args := []any{fromID, toID}
				if depType != "" {
					query += " AND type = ?"
					args = append(args, string(depType))
				}

				_, err := g.store.Exec(query, args...)
				if err != nil {
					return fmt.Errorf("failed to delete dependency %s->%s from DB: %w", fromID, toID, err)
				}

				_ = g.store.LogEvent(fromID, "edge_removed", actor, g.agentModel,
					map[string]string{"to_id": toID, "type": string(edge.Type)}, nil)
			}

			delete(g.outgoing[fromID], toID)
			delete(g.incoming[toID], fromID)
			if g.cache != nil {
				g.cache.InvalidateIndegree(toID)
				// Removing a blocking edge might change effective priority of fromID
				g.cache.PropagatePriorityChange(fromID)
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

	return g.isReachableUnsafe(fromID, toID)
}

// isReachableUnsafe checks if toID is reachable from fromID without locking
func (g *AdjacencyDAG) isReachableUnsafe(fromID, toID string) bool {
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

// GetBlockingPath returns a path from fromID to toID using only blocking edges
func (g *AdjacencyDAG) GetBlockingPath(fromID, toID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.HasNode(fromID) || !g.HasNode(toID) {
		return nil, ErrNodeNotFound
	}

	visited := make(map[string]bool)
	parent := make(map[string]string)
	queue := []string{fromID}
	visited[fromID] = true

	found := false
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr == toID {
			found = true
			break
		}

		for next, edge := range g.outgoing[curr] {
			if edge.Type.IsBlockingType() && !visited[next] {
				visited[next] = true
				parent[next] = curr
				queue = append(queue, next)
			}
		}
	}

	if !found {
		return nil, nil
	}

	// Reconstruct path
	path := []string{toID}
	curr := toID
	for curr != fromID {
		curr = parent[curr]
		path = append([]string{curr}, path...)
	}

	return path, nil
}

// TransitiveReduction removes redundant edges from the DAG
func (g *AdjacencyDAG) TransitiveReduction() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Identify redundant edges
	redundant := []*Edge{}
	for fromID, neighbors := range g.outgoing {
		for toID, edge := range neighbors {
			// Check if there's another path from fromID to toID
			// Temporarily remove edge
			delete(g.outgoing[fromID], toID)
			delete(g.incoming[toID], fromID)

			if g.isReachableUnsafe(fromID, toID) {
				redundant = append(redundant, edge)
			}

			// Restore edge
			g.outgoing[fromID][toID] = edge
			g.incoming[toID][fromID] = edge
		}
	}

	// Remove redundant edges
	for _, edge := range redundant {
		delete(g.outgoing[edge.FromID], edge.ToID)
		delete(g.incoming[edge.ToID], edge.FromID)
		if g.cache != nil {
			g.cache.InvalidateIndegree(edge.ToID)
		}
	}

	if len(redundant) > 0 && g.cache != nil {
		g.cache.InvalidateReady()
	}

	return nil
}
