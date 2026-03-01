package graph

import (
	"reflect"
	"testing"
)

func TestAdjacencyDAG_TransitiveReduction(t *testing.T) {
	// Create a DAG: A -> B, B -> C, A -> C (redundant)
	dag := NewAdjacencyDAG(true)
	dag.AddNode(&Node{ID: "A"}) //nolint:errcheck
	dag.AddNode(&Node{ID: "B"}) //nolint:errcheck
	dag.AddNode(&Node{ID: "C"}) //nolint:errcheck

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks}) //nolint:errcheck
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks}) //nolint:errcheck
	dag.AddEdge(&Edge{FromID: "A", ToID: "C", Type: DependencyBlocks}) //nolint:errcheck

	if dag.EdgeCount() != 3 {
		t.Errorf("Expected 3 edges, got %d", dag.EdgeCount())
	}

	err := dag.TransitiveReduction()
	if err != nil {
		t.Fatalf("TransitiveReduction failed: %v", err)
	}

	if dag.EdgeCount() != 2 {
		t.Errorf("Expected 2 edges after reduction, got %d", dag.EdgeCount())
	}

	// Verify A -> C is gone
	successors, _ := dag.GetSuccessors("A")
	if len(successors) != 1 || successors[0] != "B" {
		t.Errorf("Expected only B as successor of A, got %v", successors)
	}
}

func TestAdjacencyDAG_GetBlockingPath(t *testing.T) {
	// A -> B -> C (blocking)
	// A -> D -> C (non-blocking)
	dag := NewAdjacencyDAG(false)
	dag.AddNode(&Node{ID: "A"}) //nolint:errcheck
	dag.AddNode(&Node{ID: "B"}) //nolint:errcheck
	dag.AddNode(&Node{ID: "C"}) //nolint:errcheck
	dag.AddNode(&Node{ID: "D"}) //nolint:errcheck

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})    //nolint:errcheck
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})    //nolint:errcheck
	dag.AddEdge(&Edge{FromID: "A", ToID: "D", Type: DependencyRelatesTo}) //nolint:errcheck
	dag.AddEdge(&Edge{FromID: "D", ToID: "C", Type: DependencyBlocks})    //nolint:errcheck

	// Blocking path from A to C should be A -> B -> C
	path, err := dag.GetBlockingPath("A", "C")
	if err != nil {
		t.Fatalf("GetBlockingPath failed: %v", err)
	}

	expected := []string{"A", "B", "C"}
	if !reflect.DeepEqual(path, expected) {
		t.Errorf("Expected path %v, got %v", expected, path)
	}

	// No blocking path from D back to A
	path, err = dag.GetBlockingPath("D", "A")
	if err != nil {
		t.Fatalf("GetBlockingPath failed: %v", err)
	}
	if path != nil {
		t.Errorf("Expected nil path, got %v", path)
	}
}
