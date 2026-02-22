package graph

import (
	"testing"
	"time"
)

func TestReadyEngine_ComputeReady(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())

	now := time.Now()

	// Setup nodes
	// A (Priority 1) -> B (Priority 2)
	// C (Priority 0, Blocked by A)
	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityHigh, CreatedAt: now.Add(-24 * time.Hour)})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: now.Add(-12 * time.Hour)})
	dag.AddNode(&Node{ID: "C", Status: StatusOpen, Priority: PriorityCritical, CreatedAt: now})

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "A", ToID: "C", Type: DependencyBlocks})

	// Only A and B are candidiates for ready tasks, but B is blocked by A.
	// Wait, B is blocked by A, so only A should be ready.
	// Actually, B has indegree 1 (from A).
	// C has indegree 1 (from A).
	// Only A has indegree 0.

	ready, err := engine.ComputeReady(0)
	if err != nil {
		t.Fatalf("ComputeReady failed: %v", err)
	}

	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task (A), got %d", len(ready))
	}

	if ready[0].Node.ID != "A" {
		t.Errorf("expected A, got %s", ready[0].Node.ID)
	}

	// Test Priority Inheritance
	// A inherits priority from C (Critical)
	if ready[0].EffectivePriority != PriorityCritical {
		t.Errorf("expected A to inherit PriorityCritical from C, got %d", ready[0].EffectivePriority)
	}

	// Close A, now B and C should be ready
	dag.SetNodeStatus("A", StatusClosed)
	ready, _ = engine.ComputeReady(0)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready tasks (B, C), got %d", len(ready))
	}

	// C should be first (Critical)
	if ready[0].Node.ID != "C" {
		t.Errorf("expected C first, got %s", ready[0].Node.ID)
	}
}

func TestReadyEngine_GateFiltering(t *testing.T) {
	dag := NewAdjacencyDAG(false)
	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())

	now := time.Now()
	// Task with timer gate in future
	future := now.Add(1 * time.Hour).Format(time.RFC3339)
	dag.AddNode(&Node{ID: "Gated", Status: StatusOpen, Priority: PriorityHigh, CreatedAt: now, AwaitType: "timer", AwaitID: future})

	ready, _ := engine.ComputeReady(0)
	if len(ready) != 0 {
		t.Errorf("gated task should not be ready")
	}

	// Task with timer gate in past
	past := now.Add(-1 * time.Hour).Format(time.RFC3339)
	dag.AddNode(&Node{ID: "Expired", Status: StatusOpen, Priority: PriorityHigh, CreatedAt: now, AwaitType: "timer", AwaitID: past})

	ready, _ = engine.ComputeReady(0)
	if len(ready) != 1 || ready[0].Node.ID != "Expired" {
		t.Errorf("expired gated task should be ready")
	}
}

func TestReadyEngine_DeepInheritance(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	config := DefaultReadyEngineConfig()
	config.PriorityInheritanceDepth = 10
	engine := NewReadyEngine(dag, config)

	// A -> B -> C (Priority Critical)
	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityLow, CreatedAt: time.Now()})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: time.Now()})
	dag.AddNode(&Node{ID: "C", Status: StatusOpen, Priority: PriorityCritical, CreatedAt: time.Now()})

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})

	ready, _ := engine.ComputeReady(0)
	if len(ready) != 1 || ready[0].Node.ID != "A" {
		t.Fatalf("expected 1 ready task (A), got %d", len(ready))
	}

	if ready[0].EffectivePriority != PriorityCritical {
		t.Errorf("expected A to inherit Critical from C, got %d", ready[0].EffectivePriority)
	}
}

func TestReadyEngine_InheritanceLimit(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	config := DefaultReadyEngineConfig()
	config.PriorityInheritanceDepth = 1 // Limit to immediate children
	engine := NewReadyEngine(dag, config)

	// A -> B -> C (Priority Critical)
	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityLow, CreatedAt: time.Now()})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: time.Now()})
	dag.AddNode(&Node{ID: "C", Status: StatusOpen, Priority: PriorityCritical, CreatedAt: time.Now()})

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})

	ready, _ := engine.ComputeReady(0)
	// A should inherit from B (Medium), but NOT from C (Critical) because C is depth 2
	if ready[0].EffectivePriority != PriorityMedium {
		t.Errorf("expected A to inherit Medium from B (depth 1), but limited inheritance failed, got %d", ready[0].EffectivePriority)
	}
}

func TestReadyEngine_Aging(t *testing.T) {
	dag := NewAdjacencyDAG(false)
	config := DefaultReadyEngineConfig()
	config.AgingThreshold = 24 * time.Hour
	config.AgingBoost = 1
	engine := NewReadyEngine(dag, config)

	now := time.Now()
	// New task (Low)
	dag.AddNode(&Node{ID: "New", Status: StatusOpen, Priority: PriorityLow, CreatedAt: now})
	// Old task (Low, 2 days old)
	dag.AddNode(&Node{ID: "Old", Status: StatusOpen, Priority: PriorityLow, CreatedAt: now.Add(-48 * time.Hour)})

	ready, _ := engine.ComputeReady(0)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready tasks, got %d", len(ready))
	}

	// Old task should have boosted priority (Low -> Medium) and be first
	if ready[0].Node.ID != "Old" {
		t.Errorf("expected Old task first due to aging boost, got %s", ready[0].Node.ID)
	}
	if ready[0].EffectivePriority != PriorityMedium {
		t.Errorf("expected Old task to have PriorityMedium (2), got %d", ready[0].EffectivePriority)
	}
	if !ready[0].PriorityBoosted {
		t.Errorf("expected Old task to have PriorityBoosted true")
	}
}

func TestReadyEngine_Limits(t *testing.T) {
	dag := NewAdjacencyDAG(false)
	engine := NewReadyEngine(dag, nil)

	dag.AddNode(&Node{ID: "1", Status: StatusOpen, Priority: PriorityCritical, CreatedAt: time.Now()})
	dag.AddNode(&Node{ID: "2", Status: StatusOpen, Priority: PriorityHigh, CreatedAt: time.Now()})
	dag.AddNode(&Node{ID: "3", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: time.Now()})

	ready, _ := engine.ComputeReady(2)
	if len(ready) != 2 {
		t.Errorf("expected 2 ready tasks, got %d", len(ready))
	}
	if ready[0].Node.ID != "1" || ready[1].Node.ID != "2" {
		t.Errorf("tasks out of order or incorrect")
	}
}
