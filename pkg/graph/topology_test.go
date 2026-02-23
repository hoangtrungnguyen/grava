package graph

import (
	"testing"
)

func TestAdjacencyDAG_TopologicalSort(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	// A -> B -> C
	// A -> C
	dag.AddNode(&Node{ID: "A"})
	dag.AddNode(&Node{ID: "B"})
	dag.AddNode(&Node{ID: "C"})

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "A", ToID: "C", Type: DependencyBlocks})

	sorted, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if sorted[0] != "A" {
		t.Errorf("expected A first, got %s", sorted[0])
	}
	if sorted[2] != "C" {
		t.Errorf("expected C last, got %s", sorted[2])
	}

	// Add a cycle
	dag.AddEdge(&Edge{FromID: "C", ToID: "A", Type: DependencyBlocks})
	_, err = dag.TopologicalSort()
	if err != ErrCycleDetected {
		t.Errorf("expected ErrCycleDetected, got %v", err)
	}
}
