package graph

import (
	"testing"
	"time"
)

func TestGraphCache(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	cache := dag.cache

	// Test Indegree Cache
	cache.SetIndegree("A", 5)
	val, ok := cache.GetIndegree("A")
	if !ok || val != 5 {
		t.Errorf("expected 5, got %d", val)
	}

	cache.InvalidateIndegree("A")
	_, ok = cache.GetIndegree("A")
	if ok {
		t.Errorf("expected A to be invalidated")
	}

	// Test InvalidateAll
	cache.SetIndegree("B", 3)
	cache.InvalidateAll()
	_, ok = cache.GetIndegree("B")
	if ok {
		t.Errorf("expected all to be invalidated")
	}
}

func TestReadyEngine_Caching(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	now := time.Now()
	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityHigh, CreatedAt: now})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: now})

	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())

	// First compute
	tasks, err := engine.ComputeReady(0)
	if err != nil {
		t.Fatalf("ComputeReady failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}

	// Second compute - should be cached
	tasks2, _ := engine.ComputeReady(0)
	if len(tasks2) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks2))
	}

	// Add an edge - should invalidate cache
	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})

	tasks3, _ := engine.ComputeReady(0)
	if len(tasks3) != 1 || tasks3[0].Node.ID != "A" {
		t.Errorf("Expected only A to be ready, got %v", tasks3)
	}
}

func TestReadyEngine_IncrementalPriority(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	now := time.Now()
	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityLow, CreatedAt: now})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityCritical, CreatedAt: now})

	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())

	// Before edge, A is PriorityLow
	tasks, _ := engine.ComputeReady(0)
	for _, task := range tasks {
		if task.Node.ID == "A" && task.EffectivePriority != PriorityLow {
			t.Errorf("Expected A priority Low, got %v", task.EffectivePriority)
		}
	}

	// Add blocking edge A -> B. A should inherit PriorityCritical.
	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})

	tasks, _ = engine.ComputeReady(0)
	for _, task := range tasks {
		if task.Node.ID == "A" && task.EffectivePriority != PriorityCritical {
			t.Errorf("Expected A priority Critical due to inheritance, got %v", task.EffectivePriority)
		}
	}
}
