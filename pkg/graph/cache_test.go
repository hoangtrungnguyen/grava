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

func TestPriorityPropagation(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	now := time.Now()

	// A -> B -> C
	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityBacklog, CreatedAt: now})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityBacklog, CreatedAt: now})
	dag.AddNode(&Node{ID: "C", Status: StatusOpen, Priority: PriorityLow, CreatedAt: now})

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})

	// Initial calculation (lazy population of cache)
	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())
	engine.ComputeReady(0)

	// Verify initial effective priorities in cache
	// Note: Only A is guaranteed to be in cache because it was the only ready task
	// But bubbling from edges might have partially populated it.
	effA, okA := dag.cache.GetPriority("A")
	if !okA || effA != PriorityLow {
		t.Errorf("Initial priority A wrong: val=%v, ok=%v", effA, okA)
	}

	// Now update C's priority to Critical
	dag.SetNodePriority("C", PriorityCritical)

	// Verify that C, B, and A are updated proactively in the cache
	effA, okA = dag.cache.GetPriority("A")
	effB, okB := dag.cache.GetPriority("B")
	effC, okC := dag.cache.GetPriority("C")

	t.Logf("After update: A=%v(ok:%v), B=%v(ok:%v), C=%v(ok:%v)", effA, okA, effB, okB, effC, okC)

	if !okC || effC != PriorityCritical {
		t.Errorf("C priority not updated: val=%v, ok=%v", effC, okC)
	}
	if !okB || effB != PriorityCritical {
		t.Errorf("B priority not bubbled: val=%v, ok=%v", effB, okB)
	}
	if !okA || effA != PriorityCritical {
		t.Errorf("A priority not bubbled: val=%v, ok=%v", effA, okA)
	}
}

func TestPriorityPropagation_Rollback(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	now := time.Now()

	// A -> B
	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityBacklog, CreatedAt: now})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityCritical, CreatedAt: now})
	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})

	// Initial calculation
	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())
	engine.ComputeReady(0)

	effA, _ := dag.cache.GetPriority("A")
	if effA != PriorityCritical {
		t.Errorf("Expected A to inherit Critical, got %v", effA)
	}

	// Now close B
	dag.SetNodeStatus("B", StatusClosed)

	// A should no longer inherit from B
	effA, _ = dag.cache.GetPriority("A")
	if effA != PriorityBacklog {
		t.Errorf("Expected A to revert to Backlog after B closed, got %v", effA)
	}
}
