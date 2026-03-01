package graph

import (
	"testing"
)

func TestAdjacencyDAG_Traversal(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	// A -> B -> C
	// A -> D
	_ = dag.AddNode(&Node{ID: "A"})
	_ = dag.AddNode(&Node{ID: "B"})
	_ = dag.AddNode(&Node{ID: "C"})
	_ = dag.AddNode(&Node{ID: "D"})

	_ = dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	_ = dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})
	_ = dag.AddEdge(&Edge{FromID: "A", ToID: "D", Type: DependencyBlocks})

	// Test BFS
	visitedBFS := []string{}
	_ = dag.BFS("A", func(id string) bool {
		visitedBFS = append(visitedBFS, id)
		return true
	})

	if len(visitedBFS) != 4 {
		t.Errorf("expected 4 visited nodes in BFS, got %d", len(visitedBFS))
	}

	// Test DFS
	visitedDFS := []string{}
	_ = dag.DFS("A", func(id string) bool {
		visitedDFS = append(visitedDFS, id)
		return true
	})

	if len(visitedDFS) != 4 {
		t.Errorf("expected 4 visited nodes in DFS, got %d", len(visitedDFS))
	}
}
