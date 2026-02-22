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
	dag.nodes["A"].Status = StatusClosed
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
