# Graph Data Structure Implementation Plan

**Created:** 2026-02-20
**Epic:** Epic 2: Graph Mechanics and Blocked Issue Tracking
**Language:** Go (Golang)
**Target Package:** `pkg/graph`

---

## Executive Summary

This plan outlines the implementation of a production-ready graph data structure in Go to support Epic 2's dependency graph mechanics. The implementation will include:

- **Core DAG structure** with cycle prevention
- **Kahn's Algorithm** for topological sorting and cycle detection
- **Priority queue** for the Ready Engine
- **Gate system** for external dependency management
- **Caching layer** for performance optimization
- **Comprehensive testing** and benchmarking

**Performance Goals:**
- Ready Engine: <10ms for 10k nodes
- Cycle Detection: <100ms for 10k nodes
- Memory: <50MB for 10k nodes with 30k edges

---

## 1. Package Structure

```
pkg/graph/
├── graph.go              # Core graph types and interfaces
├── graph_test.go         # Core graph tests
├── dag.go                # DAG-specific operations
├── dag_test.go           # DAG tests
├── topology.go           # Topological sort (Kahn's Algorithm)
├── topology_test.go      # Topology tests
├── cycle.go              # Cycle detection algorithms
├── cycle_test.go         # Cycle detection tests
├── ready_engine.go       # Ready Engine implementation
├── ready_engine_test.go  # Ready Engine tests
├── priority_queue.go     # Priority queue for Ready Engine
├── priority_queue_test.go
├── gates.go              # Gate evaluation system
├── gates_test.go         # Gate tests
├── cache.go              # Graph caching layer
├── cache_test.go         # Cache tests
├── traversal.go          # Graph traversal utilities (BFS, DFS)
├── traversal_test.go     # Traversal tests
├── types.go              # Common types and constants
├── errors.go             # Error definitions
└── benchmark_test.go     # Performance benchmarks
```

---

## 2. Core Data Structures

### 2.1 Graph Types (`types.go`)

```go
package graph

import "time"

// DependencyType represents the semantic type of a dependency edge
type DependencyType string

const (
	// Blocking Types (Hard Dependencies)
	DependencyBlocks     DependencyType = "blocks"       // from_id blocks to_id
	DependencyBlockedBy  DependencyType = "blocked-by"   // Inverse of blocks

	// Soft Dependencies
	DependencyWaitsFor   DependencyType = "waits-for"    // Soft dependency
	DependencyDependsOn  DependencyType = "depends-on"   // General dependency

	// Hierarchical
	DependencyParentChild DependencyType = "parent-child" // Hierarchical decomposition
	DependencyChildOf     DependencyType = "child-of"     // Inverse
	DependencySubtaskOf   DependencyType = "subtask-of"   // Task breakdown
	DependencyHasSubtask  DependencyType = "has-subtask"  // Inverse

	// Semantic Relationships
	DependencyDuplicates    DependencyType = "duplicates"     // Marks as duplicate
	DependencyDuplicatedBy  DependencyType = "duplicated-by"  // Inverse
	DependencyRelatesTo     DependencyType = "relates-to"     // General association
	DependencySupersedes    DependencyType = "supersedes"     // Replaces older task
	DependencySupersededBy  DependencyType = "superseded-by"  // Inverse

	// Ordering
	DependencyFollows  DependencyType = "follows"  // Sequencing hint
	DependencyPrecedes DependencyType = "precedes" // Inverse

	// Technical
	DependencyCausedBy DependencyType = "caused-by" // Bug causation
	DependencyCauses   DependencyType = "causes"    // Inverse
	DependencyFixes    DependencyType = "fixes"     // Fix relationship
	DependencyFixedBy  DependencyType = "fixed-by"  // Inverse
)

// IsBlockingType returns true if this dependency type blocks execution
func (dt DependencyType) IsBlockingType() bool {
	return dt == DependencyBlocks || dt == DependencyBlockedBy
}

// IsSoftDependency returns true if this is a soft dependency
func (dt DependencyType) IsSoftDependency() bool {
	return dt == DependencyWaitsFor || dt == DependencyDependsOn
}

// IssueStatus represents the status of an issue
type IssueStatus string

const (
	StatusOpen       IssueStatus = "open"
	StatusInProgress IssueStatus = "in_progress"
	StatusBlocked    IssueStatus = "blocked"
	StatusClosed     IssueStatus = "closed"
	StatusTombstone  IssueStatus = "tombstone"
	StatusDeferred   IssueStatus = "deferred"
	StatusPinned     IssueStatus = "pinned"
)

// Priority represents task priority (0=Critical to 4=Backlog)
type Priority int

const (
	PriorityCritical Priority = 0
	PriorityHigh     Priority = 1
	PriorityMedium   Priority = 2
	PriorityLow      Priority = 3
	PriorityBacklog  Priority = 4
)

// Node represents a graph node (issue)
type Node struct {
	ID          string
	Status      IssueStatus
	Priority    Priority
	CreatedAt   time.Time
	AwaitType   string // Gate type: "gh:pr", "timer", "human", empty for none
	AwaitID     string // Gate identifier
	Metadata    map[string]interface{}
}

// Edge represents a directed edge between two nodes
type Edge struct {
	FromID   string
	ToID     string
	Type     DependencyType
	Metadata map[string]interface{}
}

// ReadyTask represents a task ready for execution
type ReadyTask struct {
	Node              *Node
	EffectivePriority Priority // After priority inheritance
	Age               time.Duration
	PriorityBoosted   bool
}
```

### 2.2 Graph Interface (`graph.go`)

```go
package graph

// Graph represents a directed graph
type Graph interface {
	// Node operations
	AddNode(node *Node) error
	GetNode(id string) (*Node, error)
	HasNode(id string) bool
	RemoveNode(id string) error
	NodeCount() int

	// Edge operations
	AddEdge(edge *Edge) error
	RemoveEdge(fromID, toID string, depType DependencyType) error
	GetEdges(nodeID string) ([]*Edge, error)
	GetOutgoingEdges(nodeID string) ([]*Edge, error)
	GetIncomingEdges(nodeID string) ([]*Edge, error)
	EdgeCount() int

	// Graph queries
	GetSuccessors(nodeID string) ([]string, error)
	GetPredecessors(nodeID string) ([]string, error)
	GetIndegree(nodeID string) int
	GetOutdegree(nodeID string) int

	// Traversal
	GetAllNodes() []*Node
	GetAllEdges() []*Edge
}

// DAG represents a Directed Acyclic Graph
type DAG interface {
	Graph

	// DAG-specific operations
	AddEdgeWithCycleCheck(edge *Edge) error
	DetectCycle() ([]string, error)
	TopologicalSort() ([]string, error)
	GetTransitiveDependencies(nodeID string, depth int) ([]string, error)
	IsReachable(fromID, toID string) bool
}
```

### 2.3 Adjacency List Implementation (`dag.go`)

```go
package graph

import (
	"fmt"
	"sync"
)

// AdjacencyDAG implements DAG using adjacency lists
type AdjacencyDAG struct {
	mu sync.RWMutex

	// Core data structures
	nodes map[string]*Node                    // nodeID -> Node
	outgoing map[string]map[string]*Edge      // fromID -> toID -> Edge
	incoming map[string]map[string]*Edge      // toID -> fromID -> Edge

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
```

---

## 3. Topological Sort & Cycle Detection

### 3.1 Kahn's Algorithm (`topology.go`)

```go
package graph

import "fmt"

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
```

---

## 4. Priority Queue for Ready Engine

### 4.1 Priority Queue Implementation (`priority_queue.go`)

```go
package graph

import (
	"container/heap"
)

// PriorityQueue implements heap.Interface for ReadyTask
type PriorityQueue []*ReadyTask

func (pq PriorityQueue) Len() int { return len(pq) }

// Less defines min-heap ordering: lower priority number = higher priority
// For ties, older tasks (longer age) come first
func (pq PriorityQueue) Less(i, j int) bool {
	// Compare effective priority (lower number = higher priority)
	if pq[i].EffectivePriority != pq[j].EffectivePriority {
		return pq[i].EffectivePriority < pq[j].EffectivePriority
	}

	// Tie-breaker: older tasks first
	return pq[i].Node.CreatedAt.Before(pq[j].Node.CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*ReadyTask)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // Avoid memory leak
	*pq = old[0 : n-1]
	return item
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(tasks []*ReadyTask) *PriorityQueue {
	pq := make(PriorityQueue, len(tasks))
	copy(pq, tasks)
	heap.Init(&pq)
	return &pq
}

// PushTask adds a task to the priority queue
func (pq *PriorityQueue) PushTask(task *ReadyTask) {
	heap.Push(pq, task)
}

// PopTask removes and returns the highest priority task
func (pq *PriorityQueue) PopTask() *ReadyTask {
	if pq.Len() == 0 {
		return nil
	}
	return heap.Pop(pq).(*ReadyTask)
}
```

---

## 5. Ready Engine

### 5.1 Ready Engine Implementation (`ready_engine.go`)

```go
package graph

import (
	"fmt"
	"time"
)

// ReadyEngineConfig configures the Ready Engine
type ReadyEngineConfig struct {
	EnablePriorityInheritance bool
	PriorityInheritanceDepth  int           // Max depth for priority inheritance (0 = unlimited)
	AgingThreshold            time.Duration // Duration before priority boost
	AgingBoost                Priority      // How much to boost (1 level)
	GateEvaluator             GateEvaluator
}

// DefaultReadyEngineConfig returns default configuration
func DefaultReadyEngineConfig() *ReadyEngineConfig {
	return &ReadyEngineConfig{
		EnablePriorityInheritance: true,
		PriorityInheritanceDepth:  10,
		AgingThreshold:            7 * 24 * time.Hour, // 7 days
		AgingBoost:                1,
		GateEvaluator:             NewDefaultGateEvaluator(),
	}
}

// ReadyEngine computes actionable tasks
type ReadyEngine struct {
	dag    *AdjacencyDAG
	config *ReadyEngineConfig
}

// NewReadyEngine creates a new Ready Engine
func NewReadyEngine(dag *AdjacencyDAG, config *ReadyEngineConfig) *ReadyEngine {
	if config == nil {
		config = DefaultReadyEngineConfig()
	}
	return &ReadyEngine{
		dag:    dag,
		config: config,
	}
}

// ComputeReady returns ready tasks sorted by priority
func (re *ReadyEngine) ComputeReady(limit int) ([]*ReadyTask, error) {
	re.dag.mu.RLock()
	defer re.dag.mu.RUnlock()

	now := time.Now()
	readyTasks := []*ReadyTask{}

	// Step 1: Find candidates (open, indegree == 0)
	for nodeID, node := range re.dag.nodes {
		// Only consider open tasks
		if node.Status != StatusOpen {
			continue
		}

		// Check indegree (only blocking dependencies)
		blockingIndegree := re.getBlockingIndegree(nodeID)
		if blockingIndegree > 0 {
			continue
		}

		// Check gate conditions
		if node.AwaitType != "" {
			gateOpen, err := re.config.GateEvaluator.IsGateOpen(node)
			if err != nil {
				return nil, fmt.Errorf("gate evaluation error for %s: %w", nodeID, err)
			}
			if !gateOpen {
				continue // Gate is closed, skip this task
			}
		}

		// Calculate effective priority
		effectivePriority := node.Priority
		priorityBoosted := false

		if re.config.EnablePriorityInheritance {
			inheritedPriority := re.calculateInheritedPriority(nodeID)
			if inheritedPriority < effectivePriority {
				effectivePriority = inheritedPriority
				priorityBoosted = true
			}
		}

		// Apply aging boost
		age := now.Sub(node.CreatedAt)
		if age >= re.config.AgingThreshold && effectivePriority > PriorityCritical {
			effectivePriority -= re.config.AgingBoost
			if effectivePriority < PriorityCritical {
				effectivePriority = PriorityCritical
			}
			priorityBoosted = true
		}

		readyTasks = append(readyTasks, &ReadyTask{
			Node:              node,
			EffectivePriority: effectivePriority,
			Age:               age,
			PriorityBoosted:   priorityBoosted,
		})
	}

	// Step 2: Sort by priority using priority queue
	pq := NewPriorityQueue(readyTasks)

	// Step 3: Extract top N tasks
	result := []*ReadyTask{}
	count := 0
	for pq.Len() > 0 && (limit == 0 || count < limit) {
		task := pq.PopTask()
		result = append(result, task)
		count++
	}

	return result, nil
}

// getBlockingIndegree calculates indegree considering only blocking edges
func (re *ReadyEngine) getBlockingIndegree(nodeID string) int {
	count := 0
	for _, edge := range re.dag.incoming[nodeID] {
		// Only count blocking dependencies from open nodes
		if edge.Type.IsBlockingType() {
			fromNode := re.dag.nodes[edge.FromID]
			if fromNode.Status == StatusOpen {
				count++
			}
		}
	}
	return count
}

// calculateInheritedPriority computes priority inheritance
func (re *ReadyEngine) calculateInheritedPriority(nodeID string) Priority {
	maxDepth := re.config.PriorityInheritanceDepth
	if maxDepth == 0 {
		maxDepth = 999999 // Unlimited
	}

	minPriority := re.dag.nodes[nodeID].Priority

	// BFS to find highest priority dependent
	type queueItem struct {
		id    string
		depth int
	}

	queue := []queueItem{{id: nodeID, depth: 0}}
	visited := make(map[string]bool)
	visited[nodeID] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth {
			continue
		}

		// Check all tasks blocked by this node
		for toID, edge := range re.dag.outgoing[item.id] {
			if !edge.Type.IsBlockingType() {
				continue
			}

			if visited[toID] {
				continue
			}
			visited[toID] = true

			dependentNode := re.dag.nodes[toID]
			if dependentNode.Status == StatusOpen && dependentNode.Priority < minPriority {
				minPriority = dependentNode.Priority
			}

			queue = append(queue, queueItem{id: toID, depth: item.depth + 1})
		}
	}

	return minPriority
}
```

---

## 6. Gate System

### 6.1 Gate Interface (`gates.go`)

```go
package graph

import (
	"fmt"
	"time"
)

// GateEvaluator evaluates gate conditions
type GateEvaluator interface {
	IsGateOpen(node *Node) (bool, error)
	GetGateStatus(node *Node) (string, error)
}

// DefaultGateEvaluator implements basic gate evaluation
type DefaultGateEvaluator struct {
	gitHubClient GitHubClient
}

// NewDefaultGateEvaluator creates default gate evaluator
func NewDefaultGateEvaluator() *DefaultGateEvaluator {
	return &DefaultGateEvaluator{
		gitHubClient: nil, // Injected later
	}
}

// IsGateOpen checks if a gate condition is met
func (ge *DefaultGateEvaluator) IsGateOpen(node *Node) (bool, error) {
	if node.AwaitType == "" {
		return true, nil // No gate
	}

	switch node.AwaitType {
	case "timer":
		return ge.evaluateTimerGate(node)
	case "gh:pr":
		return ge.evaluateGitHubPRGate(node)
	case "human":
		return ge.evaluateHumanGate(node)
	default:
		return false, fmt.Errorf("unknown gate type: %s", node.AwaitType)
	}
}

// evaluateTimerGate checks timer-based gates
func (ge *DefaultGateEvaluator) evaluateTimerGate(node *Node) (bool, error) {
	if node.AwaitID == "" {
		return false, fmt.Errorf("timer gate missing await_id")
	}

	// Parse timestamp (ISO 8601 or relative duration)
	targetTime, err := time.Parse(time.RFC3339, node.AwaitID)
	if err != nil {
		// Try parsing as relative duration (e.g., "+7d")
		// Implementation omitted for brevity
		return false, fmt.Errorf("invalid timer format: %w", err)
	}

	return time.Now().After(targetTime), nil
}

// evaluateGitHubPRGate checks GitHub PR status
func (ge *DefaultGateEvaluator) evaluateGitHubPRGate(node *Node) (bool, error) {
	if ge.gitHubClient == nil {
		// Graceful degradation: if GitHub API unavailable, gate is closed
		return false, nil
	}

	// Parse await_id: "owner/repo/pulls/123"
	// Query GitHub API for PR status
	// Check if merged_at != nil
	// Implementation omitted for brevity

	return false, fmt.Errorf("GitHub PR gate not yet implemented")
}

// evaluateHumanGate checks for human approval
func (ge *DefaultGateEvaluator) evaluateHumanGate(node *Node) (bool, error) {
	// Check events table for approval event
	// Implementation requires database query
	// For now, return false (needs approval)
	return false, nil
}

// GitHubClient interface for GitHub API
type GitHubClient interface {
	IsPRMerged(owner, repo string, prNumber int) (bool, error)
}
```

---

## 7. Caching Layer

### 7.1 Graph Cache (`cache.go`)

```go
package graph

import (
	"sync"
	"time"
)

// GraphCache caches computed graph properties
type GraphCache struct {
	mu sync.RWMutex

	// Cached indegrees
	indegreeMap map[string]int
	indegreeValid map[string]bool

	// Cached ready list
	readyList []*ReadyTask
	readyListValid bool
	readyListExpiry time.Time
	readyListTTL time.Duration

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
```

---

## 8. Error Handling

### 8.1 Error Types (`errors.go`)

```go
package graph

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNilNode       = errors.New("node is nil")
	ErrNilEdge       = errors.New("edge is nil")
	ErrNodeNotFound  = errors.New("node not found")
	ErrNodeExists    = errors.New("node already exists")
	ErrSelfLoop      = errors.New("self-loops are not allowed")
	ErrCycleDetected = errors.New("cycle detected in graph")
)

// CycleError represents a cycle in the graph
type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("cycle detected: %s", strings.Join(e.Cycle, " -> "))
}

func (e *CycleError) Is(target error) bool {
	return target == ErrCycleDetected
}
```

---

## 9. Testing Strategy

### 9.1 Unit Tests

**Test Coverage Requirements: >90%**

#### Graph Tests (`graph_test.go`)
- Node addition, retrieval, removal
- Edge addition, retrieval, removal
- Indegree/outdegree calculations
- Concurrent access (race detector)

#### DAG Tests (`dag_test.go`)
- Cycle detection (various cycle patterns)
- Edge addition with cycle check
- Transitive dependency calculation
- Reachability checks

#### Topology Tests (`topology_test.go`)
- Kahn's algorithm correctness
- Handling disconnected components
- Empty graph
- Single node
- Complex DAGs

#### Ready Engine Tests (`ready_engine_test.go`)
- Basic ready task computation
- Priority sorting
- Priority inheritance
- Aging mechanism
- Gate filtering
- Soft dependencies (waits-for)

#### Priority Queue Tests (`priority_queue_test.go`)
- Heap property maintenance
- Correct ordering
- Push/pop operations
- Edge cases (empty queue)

### 9.2 Integration Tests

#### Scenario Tests
- Complex multi-level blocking chains
- Priority inversion scenarios
- Gate state transitions
- Large graph operations (10k+ nodes)

### 9.3 Benchmark Tests (`benchmark_test.go`)

```go
// Example benchmark structure
func BenchmarkReadyEngine_10K(b *testing.B) {
	dag := createLargeDAG(10000, 30000) // 10k nodes, 30k edges
	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ComputeReady(100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCycleDetection_10K(b *testing.B) {
	dag := createLargeDAG(10000, 30000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dag.DetectCycle()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTopologicalSort_10K(b *testing.B) {
	dag := createLargeDAG(10000, 30000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dag.TopologicalSort()
		if err != nil {
			b.Fatal(err)
		}
	}
}
```

**Performance Targets:**
- `BenchmarkReadyEngine_10K`: <10ms per operation
- `BenchmarkCycleDetection_10K`: <100ms per operation
- `BenchmarkTopologicalSort_10K`: <50ms per operation
- Memory: <50MB for 10k nodes graph

---

## 10. Integration with Existing Codebase

### 10.1 Database Integration

The graph will be loaded from the database on-demand:

```go
// pkg/cmd/ready.go (example)
func loadGraphFromDB(store dolt.Store) (*graph.AdjacencyDAG, error) {
	dag := graph.NewAdjacencyDAG(true) // Enable cache

	// Load all open issues
	rows, err := store.Query("SELECT id, status, priority, created_at, await_type, await_id FROM issues WHERE status = 'open'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var node graph.Node
		err := rows.Scan(&node.ID, &node.Status, &node.Priority, &node.CreatedAt, &node.AwaitType, &node.AwaitID)
		if err != nil {
			return nil, err
		}
		dag.AddNode(&node)
	}

	// Load all dependencies
	rows, err = store.Query("SELECT from_id, to_id, type FROM dependencies")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var edge graph.Edge
		err := rows.Scan(&edge.FromID, &edge.ToID, &edge.Type)
		if err != nil {
			return nil, err
		}
		dag.AddEdge(&edge)
	}

	return dag, nil
}
```

### 10.2 CLI Commands

New commands to implement:

```bash
# Ready Engine
grava ready                    # Show ready tasks
grava ready --limit 10         # Limit to 10 tasks
grava ready --priority 0       # Filter by priority
grava ready --show-inherited   # Show priority inheritance

# Graph Analysis
grava graph stats              # Show graph statistics
grava graph health             # Check graph health
grava graph visualize <id>     # Export DOT format
grava graph cycle              # Check for cycles

# Blocked Analysis
grava blocked                  # List blocked tasks
grava blocked --depth 3        # Show transitive blockers

# Dependency Queries
grava dep list <id>            # List dependencies
grava dep tree <id>            # Show dependency tree
grava dep blockers <id>        # What blocks this?
grava dep blocked-by <id>      # What does this block?
grava dep path <from> <to>     # Find blocking path

# Batch Operations
grava dep batch --file deps.json
grava dep remove <from> <to>
grava dep clear <id>
```

---

## 11. Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)
**Goal:** Basic graph structure and operations

- [ ] Create package structure
- [ ] Implement core types (`types.go`, `errors.go`)
- [ ] Implement `AdjacencyDAG` basic operations
- [ ] Write unit tests (>90% coverage)
- [ ] Benchmark basic operations

**Deliverables:**
- `pkg/graph/` package with core files
- Unit tests passing
- Basic benchmarks

### Phase 2: Algorithms (Week 2-3)
**Goal:** Topological sort and cycle detection

- [ ] Implement Kahn's Algorithm (`topology.go`)
- [ ] Implement cycle detection with DFS
- [ ] Add transitive dependency calculation
- [ ] Write comprehensive tests
- [ ] Benchmark with 10k nodes

**Deliverables:**
- Working topological sort
- Cycle detection with path reconstruction
- Performance meets <100ms target

### Phase 3: Ready Engine (Week 3-4)
**Goal:** Priority-based task selection

- [ ] Implement priority queue (`priority_queue.go`)
- [ ] Implement Ready Engine (`ready_engine.go`)
- [ ] Add priority inheritance
- [ ] Add aging mechanism
- [ ] Write integration tests
- [ ] Benchmark Ready Engine

**Deliverables:**
- Ready Engine meets <10ms target
- Priority inheritance working
- CLI command `grava ready`

### Phase 4: Gate System (Week 4-5)
**Goal:** External dependency management

- [ ] Implement gate interface (`gates.go`)
- [ ] Add timer gate evaluation
- [ ] Add human gate support
- [ ] Stub GitHub PR gate
- [ ] Write gate tests

**Deliverables:**
- Timer gates functional
- Human gates functional
- `grava gate` commands

### Phase 5: Caching & Optimization (Week 5-6)
**Goal:** Performance optimization

- [ ] Implement graph cache (`cache.go`)
- [ ] Add incremental update logic
- [ ] Optimize memory usage
- [ ] Profile and optimize hot paths
- [ ] Run large-scale benchmarks

**Deliverables:**
- Cache reduces Ready Engine time by 50%
- Memory usage <50MB for 10k nodes
- All performance targets met

### Phase 6: CLI Integration (Week 6-7)
**Goal:** User-facing commands

- [ ] Implement `grava ready` command
- [ ] Implement `grava blocked` command
- [ ] Implement `grava graph` commands
- [ ] Implement batch dependency operations
- [ ] Add visualization export (DOT format)

**Deliverables:**
- All CLI commands functional
- Documentation updated
- User guide written

### Phase 7: Testing & Documentation (Week 7-8)
**Goal:** Production readiness

- [ ] Integration test suite
- [ ] End-to-end tests
- [ ] Load testing (100k nodes)
- [ ] API documentation
- [ ] Architecture documentation
- [ ] Code review

**Deliverables:**
- Test coverage >90%
- All benchmarks passing
- Documentation complete
- Code reviewed and approved

---

## 12. Performance Considerations

### 12.1 Memory Management

**Estimated Memory Usage:**
```
Node: ~200 bytes (including pointers)
Edge: ~100 bytes

10,000 nodes: ~2 MB
30,000 edges: ~3 MB
Cache overhead: ~5 MB
Total: ~10 MB (well under 50 MB target)
```

**Optimization Strategies:**
- Use pointers for large structures
- Pool frequently allocated objects
- Clear slices after use to allow GC
- Use `sync.Pool` for temporary objects

### 12.2 Concurrency

**Read-Write Lock Strategy:**
- Use `sync.RWMutex` for graph operations
- Multiple concurrent reads allowed
- Exclusive lock for writes
- Cache operations are thread-safe

**Parallelization Opportunities:**
- Parallel BFS for large graphs
- Concurrent gate evaluation
- Batch dependency operations

### 12.3 Algorithm Complexity

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| AddNode | O(1) | O(1) |
| AddEdge | O(1) | O(1) |
| GetIndegree | O(1) cached, O(E) uncached | O(V) |
| TopologicalSort | O(V + E) | O(V) |
| CycleDetection | O(V + E) | O(V) |
| ReadyEngine | O(V + E) | O(V) |
| TransitiveDeps | O(V + E) with depth limit | O(V) |

---

## 13. Best Practices from Research

### 13.1 Go-Specific Patterns

1. **Use `container/heap` for Priority Queue**
   - Standard library, well-tested, efficient O(log n) operations
   - Source: [Go container/heap](https://pkg.go.dev/container/heap)

2. **Adjacency List for Sparse Graphs**
   - More memory-efficient than adjacency matrix
   - Better cache locality for traversal
   - Source: [Graph Implementation in Go](https://journal.hexmos.com/graph-implementation-in-go/)

3. **Thread-Safe Design**
   - Use `sync.RWMutex` for concurrent access
   - Consider `sync.Pool` for temporary allocations
   - Source: [heimdalr/dag](https://github.com/heimdalr/dag)

### 13.2 DAG Best Practices

4. **Prevent Cycles Eagerly**
   - Check for cycles before committing edge
   - Cheaper than detecting cycles later
   - Source: [goombaio/dag](https://github.com/goombaio/dag)

5. **Cache Descendants/Ancestors**
   - Speeds up reachability queries
   - Trade memory for query performance
   - Source: [heimdalr/dag implementation](https://pkg.go.dev/github.com/heimdalr/dag)

6. **Use Kahn's Algorithm for Production**
   - Better error messages than DFS
   - Easier to understand and debug
   - Natural for BFS-based traversal
   - Source: [Kahn's Algorithm](https://takeuforward.org/data-structure/kahns-algorithm-topological-sort-algorithm-bfs-g-22)

### 13.3 Performance Optimization

7. **Transitive Reduction**
   - Remove redundant edges to simplify graph
   - Improves visualization and query performance
   - Source: [Reducing Graph Complexity](https://dominikbraun.io/blog/graphs/reducing-graph-complexity-using-go-and-transitive-reduction/)

8. **Incremental Updates**
   - Don't recompute entire graph on small changes
   - Track "dirty" nodes and update incrementally
   - Source: Epic 2 Review Analysis (Maven's BF and Skipper)

---

## 14. Testing & Validation Checklist

### Unit Tests
- [ ] All public methods have test coverage
- [ ] Edge cases tested (empty graph, single node, etc.)
- [ ] Concurrent access tested (race detector)
- [ ] Error conditions tested

### Integration Tests
- [ ] Database integration tests
- [ ] CLI command tests
- [ ] Multi-level blocking chains
- [ ] Priority inheritance scenarios

### Benchmarks
- [ ] Ready Engine <10ms for 10k nodes
- [ ] Cycle Detection <100ms for 10k nodes
- [ ] Memory usage <50MB for 10k nodes
- [ ] Topological Sort <50ms for 10k nodes

### Documentation
- [ ] API documentation (godoc)
- [ ] Architecture documentation
- [ ] User guide for CLI commands
- [ ] Example usage code

---

## 15. Dependencies & Libraries

### Standard Library
- `container/heap` - Priority queue
- `sync` - Concurrency primitives
- `time` - Time handling

### External Libraries (Optional)
- None required for MVP
- Consider for future:
  - `github.com/stretchr/testify` - Enhanced testing
  - Graph visualization libraries

---

## 16. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Performance not meeting targets | High | Early benchmarking, profiling, optimization |
| Memory leaks | High | Careful pointer management, leak testing |
| Race conditions | Medium | Comprehensive concurrent tests, race detector |
| Complex priority inheritance | Medium | Limit depth, extensive testing |
| GitHub API rate limiting | Low | Aggressive caching, graceful degradation |

---

## 17. Success Metrics

### Performance Metrics
- ✅ Ready Engine: <10ms for 10k nodes
- ✅ Cycle Detection: <100ms for 10k nodes
- ✅ Memory: <50MB for 10k nodes
- ✅ Test Coverage: >90%

### Functional Metrics
- ✅ All 19 dependency types supported
- ✅ Priority inheritance working correctly
- ✅ Gates (timer, human) functional
- ✅ Cycle detection accurate
- ✅ CLI commands operational

---

## Sources

- [Go container/heap Package](https://pkg.go.dev/container/heap)
- [Implementing a Priority Queue in Go](https://leapcell.io/blog/implementing-a-priority-queue-in-go-using-container-heap)
- [Priority Queue in Go - Medium](https://medium.com/@amankumarcs/priority-queue-in-go-b0b0b4844c91)
- [heimdalr/dag - Go DAG Implementation](https://github.com/heimdalr/dag)
- [goombaio/dag - Go Packages](https://github.com/goombaio/dag)
- [Kahn's Algorithm - Topological Sort](https://takeuforward.org/data-structure/kahns-algorithm-topological-sort-algorithm-bfs-g-22)
- [Understanding Topological Sorting - Medium](https://mohammad-imran.medium.com/understanding-topological-sorting-with-kahns-algo-8af5a588dd0e)
- [Graph Implementation in Go - Hexmos](https://journal.hexmos.com/graph-implementation-in-go/)
- [Reducing Graph Complexity - dominikbraun.io](https://dominikbraun.io/blog/graphs/reducing-graph-complexity-using-go-and-transitive-reduction/)
- [DAG Implementation with Concurrent Computation - Medium](https://medium.com/@sunilkv20164012/dag-mplement-in-go-607cf4c34b4b)
- [yourbasic/graph - Graph Algorithms](https://github.com/yourbasic/graph)
- [Go Priority Queue Example](https://go.dev/src/container/heap/example_pq_test.go)
- [Safe Heaps in Golang](https://husobee.github.io/heaps/golang/safe/2016/09/01/safe-heaps-golang.html)

---

**Document Version:** 1.0
**Created:** 2026-02-20
**Next Review:** Before implementation kickoff (Phase 1)
**Estimated Completion:** 8 weeks from start
