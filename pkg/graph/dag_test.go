package graph

import (
	"testing"
)

func TestAdjacencyDAG_CycleDetection(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	node1 := &Node{ID: "A"}
	node2 := &Node{ID: "B"}
	node3 := &Node{ID: "C"}
	dag.AddNode(node1)
	dag.AddNode(node2)
	dag.AddNode(node3)

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})

	// Test AddEdgeWithCycleCheck - No Cycle
	if err := dag.AddEdgeWithCycleCheck(&Edge{FromID: "A", ToID: "C", Type: DependencyBlocks}); err != nil {
		t.Errorf("unexpected error adding non-cycle edge: %v", err)
	}

	// Test AddEdgeWithCycleCheck - Simple Cycle
	err := dag.AddEdgeWithCycleCheck(&Edge{FromID: "C", ToID: "A", Type: DependencyBlocks})
	if err == nil {
		t.Errorf("expected error when adding cycle edge, got nil")
	}
	if _, ok := err.(*CycleError); !ok {
		t.Errorf("expected CycleError, got %T", err)
	}

	// Verify edge was NOT added
	if dag.IsReachable("C", "A") {
		t.Errorf("C should not be able to reach A after failed cycle addition")
	}
}

func TestAdjacencyDAG_TransitiveDependencies(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	// A -> B -> C
	dag.AddNode(&Node{ID: "A"})
	dag.AddNode(&Node{ID: "B"})
	dag.AddNode(&Node{ID: "C"})

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})

	// Transitive dependencies of C should be B and A
	deps, err := dag.GetTransitiveDependencies("C", 0)
	if err != nil {
		t.Fatalf("failed to get transitive deps: %v", err)
	}

	depMap := make(map[string]bool)
	for _, d := range deps {
		depMap[d] = true
	}

	if !depMap["B"] || !depMap["A"] {
		t.Errorf("expected [A, B], got %v", deps)
	}
}

func TestAdjacencyDAG_StatusBubbling(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	// Epic -> Task -> Subtask1 & Subtask2
	dag.AddNode(&Node{ID: "Epic1", Status: StatusOpen})
	dag.AddNode(&Node{ID: "Task1", Status: StatusOpen})
	dag.AddNode(&Node{ID: "Sub1", Status: StatusOpen})
	dag.AddNode(&Node{ID: "Sub2", Status: StatusOpen})

	// Add subtask-of edges (child --subtask-of--> parent)
	dag.AddEdge(&Edge{FromID: "Task1", ToID: "Epic1", Type: DependencySubtaskOf})
	dag.AddEdge(&Edge{FromID: "Sub1", ToID: "Task1", Type: DependencySubtaskOf})
	dag.AddEdge(&Edge{FromID: "Sub2", ToID: "Task1", Type: DependencySubtaskOf})

	// Test 1: Set Sub1 to InProgress -> Task1 and Epic1 should become InProgress
	err := dag.SetNodeStatus("Sub1", StatusInProgress)
	if err != nil {
		t.Fatalf("SetNodeStatus failed: %v", err)
	}

	task1, _ := dag.GetNode("Task1")
	if task1.Status != StatusInProgress {
		t.Errorf("Expected Task1 status to be InProgress, got %v", task1.Status)
	}
	epic1, _ := dag.GetNode("Epic1")
	if epic1.Status != StatusInProgress {
		t.Errorf("Expected Epic1 status to be InProgress, got %v", epic1.Status)
	}

	// Test 2: Set Sub1 to Closed. Task1 is still InProgress because Sub2 is Open.
	_ = dag.SetNodeStatus("Sub1", StatusClosed)
	if task1.Status == StatusClosed {
		t.Errorf("Expected Task1 to NOT be closed yet")
	}

	// Test 3: Set Sub2 to Closed. This should close Task1 and Epic1.
	_ = dag.SetNodeStatus("Sub2", StatusClosed)
	task1, _ = dag.GetNode("Task1")
	if task1.Status != StatusClosed {
		t.Errorf("Expected Task1 to be closed, got %v", task1.Status)
	}
	epic1, _ = dag.GetNode("Epic1")
	if epic1.Status != StatusClosed {
		t.Errorf("Expected Epic1 to be closed, got %v", epic1.Status)
	}
}
