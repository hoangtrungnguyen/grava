package graph

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderASCII_SimpleGraph(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "1", Title: "Task 1", Type: "task", Status: StatusOpen, Priority: PriorityHigh}
	n2 := &Node{ID: "2", Title: "Task 2", Type: "task", Status: StatusOpen, Priority: PriorityMedium}
	n3 := &Node{ID: "3", Title: "Task 3", Type: "task", Status: StatusClosed, Priority: PriorityLow}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))
	require.NoError(t, dag.AddNode(n3))

	require.NoError(t, dag.AddEdge(&Edge{FromID: "1", ToID: "2", Type: DependencyBlocks}))
	require.NoError(t, dag.AddEdge(&Edge{FromID: "2", ToID: "3", Type: DependencyBlocks}))

	output, err := dag.RenderASCII(RenderOptions{Format: "ascii"})
	require.NoError(t, err)

	assert.Contains(t, output, "Task 1")
	assert.Contains(t, output, "Task 2")
	assert.Contains(t, output, "Task 3")
	assert.Contains(t, output, "blocks")
}

func TestRenderASCII_IncludesIsolatedNodes(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "1", Title: "Connected", Type: "task", Status: StatusOpen}
	n2 := &Node{ID: "2", Title: "Isolated", Type: "task", Status: StatusOpen}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))

	output, err := dag.RenderASCII(RenderOptions{Format: "ascii"})
	require.NoError(t, err)

	assert.Contains(t, output, "Isolated Nodes:")
	assert.Contains(t, output, "Isolated")
}

func TestRenderASCII_ExcludesArchived(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "1", Title: "Active", Type: "task", Status: StatusOpen}
	n2 := &Node{ID: "2", Title: "Archived", Type: "task", Status: StatusArchived}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))

	output, err := dag.RenderASCII(RenderOptions{Format: "ascii"})
	require.NoError(t, err)

	assert.Contains(t, output, "Active")
	assert.NotContains(t, output, "Archived")
}

func TestRenderASCII_Subgraph(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "1", Title: "Root", Type: "task", Status: StatusOpen}
	n2 := &Node{ID: "2", Title: "Child", Type: "task", Status: StatusOpen}
	n3 := &Node{ID: "3", Title: "Grandchild", Type: "task", Status: StatusOpen}
	n4 := &Node{ID: "4", Title: "Unrelated", Type: "task", Status: StatusOpen}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))
	require.NoError(t, dag.AddNode(n3))
	require.NoError(t, dag.AddNode(n4))

	require.NoError(t, dag.AddEdge(&Edge{FromID: "1", ToID: "2", Type: DependencyBlocks}))
	require.NoError(t, dag.AddEdge(&Edge{FromID: "2", ToID: "3", Type: DependencyBlocks}))

	output, err := dag.RenderASCII(RenderOptions{Format: "ascii", RootID: "1"})
	require.NoError(t, err)

	assert.Contains(t, output, "Root")
	assert.Contains(t, output, "Child")
	assert.Contains(t, output, "Grandchild")
	assert.NotContains(t, output, "Unrelated")
}

func TestRenderDOT_SimpleGraph(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "task-1", Title: "Buy milk", Type: "task", Status: StatusOpen}
	n2 := &Node{ID: "task-2", Title: "Cook dinner", Type: "task", Status: StatusInProgress}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))

	require.NoError(t, dag.AddEdge(&Edge{FromID: "task-1", ToID: "task-2", Type: DependencyBlocks}))

	output, err := dag.RenderDOT(RenderOptions{Format: "dot"})
	require.NoError(t, err)

	assert.Contains(t, output, "digraph G")
	assert.Contains(t, output, "Buy milk")
	assert.Contains(t, output, "Cook dinner")
	assert.Contains(t, output, "task-1")
	assert.Contains(t, output, "task-2")
	assert.Contains(t, output, "blocks")
	assert.Contains(t, output, "lightblue") // InProgress color
}

func TestRenderDOT_DashedEdges(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "1", Title: "A", Type: "task", Status: StatusOpen}
	n2 := &Node{ID: "2", Title: "B", Type: "task", Status: StatusOpen}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))

	require.NoError(t, dag.AddEdge(&Edge{FromID: "1", ToID: "2", Type: DependencyWaitsFor}))

	output, err := dag.RenderDOT(RenderOptions{Format: "dot"})
	require.NoError(t, err)

	assert.Contains(t, output, "dashed")
}

func TestRenderJSON_ValidStructure(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "task-1", Title: "Task One", Type: "task", Status: StatusOpen, Priority: PriorityHigh}
	n2 := &Node{ID: "task-2", Title: "Task Two", Type: "task", Status: StatusInProgress, Priority: PriorityMedium}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))

	require.NoError(t, dag.AddEdge(&Edge{FromID: "task-1", ToID: "task-2", Type: DependencyBlocks}))

	output, err := dag.RenderJSON(RenderOptions{Format: "json"})
	require.NoError(t, err)

	var result GraphJSON
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, 2, len(result.Nodes))
	assert.Equal(t, 1, len(result.Edges))

	// Find nodes
	var node1, node2 *NodeJSON
	for i := range result.Nodes {
		if result.Nodes[i].ID == "task-1" {
			node1 = &result.Nodes[i]
		} else if result.Nodes[i].ID == "task-2" {
			node2 = &result.Nodes[i]
		}
	}

	require.NotNil(t, node1)
	require.NotNil(t, node2)

	assert.Equal(t, "Task One", node1.Title)
	assert.Equal(t, "Task Two", node2.Title)
	assert.Equal(t, "task", node1.Type)

	// Check edge
	assert.Equal(t, "task-1", result.Edges[0].From)
	assert.Equal(t, "task-2", result.Edges[0].To)
	assert.Equal(t, "blocks", result.Edges[0].Type)
}

func TestRenderJSON_IncludesIsolatedNodes(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "1", Title: "Isolated", Type: "task", Status: StatusOpen}

	require.NoError(t, dag.AddNode(n1))

	output, err := dag.RenderJSON(RenderOptions{Format: "json"})
	require.NoError(t, err)

	var result GraphJSON
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, 1, len(result.Nodes))
	assert.Equal(t, 0, len(result.Edges))
	assert.Equal(t, "Isolated", result.Nodes[0].Title)
}

func TestRender_InvalidFormat(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	_, err := dag.Render(RenderOptions{Format: "invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestRenderASCII_MultipleEdgeTypes(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n1 := &Node{ID: "1", Title: "Task A", Type: "task", Status: StatusOpen}
	n2 := &Node{ID: "2", Title: "Task B", Type: "task", Status: StatusOpen}
	n3 := &Node{ID: "3", Title: "Task C", Type: "task", Status: StatusOpen}

	require.NoError(t, dag.AddNode(n1))
	require.NoError(t, dag.AddNode(n2))
	require.NoError(t, dag.AddNode(n3))

	require.NoError(t, dag.AddEdge(&Edge{FromID: "1", ToID: "2", Type: DependencyBlocks}))
	require.NoError(t, dag.AddEdge(&Edge{FromID: "2", ToID: "3", Type: DependencyWaitsFor}))

	output, err := dag.RenderASCII(RenderOptions{Format: "ascii"})
	require.NoError(t, err)

	assert.Contains(t, output, "blocks")
	assert.Contains(t, output, "waits-for")
}

func TestRenderDOT_ClosedNodeColor(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	n := &Node{ID: "1", Title: "Done", Type: "task", Status: StatusClosed}
	require.NoError(t, dag.AddNode(n))

	output, err := dag.RenderDOT(RenderOptions{Format: "dot"})
	require.NoError(t, err)

	assert.Contains(t, output, "gray")
}

func TestGetReachableNodes(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	// Create chain: 1 -> 2 -> 3 -> 4
	for i := 1; i <= 4; i++ {
		n := &Node{ID: string(rune('0' + i)), Title: "Task", Type: "task", Status: StatusOpen}
		require.NoError(t, dag.AddNode(n))
	}

	require.NoError(t, dag.AddEdge(&Edge{FromID: "1", ToID: "2", Type: DependencyBlocks}))
	require.NoError(t, dag.AddEdge(&Edge{FromID: "2", ToID: "3", Type: DependencyBlocks}))
	require.NoError(t, dag.AddEdge(&Edge{FromID: "3", ToID: "4", Type: DependencyBlocks}))

	reachable := dag.getReachableNodes("1")

	assert.Contains(t, reachable, "1")
	assert.Contains(t, reachable, "2")
	assert.Contains(t, reachable, "3")
	assert.Contains(t, reachable, "4")
	assert.Equal(t, 4, len(reachable))
}

func TestRenderJSON_EmptyGraph(t *testing.T) {
	dag := NewAdjacencyDAG(false)

	output, err := dag.RenderJSON(RenderOptions{Format: "json"})
	require.NoError(t, err)

	var result GraphJSON
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, 0, len(result.Nodes))
	assert.Equal(t, 0, len(result.Edges))
}
