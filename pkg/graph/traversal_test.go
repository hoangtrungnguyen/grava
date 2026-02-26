package graph

import (
	"testing"
)

func TestAdjacencyDAG_Traversal(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	// A -> B -> C
	// A -> D
	dag.AddNode(&Node{ID: "A"})
	dag.AddNode(&Node{ID: "B"})
	dag.AddNode(&Node{ID: "C"})
	dag.AddNode(&Node{ID: "D"})

	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyBlocks})
	dag.AddEdge(&Edge{FromID: "A", ToID: "D", Type: DependencyBlocks})

	// Test BFS
	visitedBFS := []string{}
	dag.BFS("A", func(id string) bool { //nolint:errcheck
		visitedBFS = append(visitedBFS, id)
		return true
	})

	if len(visitedBFS) != 4 {
		t.Errorf("expected 4 visited nodes in BFS, got %d", len(visitedBFS))
	}

	// Test DFS
	visitedDFS := []string{}
	dag.DFS("A", func(id string) bool { //nolint:errcheck
		visitedDFS = append(visitedDFS, id)
		return true
	})

	if len(visitedDFS) != 4 {
		t.Errorf("expected 4 visited nodes in DFS, got %d", len(visitedDFS))
	}
}
