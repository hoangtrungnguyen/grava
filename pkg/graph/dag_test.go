package graph

import (
	"testing"
)

func TestAdjacencyDAG_CycleDetection(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	node1 := &Node{ID: "A"}
	node2 := &Node{ID: "B"}
	node3 := &Node{ID: "C"}
	_ = dag.AddNode(node1)
	_ = dag.AddNode(node2)
	_ = dag.AddNode(node3)

	_ = dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	_ = dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})

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
	_ = dag.AddNode(&Node{ID: "A"})
	_ = dag.AddNode(&Node{ID: "B"})
	_ = dag.AddNode(&Node{ID: "C"})

	_ = dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	_ = dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})

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
