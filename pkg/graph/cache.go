package graph

import (
	"sync"
	"time"
)

// GraphCache caches computed graph properties
type GraphCache struct {
	mu sync.RWMutex

	// Cached indegrees
	indegreeMap   map[string]int
	indegreeValid map[string]bool

	// Cached ready list
	readyList       []*ReadyTask
	readyListValid  bool
	readyListExpiry time.Time
	readyListTTL    time.Duration

	// Reference to DAG
	dag *AdjacencyDAG
}

// NewGraphCache creates a new cache
func NewGraphCache(dag *AdjacencyDAG) *GraphCache {
	return &GraphCache{
		indegreeMap:   make(map[string]int),
		indegreeValid: make(map[string]bool),
		readyListTTL:  1 * time.Minute,
		dag:           dag,
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

// InvalidateIndegree invalidates cached indegree
func (c *GraphCache) InvalidateIndegree(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.indegreeValid[nodeID] = false
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
	c.readyListValid = false
}
