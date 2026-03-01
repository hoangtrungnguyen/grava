package graph

import (
	"testing"
	"time"
)

func TestAdjacencyDAG_Nodes(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	node1 := &Node{ID: "task1", Status: StatusOpen, CreatedAt: time.Now()}
	node2 := &Node{ID: "task2", Status: StatusOpen, CreatedAt: time.Now()}

	// Test AddNode
	if err := dag.AddNode(node1); err != nil {
		t.Fatalf("failed to add node1: %v", err)
	}
	if err := dag.AddNode(node2); err != nil {
		t.Fatalf("failed to add node2: %v", err)
	}

	// Test Duplicate AddNode
	if err := dag.AddNode(node1); err != ErrNodeExists {
		t.Errorf("expected ErrNodeExists, got %v", err)
	}

	// Test GetNode
	n, err := dag.GetNode("task1")
	if err != nil {
		t.Fatalf("failed to get node1: %v", err)
	}
	if n.ID != "task1" {
		t.Errorf("expected task1, got %s", n.ID)
	}

	// Test NodeCount
	if count := dag.NodeCount(); count != 2 {
		t.Errorf("expected 2 nodes, got %d", count)
	}

	// Test RemoveNode
	if err := dag.RemoveNode("task1"); err != nil {
		t.Fatalf("failed to remove task1: %v", err)
	}
	if dag.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", dag.NodeCount())
	}
}

func TestAdjacencyDAG_Edges(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	node1 := &Node{ID: "task1"}
	node2 := &Node{ID: "task2"}
	_ = dag.AddNode(node1)
	_ = dag.AddNode(node2)

	edge := &Edge{FromID: "task1", ToID: "task2", Type: DependencyBlocks}

	// Test AddEdge
	if err := dag.AddEdge(edge); err != nil {
		t.Fatalf("failed to add edge: %v", err)
	}

	// Test Indegree/Outdegree
	if deg := dag.GetIndegree("task2"); deg != 1 {
		t.Errorf("expected indegree 1 for task2, got %d", deg)
	}
	if deg := dag.GetOutdegree("task1"); deg != 1 {
		t.Errorf("expected outdegree 1 for task1, got %d", deg)
	}

	// Test EdgeCount
	if count := dag.EdgeCount(); count != 1 {
		t.Errorf("expected 1 edge, got %d", count)
	}

	// Test GetOutgoingEdges
	edges, _ := dag.GetOutgoingEdges("task1")
	if len(edges) != 1 || edges[0].ToID != "task2" {
		t.Errorf("invalid outgoing edges for task1")
	}

	// Test RemoveEdge
	if err := dag.RemoveEdge("task1", "task2", DependencyBlocks); err != nil {
		t.Fatalf("failed to remove edge: %v", err)
	}
	if dag.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", dag.EdgeCount())
	}
}
