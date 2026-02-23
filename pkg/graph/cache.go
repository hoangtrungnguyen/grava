package graph

import (
	"sync"
	"time"
)

// GraphCache caches computed graph properties
type GraphCache struct {
	mu sync.RWMutex

	// Cached indegrees (total)
	indegreeMap   map[string]int
	indegreeValid map[string]bool

	// Cached blocking indegrees (only blocking edges from open nodes)
	blockingIndegreeMap   map[string]int
	blockingIndegreeValid map[string]bool

	// Cached effective priorities
	priorityMap   map[string]Priority
	priorityValid map[string]bool

	// Cached ready list
	readyList       []*ReadyTask
	readyListValid  bool
	readyListExpiry time.Time
	readyListTTL    time.Duration

	// Track dirty nodes for incremental updates
	dirtyNodes map[string]bool

	// Settings
	priorityInheritanceDepth int

	// Reference to DAG
	dag *AdjacencyDAG
}

// NewGraphCache creates a new cache
func NewGraphCache(dag *AdjacencyDAG) *GraphCache {
	return &GraphCache{
		indegreeMap:              make(map[string]int),
		indegreeValid:            make(map[string]bool),
		blockingIndegreeMap:      make(map[string]int),
		blockingIndegreeValid:    make(map[string]bool),
		priorityMap:              make(map[string]Priority),
		priorityValid:            make(map[string]bool),
		dirtyNodes:               make(map[string]bool),
		readyListTTL:             1 * time.Minute,
		priorityInheritanceDepth: 10, // Default matching ReadyEngine
		dag:                      dag,
	}
}

// GetIndegree returns cached indegree
func (c *GraphCache) GetIndegree(nodeID string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.indegreeValid[nodeID] {
		return 0, false
	}

	return c.indegreeMap[nodeID], true
}

// SetIndegree caches indegree
func (c *GraphCache) SetIndegree(nodeID string, indegree int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.indegreeMap[nodeID] = indegree
	c.indegreeValid[nodeID] = true
}

// GetBlockingIndegree returns cached blocking indegree
func (c *GraphCache) GetBlockingIndegree(nodeID string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.blockingIndegreeValid[nodeID] {
		return 0, false
	}

	return c.blockingIndegreeMap[nodeID], true
}

// SetBlockingIndegree caches blocking indegree
func (c *GraphCache) SetBlockingIndegree(nodeID string, indegree int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.blockingIndegreeMap[nodeID] = indegree
	c.blockingIndegreeValid[nodeID] = true
}

// GetPriority returns cached effective priority
func (c *GraphCache) GetPriority(nodeID string) (Priority, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.priorityValid[nodeID] {
		return 0, false
	}

	return c.priorityMap[nodeID], true
}

// SetPriority caches effective priority
func (c *GraphCache) SetPriority(nodeID string, priority Priority) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.priorityMap[nodeID] = priority
	c.priorityValid[nodeID] = true
}

// MarkDirty marks a node as dirty, requiring recomputation of its properties
func (c *GraphCache) MarkDirty(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dirtyNodes[nodeID] = true
	c.indegreeValid[nodeID] = false
	c.blockingIndegreeValid[nodeID] = false
	c.priorityValid[nodeID] = false
	c.readyListValid = false
}

// InvalidateIndegree invalidates cached indegree
func (c *GraphCache) InvalidateIndegree(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.indegreeValid[nodeID] = false
	c.blockingIndegreeValid[nodeID] = false
	c.readyListValid = false
}

// InvalidateReady invalidates ready list cache
func (c *GraphCache) InvalidateReady() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.readyListValid = false
}

// InvalidateAll clears all caches
func (c *GraphCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.indegreeValid = make(map[string]bool)
	c.blockingIndegreeValid = make(map[string]bool)
	c.priorityValid = make(map[string]bool)
	c.dirtyNodes = make(map[string]bool)
	c.readyListValid = false
}

// PropagatePriorityChange proactively updates the effective priority of a node and its predecessors
func (c *GraphCache) PropagatePriorityChange(nodeID string) {
	// Note: Caller is expected to hold c.dag.mu.Lock()
	c.propagatePriorityChangeUnsafe(nodeID)
}

func (c *GraphCache) propagatePriorityChangeUnsafe(nodeID string) {
	_, exists := c.dag.nodes[nodeID]
	if !exists {
		return
	}

	// Calculate new effective priority for this node
	newPriority := c.calculateEffectivePriorityUnsafe(nodeID)

	c.mu.RLock()
	oldPriority, valid := c.priorityMap[nodeID]
	isValid := c.priorityValid[nodeID]
	c.mu.RUnlock()

	if isValid && valid && oldPriority == newPriority {
		// Priority hasn't changed, stop propagation
		return
	}

	// Update cache
	c.SetPriority(nodeID, newPriority)
	c.InvalidateReady()

	// Propagate to predecessors (nodes that block this node)
	for predID, edge := range c.dag.incoming[nodeID] {
		if edge.Type.IsBlockingType() {
			c.propagatePriorityChangeUnsafe(predID)
		}
	}
}

// SetPriorityInheritanceDepth updates the depth limit for priority inheritance
func (c *GraphCache) SetPriorityInheritanceDepth(depth int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.priorityInheritanceDepth == depth {
		return
	}

	c.priorityInheritanceDepth = depth
	// Must invalidate priority cache when depth limit changes
	c.priorityValid = make(map[string]bool)
	c.readyListValid = false
}

// calculateEffectivePriority is a helper to compute effective priority including inheritance
func (c *GraphCache) calculateEffectivePriorityUnsafe(nodeID string) Priority {
	return c.calculateEffectivePriorityWithDepth(nodeID, 0, make(map[string]bool))
}

func (c *GraphCache) calculateEffectivePriorityWithDepth(nodeID string, depth int, visited map[string]bool) Priority {
	node := c.dag.nodes[nodeID]
	currentMin := node.Priority

	if visited[nodeID] {
		return currentMin
	}
	visited[nodeID] = true
	defer delete(visited, nodeID)

	// If we reached the limit, stop inheritance
	if depth >= c.priorityInheritanceDepth {
		return currentMin
	}

	// Check immediate successors
	for toID, edge := range c.dag.outgoing[nodeID] {
		if !edge.Type.IsBlockingType() {
			continue
		}

		successor, exists := c.dag.nodes[toID]
		if !exists || successor.Status != StatusOpen {
			continue
		}

		// Recurse to find successor's effective priority
		successorEff := c.calculateEffectivePriorityWithDepth(toID, depth+1, visited)

		if successorEff < currentMin {
			currentMin = successorEff
		}
	}

	return currentMin
}
