package graph

import (
	"strings"
	"testing"
)

func TestToMermaid(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	// Add some nodes with different statuses
	_ = dag.AddNode(&Node{ID: "A", Title: "Task A", Status: StatusOpen})
	_ = dag.AddNode(&Node{ID: "B", Title: "Task \"B\"", Status: StatusInProgress})
	_ = dag.AddNode(&Node{ID: "C", Title: "Task [C]", Status: StatusClosed})

	// Add some edges
	_ = dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})
	_ = dag.AddEdge(&Edge{FromID: "B", ToID: "C", Type: DependencyRelatesTo}) // Non-blocking

	mermaid := ToMermaid(dag)

	// Check for header
	if !strings.Contains(mermaid, "graph TD") {
		t.Error("Mermaid output should start with graph TD")
	}

	// Check for class definitions
	if !strings.Contains(mermaid, "classDef open") {
		t.Error("Mermaid output should contain classDef open")
	}

	// Check for nodes and clean titles
	if !strings.Contains(mermaid, "A[\"Task A<br/>(A)\"]") {
		t.Error("Task A node definition wrong")
	}
	if !strings.Contains(mermaid, "B[\"Task 'B'<br/>(B)\"]") {
		t.Error("Task B node with quotes not cleaned correctly")
	}
	if !strings.Contains(mermaid, "C[\"Task (C)<br/>(C)\"]") {
		t.Error("Task C node with brackets not cleaned correctly")
	}

	// Check for classes
	if !strings.Contains(mermaid, "class A open") {
		t.Error("A should be open")
	}
	if !strings.Contains(mermaid, "class B in_progress") {
		t.Error("B should be in_progress")
	}
	if !strings.Contains(mermaid, "class C closed") {
		t.Error("C should be closed")
	}

	// Check for edges
	if !strings.Contains(mermaid, "A --> B") {
		t.Error("Blocking edge A -> B missing or wrong arrow")
	}
	if !strings.Contains(mermaid, "B -.-> C") {
		t.Error("Relates-to edge B -> C should use dashed arrow")
	}
}
