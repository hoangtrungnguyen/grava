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
